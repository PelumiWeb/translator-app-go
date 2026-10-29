import Synchronization

enum TranscriptionRoute: CaseIterable, Sendable {
    /// Use the phone when it can do the job well, the server when it cannot.
    case automatic
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

/// The numbers the automatic route decides by, the same as Android's. They are starting points chosen without
/// measurements from real phones; see ADR 0007 before trusting them.
struct FallbackThresholds: Sendable {
    /// A phone result with confidence below this is sent to the server as well.
    var minConfidence: Float = 0.6

    /// A phone slower than this is not used. The real-time factor is time taken divided by audio length: at 1.0,
    /// ten seconds of speech take ten seconds to transcribe.
    var maxRealTimeFactor: Double = 1.0
}

/// Sends each recording to the phone, the server, or both in turn.
///
/// `.onDevice` and `.cloud` do exactly that. `.automatic` prefers the phone, because it is private, free and works
/// offline, and falls back to the server in three cases:
///
/// 1. There is no model on the phone.
/// 2. The phone is too slow, by `realTimeFactor`.
/// 3. The phone tried and the result is poor: low confidence, no words found, repeated nonsense, or the model
///    failed.
///
/// The server is a fallback, not a requirement. Whenever it cannot be reached and the phone has, or can produce, a
/// result, that result is used.
struct RoutingTranscriber: Transcriber {
    let onDevice: any Transcriber
    let cloud: any Transcriber
    let route: RouteSetting

    /// Whether the on-device model is installed.
    let modelReady: @Sendable () async -> Bool

    /// This phone's measured speed, or nil if it has not been measured.
    let realTimeFactor: @Sendable () async -> Double?

    var thresholds = FallbackThresholds()

    func transcribe(_ audio: PCMAudio, language: Language) -> AsyncThrowingStream<TranscriptEvent, any Error> {
        switch route.current {
        case .automatic: automatic(audio, language: language)
        case .onDevice: onDevice.transcribe(audio, language: language)
        case .cloud: cloud.transcribe(audio, language: language)
        }
    }

    private func automatic(_ audio: PCMAudio, language: Language) -> AsyncThrowingStream<TranscriptEvent, any Error> {
        .producing { events in
            if await !modelReady() {
                // Nothing to fall back on: if the server fails, so does this.
                try await forward(cloud.transcribe(audio, language: language), to: events, note: .noModel)
                return
            }

            if let speed = await realTimeFactor(), speed > thresholds.maxRealTimeFactor {
                do {
                    try await forward(cloud.transcribe(audio, language: language), to: events, note: .slowDevice)
                } catch is TranscriptionError {
                    // Offline, most likely. Slow is better than nothing.
                    try await forward(onDevice.transcribe(audio, language: language), to: events, note: .cloudUnavailable)
                }
                return
            }

            // The usual case: try the phone, and judge what it produced.
            var local: Transcript?
            var localFailure: TranscriptionError?
            do {
                for try await event in onDevice.transcribe(audio, language: language) {
                    // The final result is held back until it has been judged.
                    if case .final(let transcript) = event {
                        local = transcript
                    } else {
                        events.yield(event)
                    }
                }
            } catch let error as TranscriptionError {
                // Nothing was recorded: uploading silence helps nobody.
                if case .silentAudio = error {
                    throw error
                }
                localFailure = error
            }

            let reason: RoutingNote
            if let local, localFailure == nil {
                // No confidence figure counts as confident: there is nothing to doubt it with.
                if (local.confidence ?? 1) >= thresholds.minConfidence {
                    events.yield(.final(local))
                    return
                }
                reason = .lowConfidence
            } else {
                reason = switch localFailure {
                case .noSpeech?: .noSpeechOnDevice
                case .garbled?: .garbledOnDevice
                default: .onDeviceFailed
                }
            }

            // Show the doubtful text while the server works, rather than nothing.
            if let local {
                events.yield(.partial(local.text))
            }

            do {
                try await forward(cloud.transcribe(audio, language: language), to: events, note: reason)
            } catch let error as TranscriptionError {
                // The server could not help. A doubtful result still beats an error; with no result at all, report
                // what the phone said.
                guard var result = local else {
                    throw localFailure ?? error
                }
                result.note = .cloudUnavailable
                events.yield(.final(result))
            }
        }
    }

    /// Passes a transcriber's events on, marking its final result with `note`.
    private func forward(
        _ stream: AsyncThrowingStream<TranscriptEvent, any Error>,
        to events: AsyncThrowingStream<TranscriptEvent, any Error>.Continuation,
        note: RoutingNote
    ) async throws {
        for try await event in stream {
            if case .final(var transcript) = event {
                transcript.note = note
                events.yield(.final(transcript))
            } else {
                events.yield(event)
            }
        }
    }
}
