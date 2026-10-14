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
            let jobID = try await upload(audio.wavData(), language: language)
            try await follow(jobID, events: events)
        }
    }

    /// Uploads until the server answers. Every attempt carries the same key, so if an earlier one did arrive and
    /// only its response was lost, the server hands back that job instead of creating a second.
    private func upload(_ wav: Data, language: Language) async throws -> String {
        let idempotencyKey = UUID().uuidString
        var attempt = 1
        while true {
            do {
                return try await backend.createJob(wav: wav, sourceLanguage: language, idempotencyKey: idempotencyKey)
            } catch let error as BackendError {
                guard error.retryable, attempt < maxAttempts else {
                    throw TranscriptionError.failed(error.message)
                }
            }
            try await Task.sleep(for: retryDelay(attempt))
            attempt += 1
        }
    }

    /// Follows the job to its end, reconnecting when the stream breaks. Each reconnect tells the server the last
    /// event already received, and the server continues from the next one, so nothing is repeated or skipped.
    private func follow(
        _ jobID: String,
        events: AsyncThrowingStream<TranscriptEvent, any Error>.Continuation
    ) async throws {
        var lastSeq = 0
        var failures = 0 // in a row, without any new event in between

        while true {
            let seqBefore = lastSeq
            do {
                for try await update in try await backend.jobEvents(jobID: jobID, afterSeq: lastSeq) {
                    // A guard for a server that replays more than it was asked to: never show the same event
                    // twice.
                    guard update.seq > lastSeq else { continue }
                    lastSeq = update.seq

                    switch update.event {
                    case .queued: events.yield(.stageChanged(.queued))
                    case .processing: events.yield(.stageChanged(.processing))
                    case .partial(let text): events.yield(.partial(text))
                    case .done(let text, _):
                        events.yield(.final(Transcript(text: text, confidence: nil, source: .cloud)))
                        return
                    case .failed(let message):
                        // The job itself failed. Reconnecting would only replay the same ending.
                        throw TranscriptionError.failed(message)
                    case .unknown: break
                    }
                }
            } catch let error as BackendError {
                guard error.retryable else { throw TranscriptionError.failed(error.message) }
            }

            // The stream ended, cleanly or not, before the job did. The server closes a stream only after "done"
            // or "error", so this was the connection, a server restart, or a client that fell behind. In every
            // case the answer is the same: ask again.
            if lastSeq > seqBefore {
                failures = 0 // it was making progress
            }
            failures += 1
            if failures >= maxAttempts {
                throw TranscriptionError.failed("The connection to the server was lost")
            }
            events.yield(.stageChanged(.reconnecting))
            try await Task.sleep(for: retryDelay(failures))
        }
    }
}
