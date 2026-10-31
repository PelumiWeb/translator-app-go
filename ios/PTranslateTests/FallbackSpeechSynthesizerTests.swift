import Testing
@testable import PTranslate

struct FallbackSpeechSynthesizerTests {
    private func speak(_ synthesizer: FallbackSpeechSynthesizer) async throws(SpeechError) {
        try await synthesizer.speak("buenos días", language: Language("es"))
    }

    @Test func speaksInThePreferredVoiceWhenItCan() async throws {
        let ownVoice = FakeSynthesizer()
        let phoneVoice = FakeSynthesizer()

        try await speak(FallbackSpeechSynthesizer(preferred: ownVoice, fallback: phoneVoice))

        #expect(ownVoice.spoken == ["es: buenos días"])
        #expect(phoneVoice.spoken.isEmpty)
    }

    @Test func fallsBackWhenThePreferredVoiceCannotSpeak() async throws {
        let ownVoice = FakeSynthesizer(failure: SpeechError(message: "Could not reach the server"))
        let phoneVoice = FakeSynthesizer()

        try await speak(FallbackSpeechSynthesizer(preferred: ownVoice, fallback: phoneVoice))

        #expect(phoneVoice.spoken == ["es: buenos días"])
    }

    @Test func reportsTheFallbacksFailureWhenNeitherCanSpeak() async {
        let synthesizer = FallbackSpeechSynthesizer(
            preferred: FakeSynthesizer(failure: SpeechError(message: "Could not reach the server")),
            fallback: FakeSynthesizer(failure: SpeechError(message: "This iPhone has no voice for Spanish"))
        )

        // The one that says why nothing at all was heard.
        await #expect(throws: SpeechError(message: "This iPhone has no voice for Spanish")) {
            try await speak(synthesizer)
        }
    }

    @Test func stopStopsBoth() async {
        let ownVoice = FakeSynthesizer()
        let phoneVoice = FakeSynthesizer()

        await FallbackSpeechSynthesizer(preferred: ownVoice, fallback: phoneVoice).stop()

        #expect(ownVoice.stops == 1)
        #expect(phoneVoice.stops == 1)
    }
}
