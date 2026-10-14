extension AsyncThrowingStream where Failure == any Error {
    /// A stream fed by `produce`, which runs in a task the stream owns. The task starts at once and is cancelled
    /// when the consumer stops listening, so the work never outlives its stream. This is how `Core` offers what
    /// Android offers as a `Flow`.
    static func producing(
        _ produce: @escaping @Sendable (Continuation) async throws -> Void
    ) -> AsyncThrowingStream<Element, Failure> {
        let (stream, continuation) = makeStream()
        let task = Task {
            do {
                try await produce(continuation)
                continuation.finish()
            } catch {
                continuation.finish(throwing: error)
            }
        }
        continuation.onTermination = { _ in task.cancel() }
        return stream
    }
}
