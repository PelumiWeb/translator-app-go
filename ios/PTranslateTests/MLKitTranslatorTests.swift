import Foundation
import Testing
@testable import PTranslate

/// Runs ML Kit for real, the same cases as Android's device test. The first run downloads the Spanish language
/// pack (about 35 MB), so the simulator needs a network connection.
struct MLKitTranslatorTests {
    private let translator = MLKitTranslator()

    @Test func listsTheLanguagesItSupports() {
        let tags = translator.supportedLanguages.map(\.tag)

        #expect(Set(["en", "es", "fr", "ar", "zh"]).isSubset(of: tags), "languages: \(tags)")
        #expect(tags == tags.sorted())
    }

    @Test func translatesEnglishToSpanish() async throws {
        try await translator.prepare(source: .english, target: .spanish)
        #expect(await translator.isReady(source: .english, target: .spanish), "ready after prepare")

        let translated = try await translator.translate("Good morning, how are you?", source: .english, target: .spanish)

        #expect(translated.localizedCaseInsensitiveContains("buenos días"), "translation: \(translated)")
    }

    @Test func rejectsALanguageMLKitDoesNotHave() async {
        await #expect(throws: TranslationError(message: "Translation does not support yo")) {
            try await translator.translate("hello", source: .english, target: Language("yo"))
        }
    }
}
