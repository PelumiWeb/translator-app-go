/// Speaks with `preferred` when it can, and with `fallback` when it cannot, so the translation is always heard. In
/// the app: the user's own voice through the server, or the phone's voice when no voice is recorded, there is no
/// network, or the server fails.
struct FallbackSpeechSynthesizer: SpeechSynthesizer {
    let preferred: any SpeechSynthesizer
    let fallback: any SpeechSynthesizer

    func speak(_ text: String, language: Language, onStart: @escaping @Sendable (SpeechVoice) -> Void) async throws(SpeechError) {
        do {
            try await preferred.speak(text, language: language, onStart: onStart)
        } catch {
            try await fallback.speak(text, language: language, onStart: onStart)
        }
    }

    func stop() async {
        await preferred.stop()
        await fallback.stop()
    }
}
