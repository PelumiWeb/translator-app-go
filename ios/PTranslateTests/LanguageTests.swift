import Foundation
import Testing
@testable import PTranslate

struct LanguageTests {
    /// Entries as Apple lists them: some languages once per region, Arabic only for the Emirates.
    private let apple = ["en-US", "en-GB", "pt-BR", "pt-PT", "ar-AE", "nb-NO", "zh-Hans-CN", "zh-Hant-TW"]
        .map { Locale.Language(identifier: $0) }

    @Test func takesEachLanguageOnceFromAppleWithTheTagsTheRestOfTheAppUses() {
        #expect(Language.distinct(in: apple).map(\.tag) == ["ar", "en", "no", "pt", "zh"])
    }

    @Test func matchesTheVariantAppleTreatsAsTheLanguagesDefault() {
        #expect(Language("en").match(in: apple)?.minimalIdentifier == "en")
        #expect(Language("pt").match(in: apple)?.minimalIdentifier == "pt")
        #expect(Language("zh").match(in: apple)?.minimalIdentifier == "zh")
        // Not Arabic's default region, but the only one listed.
        #expect(Language("ar").match(in: apple)?.minimalIdentifier == "ar-AE")
        #expect(Language("no").match(in: apple)?.minimalIdentifier == "nb")
        #expect(Language("yo").match(in: apple) == nil)
    }
}
