import Foundation

/// Translates text between two languages. The app depends on this protocol only; Apple's Translation framework is
/// one implementation of it.
protocol Translator: Sendable {
    /// Every language this translator can translate from and to.
    var supportedLanguages: [Language] { get async }

    /// Whether translating between these two needs no download first.
    func isReady(source: Language, target: Language) async -> Bool

    /// Downloads whatever `translate` needs for this pair. Does nothing if it is already there. Fails if the
    /// download fails or the user declines it.
    func prepare(source: Language, target: Language) async throws(TranslationError)

    /// Fails if the pair is unsupported or not prepared.
    func translate(_ text: String, source: Language, target: Language) async throws(TranslationError) -> String
}

/// Why a translation, or the download before it, did not happen, in words for the user.
struct TranslationError: LocalizedError, Equatable {
    let message: String

    var errorDescription: String? { message }
}
