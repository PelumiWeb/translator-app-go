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

    /// Which transcriber the next recording uses. The choice on screen sets it.
    let route = RouteSetting(.automatic)

    let thresholds = FallbackThresholds()

    /// How fast this phone runs the model, measured once per model.
    let speed: any DeviceSpeed

    /// Language pack downloads, which iOS lets only a screen start. The app's root view runs them.
    let translationDownloads: TranslationDownloads

    let translator: any Translator

    /// Says translations aloud in one of the phone's own voices.
    let synthesizer: any SpeechSynthesizer = SystemSpeechSynthesizer()

    let pipeline: SpeechTranslationPipeline

    /// The recording of the user's voice, in the app's private files.
    let voice: any VoiceSampleStore = FileVoiceSampleStore(directory: .applicationSupportDirectory.appending(path: "voice"))
    let voiceSetup: any VoiceSetupPreference = UserDefaultsVoiceSetup(defaults: .standard)

    /// Whenever a model becomes available, makes sure its speed is known. Lives as long as the app.
    private let measureWhenReady: Task<Void, Never>

    init(bundle: Bundle = .main) {
        let backend = BackendClient(baseURL: Self.backendURL(in: bundle), session: URLSession(configuration: .default))
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
            for await model in Observations({ models.installedModel }) where model != nil {
                await benchmark.ensureMeasured()
            }
        }

        let downloads = TranslationDownloads()
        translationDownloads = downloads
        let translator = AppleTranslator(download: { source, target in
            try await downloads.download(from: source, to: target)
        })
        self.translator = translator

        pipeline = SpeechTranslationPipeline(
            transcriber: RoutingTranscriber(
                onDevice: whisper,
                cloud: RemoteTranscriber(backend: backend),
                route: route,
                modelReady: { await models.installedModel != nil },
                realTimeFactor: { await benchmark.realTimeFactor },
                thresholds: thresholds
            ),
            translator: translator,
            synthesizer: synthesizer
        )
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
