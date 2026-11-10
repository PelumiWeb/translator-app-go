import Foundation
// @preconcurrency: ML Kit is Objective-C and marks none of its types Sendable. Each of its objects is made inside
// the @concurrent function that uses it and never leaves it, so none is shared between threads.
@preconcurrency import MLKitCommon
@preconcurrency import MLKitTranslate

/// On-device translation with Google's ML Kit, the same translator as on Android (ADR 0024).
///
/// ML Kit keeps one language pack (about 35 MB) per language and downloads and stores them itself, so they do not
/// go through this app's model manager. It translates through English: Japanese to French is Japanese to English,
/// then English to French, and needs both packs. English needs none.
struct MLKitTranslator: Translator {
    var supportedLanguages: [Language] {
        TranslateLanguage.allLanguages().map { Language($0.rawValue) }.sorted { $0.tag < $1.tag }
    }

    func isReady(source: Language, target: Language) async -> Bool {
        guard let (from, to) = try? Self.pair(source, target) else { return false }
        let (sourceReady, targetReady) = await (Self.isDownloaded(from), Self.isDownloaded(to))
        return sourceReady && targetReady
    }

    func prepare(source: Language, target: Language) async throws(TranslationError) {
        // Outside the download: an unsupported language has its own, clearer error.
        let (from, to) = try Self.pair(source, target)
        try await Self.download(from, to)
    }

    func translate(_ text: String, source: Language, target: Language) async throws(TranslationError) -> String {
        let (from, to) = try Self.pair(source, target)
        return try await Self.translate(text, from, to)
    }

    /// ML Kit's codes for the two languages, which are the tags the rest of the app uses.
    private static func pair(_ source: Language, _ target: Language) throws(TranslationError) -> (String, String) {
        let supported = Set(TranslateLanguage.allLanguages().map(\.rawValue))
        for language in [source, target] where !supported.contains(language.tag) {
            throw TranslationError(message: "Translation does not support \(language.tag)")
        }
        return (source.tag, target.tag)
    }

    @concurrent
    private static func isDownloaded(_ code: String) async -> Bool {
        let model = TranslateRemoteModel.translateRemoteModel(language: TranslateLanguage(rawValue: code))
        // ML Kit's model manager, not the app's own, which keeps the Whisper model.
        return MLKitCommon.ModelManager.modelManager().isModelDownloaded(model)
    }

    @concurrent
    private static func download(_ source: String, _ target: String) async throws(TranslationError) {
        // Over mobile data too, as on Android: the user is waiting for this translation.
        let conditions = ModelDownloadConditions(allowsCellularAccess: true, allowsBackgroundDownloading: false)
        do {
            try await client(source, target).downloadModelIfNeeded(with: conditions)
        } catch {
            throw TranslationError(message: "Could not download the language pack. Check the connection")
        }
    }

    @concurrent
    private static func translate(_ text: String, _ source: String, _ target: String) async throws(TranslationError) -> String {
        do {
            return try await client(source, target).translate(text)
        } catch {
            throw TranslationError(message: "Translation failed")
        }
    }

    /// ML Kit's translator for a pair. ML Kit keeps one per pair and hands back the same one each time, so unlike
    /// on Android there is nothing to cache here. Asking for it is thread safe, by ML Kit's documentation.
    private static func client(_ source: String, _ target: String) -> MLKitTranslate.Translator {
        let options = TranslatorOptions(
            sourceLanguage: TranslateLanguage(rawValue: source),
            targetLanguage: TranslateLanguage(rawValue: target)
        )
        return MLKitTranslate.Translator.translator(options: options)
    }
}
