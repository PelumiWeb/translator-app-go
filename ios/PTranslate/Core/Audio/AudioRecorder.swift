import Foundation

protocol AudioRecorder {
    /// Starts capturing from the microphone. The caller must already have the user's permission.
    ///
    /// - Throws: `RecorderError` if the microphone cannot be opened.
    func start() throws -> any Recording
}

/// A capture in progress. Finish it with exactly one of `stop()` or `cancel()`.
protocol Recording: AnyObject {
    /// Stops listening for now, keeping what was recorded. The microphone is off until `resume()`.
    func pause()

    /// Listens again after `pause()`, adding to what was recorded. If the microphone cannot be had again, as during a
    /// phone call, the recording keeps what it has.
    func resume()

    /// Stops capturing and returns what was recorded.
    func stop() -> PCMAudio

    /// Stops capturing and throws the audio away.
    func cancel()
}

struct RecorderError: LocalizedError {
    let message: String

    var errorDescription: String? { message }
}
