import Foundation
import Translation

/// On-device translation with Apple's Translation framework, the translator behind Apple's Translate app.
///
/// iOS keeps the language packs and shares them with that app. It downloads one only from a screen, after asking
/// the user in its own sheet, so `prepare` hands the download to `download`, which the app routes to its root view
/// (see `TranslationDownloads`). Translating needs no screen.
///
/// Apple lists some languages once per region (en-US, en-GB ...). Each is offered once, as the variant iOS treats as
/// that language's default: English is American English, Portuguese is Brazilian, Chinese is simplified.
struct AppleTranslator: Translator {
    private let download: @Sendable (Locale.Language, Locale.Language) async throws -> Void

    /// - Parameter download: asks iOS for the packs a pair needs, and returns once they are installed. Throws if
    ///   the user declines or the download fails.
    init(download: @escaping @Sendable (Locale.Language, Locale.Language) async throws -> Void) {
        self.download = download
    }

    var supportedLanguages: [Language] {
        get async { Self.languages(in: await Self.supported()) }
    }

    func isReady(source: Language, target: Language) async -> Bool {
        guard let (from, to) = try? await Self.pair(source, target) else { return false }
        return await Self.status(from, to) == .installed
    }

    func prepare(source: Language, target: Language) async throws(TranslationError) {
        let (from, to) = try await Self.pair(source, target)
        switch await Self.status(from, to) {
        case .installed:
            return
        case .supported:
            break
        case .unsupported:
            throw Self.cannotTranslate
        @unknown default:
            break // a state added after this was written: iOS's own download decides what is needed
        }
        do {
            try await download(from, to)
        } catch {
            throw TranslationError(message: "The language pack was not downloaded")
        }
    }

    func translate(_ text: String, source: Language, target: Language) async throws(TranslationError) -> String {
        let (from, to) = try await Self.pair(source, target)
        return try await Self.translate(text, from, to)
    }

    /// The languages in Apple's list, once each, with the tags the rest of the app uses.
    static func languages(in supported: [Locale.Language]) -> [Language] {
        Set(supported.compactMap { $0.languageCode?.identifier })
            .map { Language(appTags[$0] ?? $0) }
            .sorted { $0.tag < $1.tag }
    }

    /// The entry in Apple's list that stands for `language`: the one iOS treats as the language's default, which
    /// needs no region to name it, or else the first one listed.
    static func match(_ language: Language, in supported: [Locale.Language]) -> Locale.Language? {
        let code = appleCodes[language.tag] ?? language.tag
        let candidates = supported.filter { $0.languageCode?.identifier == code }
        return candidates.first { $0.minimalIdentifier == code } ?? candidates.first
    }

    // Norwegian is "nb" (Bokmål) to Apple, and "no" to Whisper and to ML Kit on Android.
    private static let appTags = ["nb": "no"]
    private static let appleCodes = ["no": "nb"]

    private static func pair(
        _ source: Language,
        _ target: Language
    ) async throws(TranslationError) -> (Locale.Language, Locale.Language) {
        let supported = await supported()
        guard let from = match(source, in: supported) else { throw unsupported(source) }
        guard let to = match(target, in: supported) else { throw unsupported(target) }
        return (from, to)
    }

    private static func unsupported(_ language: Language) -> TranslationError {
        TranslationError(message: "Translation does not support \(language.tag)")
    }

    private static var cannotTranslate: TranslationError {
        #if targetEnvironment(simulator)
        // Seen on the iOS 26.4 simulator: it lists Apple's languages, then reports every pair as unsupported.
        TranslationError(message: "Apple's translation does not work in the simulator. Try it on an iPhone")
        #else
        TranslationError(message: "This iPhone cannot translate between these two languages")
        #endif
    }

    // The framework's objects are not Sendable, so each is made inside a @concurrent function and never leaves it:
    // that way it never crosses from one actor to another.

    @concurrent
    private static func supported() async -> [Locale.Language] {
        await LanguageAvailability().supportedLanguages
    }

    @concurrent
    private static func status(_ source: Locale.Language, _ target: Locale.Language) async -> LanguageAvailability.Status {
        await LanguageAvailability().status(from: source, to: target)
    }

    @concurrent
    private static func translate(
        _ text: String,
        _ source: Locale.Language,
        _ target: Locale.Language
    ) async throws(TranslationError) -> String {
        let session = TranslationSession(installedSource: source, target: target)
        do {
            return try await session.translate(text).targetText
        } catch Translation.TranslationError.notInstalled {
            throw TranslationError(message: "The language pack is not on this iPhone")
        } catch {
            throw TranslationError(message: "Translation failed")
        }
    }
}
