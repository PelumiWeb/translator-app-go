/// Runs `call` until it succeeds, waiting `retryDelay` between tries. Gives up at once when the server refused the
/// request, since the same request gets the same answer, and after `maxAttempts` tries in all. Throws the last
/// failure.
func retrying<T>(
    maxAttempts: Int,
    retryDelay: (_ attempt: Int) -> Duration,
    _ call: () async throws -> T
) async throws -> T {
    var attempt = 1
    while true {
        do {
            return try await call()
        } catch let error as BackendError {
            guard error.retryable, attempt < maxAttempts else { throw error }
        }
        try await Task.sleep(for: retryDelay(attempt))
        attempt += 1
    }
}

extension BackendClient {
    /// Follows a job to its end, reconnecting when the stream breaks. Each reconnect tells the server the last
    /// event already received, and the server continues from the next one, so nothing is repeated or skipped.
    ///
    /// `onEvent` sees each event once, in order, up to and including `.done`. `onReconnecting` runs before each new
    /// connection. Throws a `BackendError` when the job failed (with its message), when the server refused the
    /// stream, or after `maxAttempts` connections in a row that brought nothing new.
    func followJob(
        _ jobID: String,
        maxAttempts: Int,
        retryDelay: (_ attempt: Int) -> Duration,
        onReconnecting: () -> Void = {},
        onEvent: (JobEvent) -> Void
    ) async throws {
        var lastSeq = 0
        var failures = 0 // in a row, without any new event in between

        while true {
            let seqBefore = lastSeq
            do {
                for try await update in try await jobEvents(jobID: jobID, afterSeq: lastSeq) {
                    // A guard for a server that replays more than it was asked to: never pass on the same event
                    // twice.
                    guard update.seq > lastSeq else { continue }
                    lastSeq = update.seq

                    switch update.event {
                    case .failed(let message):
                        // The job itself failed. Reconnecting would only replay the same ending.
                        throw BackendError(message: message, retryable: false)
                    case .done:
                        onEvent(update.event)
                        return
                    default:
                        onEvent(update.event)
                    }
                }
            } catch let error as BackendError {
                guard error.retryable else { throw error }
            }

            // The stream ended, cleanly or not, before the job did. The server closes a stream only after "done"
            // or "error", so this was the connection, a server restart, or a client that fell behind. In every
            // case the answer is the same: ask again.
            if lastSeq > seqBefore {
                failures = 0 // it was making progress
            }
            failures += 1
            if failures >= maxAttempts {
                throw BackendError(message: "The connection to the server was lost", retryable: true)
            }
            onReconnecting()
            try await Task.sleep(for: retryDelay(failures))
        }
    }
}
