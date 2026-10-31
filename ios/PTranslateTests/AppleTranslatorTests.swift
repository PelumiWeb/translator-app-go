import Foundation
import Testing
@testable import PTranslate

struct AppleTranslatorTests {
    private let translator = AppleTranslator(download: { _, _ in })

    // These two ask the real framework, which lists its languages on the simulator too.

    @Test func listsTheLanguagesIOSTranslates() async {
        let tags = await translator.supportedLanguages.map(\.tag)

        #expect(Set(["en", "es", "fr", "zh", "no"]).isSubset(of: tags), "languages: \(tags)")
        #expect(tags == tags.sorted())
    }

    @Test func rejectsALanguageIOSDoesNotHave() async {
        await #expect(throws: TranslationError(message: "Translation does not support yo")) {
            try await translator.translate("hello", source: .english, target: Language("yo"))
        }
    }

    /// Apple's translation does not run in the simulator, so this one runs only on an iPhone that has English and
    /// Spanish downloaded (Settings, Apps, Translate, Downloaded Languages).
    @Test(.enabled(if: !runningInSimulator, "Apple's translation does not run in the simulator"))
    func translatesEnglishToSpanishOnAnIPhone() async throws {
        #expect(await translator.isReady(source: .english, target: .spanish))

        let translated = try await translator.translate("Good morning, how are you?", source: .english, target: .spanish)

        #expect(translated.localizedCaseInsensitiveContains("buenos días"), "translation: \(translated)")
    }
}

private var runningInSimulator: Bool {
    #if targetEnvironment(simulator)
    true
    #else
    false
    #endif
}
