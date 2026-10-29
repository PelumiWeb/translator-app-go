import Testing
@testable import PTranslate

/// The same cases as the Android tests: both phones must judge a transcript the same way.
struct TranscriptChecksTests {
    // The two outputs that reached the screen, and were read aloud, on Android before this check existed.
    @Test func catchesTheRepetitionLoopsWhisperActuallyProduced() {
        let english = Array(repeating: "I,", count: 110).joined(separator: " ")
        let spanish = "Yo, " + Array(repeating: "yo,", count: 140).joined(separator: " ")

        #expect(looksLikeRepetitionLoop(english), "ratio \(compressionRatio(english))")
        #expect(looksLikeRepetitionLoop(spanish), "ratio \(compressionRatio(spanish))")
    }

    @Test func catchesARepeatedPhraseAsWellAsARepeatedWord() {
        let loop = Array(repeating: "Thank you for watching.", count: 25).joined(separator: " ")

        #expect(looksLikeRepetitionLoop(loop), "ratio \(compressionRatio(loop))")
    }

    @Test(arguments: [
        "Good morning, how are you?",
        "And so my fellow Americans, ask not what your country can do for you, ask what you can do for your country.",
        "Can you tell me where the nearest pharmacy is? I need something for a headache and I do not speak the language.",
        "Buenos días, ¿cómo estás? Me gustaría reservar una mesa para dos personas esta noche a las ocho.",
        // Long, and naturally repetitive in its wording.
        "I went to the market and I bought some rice, and then I went to the bank and I paid the bill, "
            + "and then I went home and I cooked the rice, and then I sat down and I ate it.",
    ])
    func leavesOrdinarySpeechAlone(sentence: String) {
        #expect(!looksLikeRepetitionLoop(sentence), "ratio \(compressionRatio(sentence))")
    }

    // People do repeat themselves. A handful of repeats must not be thrown away.
    @Test(arguments: ["No, no, no, no!", "Yes yes yes", "Wait, wait, wait, wait, wait.", "Go go go go go go"])
    func leavesShortDeliberateRepetitionAlone(phrase: String) {
        #expect(!looksLikeRepetitionLoop(phrase), "ratio \(compressionRatio(phrase))")
    }

    // "[Music]" reached the screen as a transcript on Android before this check existed.
    @Test(arguments: ["[Music]", "[BLANK_AUDIO]", "(applause)", " [Music] ", "[Music] [Applause]", "(wind blowing) [Music]", "[ Silence ]"])
    func textThatIsOnlyLabelsForSoundsHasNoWordsInIt(text: String) {
        #expect(isOnlySoundLabels(text))
    }

    @Test(arguments: ["Hello [laughs] there", "Good morning.", "[Music] Good morning everyone", "It costs five dollars (about four euros).", ""])
    func speechIsKeptEvenWhenItHasALabelOrBracketsInIt(text: String) {
        #expect(!isOnlySoundLabels(text))
    }

    @Test func anEmptyTranscriptIsNotALoop() {
        #expect(!looksLikeRepetitionLoop(""))
    }
}
