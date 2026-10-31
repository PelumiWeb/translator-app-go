import Foundation

/// Says text aloud. The app depends on this protocol only: today the phone's built-in voices are behind it, later
/// a voice cloned from the speaker.
protocol SpeechSynthesizer: Sendable {
    /// Speaks `text` in `language` and returns when it has finished, or when `stop()` cut it short. Cancelling the
    /// caller stops it too, as far as each implementation can. Fails if it cannot be spoken, for example because
    /// the phone has no voice for that language.
    ///
    /// `onStart` is called once, when the speech becomes audible, with the voice it is in. That can be seconds after
    /// the call: a voice made on the server takes that long. It may be called on another thread.
    func speak(_ text: String, language: Language, onStart: @escaping @Sendable (SpeechVoice) -> Void) async throws(SpeechError)

    /// Stops whatever is being spoken. Does nothing if nothing is. Async, unlike Android's, because a voice may be
    /// playing on the main actor, which another thread can reach only by waiting for it.
    func stop() async
}

extension SpeechSynthesizer {
    func speak(_ text: String, language: Language) async throws(SpeechError) {
        try await speak(text, language: language, onStart: { _ in })
    }
}

/// Whose voice a translation is spoken in.
enum SpeechVoice: Equatable, Sendable {
    /// The user's own, made on the server from their recording.
    case own

    /// One of the phone's built-in voices.
    case phone
}

/// Why something could not be spoken, in words for the user.
struct SpeechError: LocalizedError, Equatable {
    let message: String

    var errorDescription: String? { message }
}
