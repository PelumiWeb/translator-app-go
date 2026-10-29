import Synchronization

enum TranscriptionRoute: CaseIterable, Sendable {
    case onDevice
    case cloud
}

/// The route the next recording takes. Set by the screen, read by `RoutingTranscriber` from whatever thread it
/// runs on, hence the lock.
final class RouteSetting: Sendable {
    private let value: Mutex<TranscriptionRoute>

    init(_ route: TranscriptionRoute) {
        value = Mutex(route)
    }

    var current: TranscriptionRoute {
        get { value.withLock { $0 } }
        set { value.withLock { $0 = newValue } }
    }
}

/// Sends each recording to the phone or to the server, as chosen on screen. The automatic route, which picks
/// between them per recording, joins at checkpoint 10.6.
struct RoutingTranscriber: Transcriber {
    let onDevice: any Transcriber
    let cloud: any Transcriber
    let route: RouteSetting

    func transcribe(_ audio: PCMAudio, language: Language) -> AsyncThrowingStream<TranscriptEvent, any Error> {
        switch route.current {
        case .onDevice: onDevice.transcribe(audio, language: language)
        case .cloud: cloud.transcribe(audio, language: language)
        }
    }
}
