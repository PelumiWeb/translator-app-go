import Foundation

/// Builds the app's object graph by hand: every type receives what it needs through its initialiser, and this is
/// the one place that decides which implementation that is. There is no DI framework.
final class AppContainer {
    let recorder: any AudioRecorder = EngineAudioRecorder()
    let player: any AudioPlayer = EngineAudioPlayer()
    let pipeline: SpeechTranslationPipeline

    /// The recording of the user's voice, in the app's private files.
    let voice: any VoiceSampleStore = FileVoiceSampleStore(directory: .applicationSupportDirectory.appending(path: "voice"))
    let voiceSetup: any VoiceSetupPreference = UserDefaultsVoiceSetup(defaults: .standard)

    init(bundle: Bundle = .main) {
        let backend = BackendClient(baseURL: Self.backendURL(in: bundle), session: URLSession(configuration: .default))
        pipeline = SpeechTranslationPipeline(transcriber: RemoteTranscriber(backend: backend))
    }

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
