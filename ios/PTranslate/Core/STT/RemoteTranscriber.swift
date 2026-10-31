import Foundation

/// Transcribes on the backend: uploads the audio, then follows the job's event stream. Both steps survive a
/// connection that drops, which on a phone is ordinary.
struct RemoteTranscriber: Transcriber {
    private let backend: BackendClient
    private let maxAttempts: Int
    private let retryDelay: @Sendable (_ attempt: Int) -> Duration

    /// - Parameters:
    ///   - maxAttempts: how many times in a row the upload, or the stream, may fail before giving up.
    ///   - retryDelay: how long to wait before try number `attempt + 1`.
    init(
        backend: BackendClient,
        maxAttempts: Int = 5,
        retryDelay: @escaping @Sendable (_ attempt: Int) -> Duration = { .seconds($0) }
    ) {
        self.backend = backend
        self.maxAttempts = maxAttempts
        self.retryDelay = retryDelay
    }

    func transcribe(_ audio: PCMAudio, language: Language) -> AsyncThrowingStream<TranscriptEvent, any Error> {
        .producing { events in
            events.yield(.stageChanged(.uploading))
            let wav = audio.wavData()
            // Every attempt carries the same key, so if an earlier one did arrive and only its response was lost,
            // the server hands back that job instead of creating a second.
            let idempotencyKey = UUID().uuidString
            do {
                let jobID = try await retrying(maxAttempts: maxAttempts, retryDelay: retryDelay) {
                    try await backend.createJob(wav: wav, sourceLanguage: language, idempotencyKey: idempotencyKey)
                }
                try await backend.followJob(
                    jobID,
                    maxAttempts: maxAttempts,
                    retryDelay: retryDelay,
                    onReconnecting: { events.yield(.stageChanged(.reconnecting)) }
                ) { event in
                    switch event {
                    case .queued: events.yield(.stageChanged(.queued))
                    case .processing: events.yield(.stageChanged(.processing))
                    case .partial(let text): events.yield(.partial(text))
                    case .done(let text, _): events.yield(.final(Transcript(text: text, confidence: nil, source: .cloud)))
                    case .failed, .unknown: break
                    }
                }
            } catch let error as BackendError {
                throw TranscriptionError.failed(error.message)
            }
        }
    }
}
