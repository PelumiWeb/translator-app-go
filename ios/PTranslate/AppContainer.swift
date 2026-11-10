import Foundation
import Observation

/// Builds the app's object graph by hand: every type receives what it needs through its initialiser, and this is
/// the one place that decides which implementation that is. There is no DI framework.
@MainActor
final class AppContainer {
    let recorder: any AudioRecorder = EngineAudioRecorder()
    let player: any AudioPlayer = EngineAudioPlayer()

    /// Downloads the Whisper model from the backend and keeps it in the app's files.
    let models: ModelManager

    /// Whether this phone can run Whisper at all. When it cannot, every recording goes to the server.
    let onDeviceTranscription = WhisperSupport.processorSupported

    /// Which transcriber the next recording uses. The choice on screen sets it.
    let route = RouteSetting(WhisperSupport.processorSupported ? .automatic : .cloud)

    let thresholds = FallbackThresholds()

    /// How fast this phone runs the model, measured once per model.
    let speed: any DeviceSpeed

    let translator: any Translator = MLKitTranslator()

    /// Says translations aloud in the user's own voice, through the server, or in one of the phone's voices when
    /// that cannot be done.
    let synthesizer: any SpeechSynthesizer

    let pipeline: SpeechTranslationPipeline

    /// Writes down what was said: on this phone or on the server, as the route and the phone allow.
    let transcriber: any Transcriber

    /// The signed-in user, kept across launches, the token in the Keychain (ADR 0018).
    let account: Account

    let rooms: RoomsClient

    /// One stream of room changes for every screen that shows rooms.
    let roomFeed: RoomFeed

    /// Plays room notes: the sender's voice from the server, or this phone's voice reading the words.
    let messagePlayer: MessagePlayer

    /// The language the phone is set to, offered first when signing up, if Whisper knows it.
    let phoneLanguage: Language? = Locale.current.language.languageCode
        .map { Language($0.identifier == "nb" ? "no" : $0.identifier) }
        .flatMap { WhisperLanguages.supports($0) ? $0 : nil }

    /// The recording of the user's voice, in the app's private files.
    let voice: any VoiceSampleStore = FileVoiceSampleStore(directory: .applicationSupportDirectory.appending(path: "voice"))
    let voiceSetup: any VoiceSetupPreference = UserDefaultsVoiceSetup(defaults: .standard)

    /// Whenever a model becomes available, makes sure its speed is known. Lives as long as the app.
    private let measureWhenReady: Task<Void, Never>

    init(bundle: Bundle = .main) {
        let session = URLSession(configuration: .default)

        // Requests on behalf of the signed-in user. The token comes from the account, and a session the server says
        // has ended is forgotten there. The account is made after the clients it uses, so the closures reach it
        // through a box filled in just below.
        let box = AccountBox()
        let backend = BackendClient(baseURL: Self.backendURL(in: bundle), session: session, token: { await box.account?.token })
        let rest = Rest(
            baseURL: Self.backendURL(in: bundle),
            session: session,
            token: { await box.account?.token },
            sessionEnded: { await box.account?.forget() }
        )
        let account = Account(
            client: AccountClient(rest: rest),
            store: KeychainSessionStore(defaults: .standard),
            progress: UserDefaultsOnboardingProgress(defaults: .standard)
        )
        box.account = account
        self.account = account
        let rooms = RoomsClient(rest: rest)
        self.rooms = rooms
        roomFeed = RoomFeed(rooms: rooms)
        let models = ModelManager(
            backend: backend,
            directory: .applicationSupportDirectory.appending(path: "models"),
            modelID: Self.whisperModelID
        )
        self.models = models

        let whisper = WhisperTranscriber(
            modelFile: { await models.installedModel },
            // More threads than fast cores makes Whisper slower, not faster: the slow cores hold the others back.
            threads: min(max(ProcessInfo.processInfo.activeProcessorCount, 1), 4)
        )

        let clip = Self.benchmarkClip(in: bundle)
        let benchmark = DeviceBenchmark(
            // The file's name and size stand in for a version: a different model has at least a different size.
            modelVersion: {
                guard
                    let file = models.installedModel,
                    let size = try? file.resourceValues(forKeys: [.fileSizeKey]).fileSize
                else { return nil }
                return "\(file.lastPathComponent):\(size)"
            },
            sample: { try await Self.read(clip) },
            prepare: { try await whisper.load() },
            transcribe: { audio in
                for try await _ in whisper.transcribe(audio, language: .english) {}
            },
            store: UserDefaultsSpeedStore(defaults: .standard)
        )
        speed = benchmark
        measureWhenReady = Task {
            // A phone that cannot run Whisper never measures it (WhisperSupport).
            guard WhisperSupport.processorSupported else { return }
            while !Task.isCancelled {
                if models.installedModel != nil {
                    await benchmark.ensureMeasured()
                }
                await nextChange { _ = models.installedModel }
            }
        }

        let (voice, player) = (voice, player)
        let phoneVoice = SystemSpeechSynthesizer()
        let synthesizer = FallbackSpeechSynthesizer(
            preferred: RemoteSpeechSynthesizer(
                backend: backend,
                voiceSample: { @MainActor in try await voice.loadWAV() },
                play: { @MainActor audio in try await player.play(audio) },
                stopPlaying: { @MainActor in player.stop() }
            ),
            fallback: phoneVoice
        )
        self.synthesizer = synthesizer
        messagePlayer = MessagePlayer(rooms: rooms, player: player, phoneVoice: phoneVoice)

        let transcriber = RoutingTranscriber(
            onDevice: whisper,
            cloud: RemoteTranscriber(backend: backend),
            route: route,
            modelReady: { await models.installedModel != nil },
            realTimeFactor: { await benchmark.realTimeFactor },
            thresholds: thresholds
        )
        self.transcriber = transcriber
        pipeline = SpeechTranslationPipeline(
            transcriber: transcriber,
            translator: translator,
            synthesizer: synthesizer
        )
    }

    /// Signing out, or deleting the account, leaves nothing of the person on this phone: the next person to sign
    /// in must not speak in their voice. Called by the root view when the session goes.
    func forgetLocalVoice() async {
        try? await voice.delete()
        voiceSetup.skipped = false
    }

    isolated deinit {
        measureWhenReady.cancel()
    }

    /// Multilingual "base", quantised: 57 MB. The reasons are in ADR 0004.
    private static let whisperModelID = "ggml-base-q5_1"

    /// The BACKEND_URL build setting, which reaches the app through Info.plist.
    private static func backendURL(in bundle: Bundle) -> URL {
        guard
            let value = bundle.object(forInfoDictionaryKey: "BackendURL") as? String,
            let url = URL(string: value)
        else {
            preconditionFailure("BackendURL is missing from Info.plist")
        }
        return url
    }

    /// Eleven seconds of clear English speech, from whisper.cpp's samples: the clip Android measures with.
    private static func benchmarkClip(in bundle: Bundle) -> URL {
        guard let url = bundle.url(forResource: "benchmark", withExtension: "wav") else {
            preconditionFailure("benchmark.wav is missing from the app")
        }
        return url
    }

    @concurrent
    private nonisolated static func read(_ clip: URL) async throws -> PCMAudio {
        try PCMAudio(wav: Data(contentsOf: clip))
    }
}

/// Lets the request closures reach the account, which is made after them.
@MainActor
private final class AccountBox {
    weak var account: Account?
}
