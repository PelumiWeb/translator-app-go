import AVFAudio
import Foundation
import Synchronization
import Testing
@testable import PTranslate

/// Runs the phone's real speech synthesizer, which works in the simulator too. The tests speak aloud briefly.
@Suite(.serialized)
struct SystemSpeechSynthesizerTests {
    private let synthesizer = SystemSpeechSynthesizer()

    @Test(.timeLimit(.minutes(1)))
    func speaksAndReturnsWhenFinished() async throws {
        let started = Mutex<[SpeechVoice]>([])

        // Returning at all, without an error, is the result: a voice was found and the utterance reported done.
        try await synthesizer.speak("Good morning.", language: .english, onStart: { voice in started.withLock { $0.append(voice) } })

        #expect(started.withLock { $0 } == [.phone], "reported as started once, in the phone's voice")
    }

    @Test(.timeLimit(.minutes(1)))
    func stopEndsALongUtteranceEarly() async throws {
        let long = String(repeating: "This sentence is repeated many times. ", count: 40) // about a minute of speech
        let clock = ContinuousClock()
        let started = clock.now

        async let speaking: Void = synthesizer.speak(long, language: .english)
        try await Task.sleep(for: .milliseconds(1_500))
        synthesizer.stop()
        try await speaking

        let elapsed = clock.now - started
        #expect(elapsed < .seconds(15), "speaking took \(elapsed); stop did not cut it short")
    }

    @Test func aLanguageWithNoVoiceIsReported() async {
        // Klingon: a valid language tag that no phone ships a voice for.
        await #expect(throws: SpeechError(message: "This iPhone has no voice for Klingon")) {
            try await synthesizer.speak("nuqneH", language: Language("tlh"))
        }
    }

    @Test func speaksALanguageInTheVoiceOfItsDefaultRegion() throws {
        let english = try #require(SystemSpeechSynthesizer.voice(for: .english))
        let spanish = try #require(SystemSpeechSynthesizer.voice(for: .spanish))

        #expect(english.language == "en-US")
        #expect(spanish.language == "es-ES")
    }
}
