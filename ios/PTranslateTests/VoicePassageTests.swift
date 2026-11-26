import Foundation
import Testing
@testable import PTranslate

struct VoicePassageTests {
    @Test func theEnglishPassageIsWordForWordTheServers() {
        // provider.OnboardingPassage, which the server assumes for a recording that comes without its words.
        #expect(VoicePassage.text(for: .english) == "When I travel, I like to talk with the people I meet about their food, their music, "
            + "and the places they love. A good conversation makes the world feel smaller. I am recording this so that "
            + "when my words are translated, they can still be spoken in my own voice.")
    }

    @Test func eachLanguageReadsItsOwnPassage() {
        #expect(VoicePassage.lines(for: Language("es")).first == "Cuando viajo, me gusta hablar con la gente que conozco")
        #expect(VoicePassage.lines(for: Language("yo")) == VoicePassage.translations["yo"])
    }

    @Test(arguments: [Language("cy"), nil])
    func aLanguageWithoutATranslationReadsTheEnglish(_ language: Language?) {
        #expect(VoicePassage.lines(for: language) == VoicePassage.english)
        #expect(VoicePassage.text(for: language) == VoicePassage.text(for: .english))
    }

    @Test func everyTranslationIsForALanguageThatCanBeChosenAndHasTheTeleprompterSLines() {
        for (tag, lines) in VoicePassage.translations {
            #expect(WhisperLanguages.supports(Language(tag)), "\(tag) cannot be chosen at sign-up")
            #expect(lines.count == VoicePassage.english.count, "\(tag) has \(lines.count) lines")
            #expect(lines.allSatisfy { !$0.trimmingCharacters(in: .whitespaces).isEmpty }, "\(tag) has an empty line")
        }
    }

    @Test func linesJoinWithSpacesExceptWhereTheLanguageUsesNone() {
        #expect(VoicePassage.text(for: Language("fr")).contains("rencontre de leur cuisine"))
        #expect(VoicePassage.text(for: Language("zh")).contains("聊天，聊他们"))
    }
}
