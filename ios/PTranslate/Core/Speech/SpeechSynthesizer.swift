import Foundation

/// Says text aloud. The app depends on this protocol only: today the phone's built-in voices are behind it, later
/// a voice cloned from the speaker.
protocol SpeechSynthesizer: Sendable {
    /// Speaks `text` in `language` and returns when it has finished, or when `stop()` cut it short. Cancelling the
    /// caller stops the speech too. Fails if it cannot be spoken, for example because the phone has no voice for
    /// that language.
    func speak(_ text: String, language: Language) async throws(SpeechError)

    /// Stops whatever is being spoken. Does nothing if nothing is.
    func stop()
}

/// Why something could not be spoken, in words for the user.
struct SpeechError: LocalizedError, Equatable {
    let message: String

    var errorDescription: String? { message }
}
