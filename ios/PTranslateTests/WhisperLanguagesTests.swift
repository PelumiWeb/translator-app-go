import Testing
@testable import PTranslate

struct WhisperLanguagesTests {
    @Test func knowsWhatWhisperCanAndCannotTranscribe() {
        #expect(WhisperLanguages.supports(Language("en")))
        #expect(WhisperLanguages.supports(Language("yo")))
        #expect(WhisperLanguages.supports(Language("yue")))
        // ML Kit translates these two, but Whisper cannot transcribe them.
        #expect(!WhisperLanguages.supports(Language("eo")))
        #expect(!WhisperLanguages.supports(Language("ga")))
        #expect(!WhisperLanguages.supports(Language("")))
    }
}
