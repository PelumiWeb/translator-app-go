import Foundation

/// Builds the app's object graph by hand: every type receives what it needs through its initialiser, and this is
/// the one place that decides which implementation that is. There is no DI framework.
@MainActor
final class AppContainer {
    let recorder: any AudioRecorder = EngineAudioRecorder()
    let player: any AudioPlayer = EngineAudioPlayer()

    /// Downloads the Whisper model from the backend and keeps it in the app's files.
    let models: ModelManager

    /// Which transcriber the next recording uses. The choice on screen sets it. The server until checkpoint 10.6
    /// adds the automatic route, as the app did before it could transcribe on the phone.
    let route = RouteSetting(.cloud)

    let pipeline: SpeechTranslationPipeline

    /// The recording of the user's voice, in the app's private files.
    let voice: any VoiceSampleStore = FileVoiceSampleStore(directory: .applicationSupportDirectory.appending(path: "voice"))
    let voiceSetup: any VoiceSetupPreference = UserDefaultsVoiceSetup(defaults: .standard)

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
        pipeline = SpeechTranslationPipeline(
            transcriber: RoutingTranscriber(onDevice: whisper, cloud: RemoteTranscriber(backend: backend), route: route)
        )
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
}
