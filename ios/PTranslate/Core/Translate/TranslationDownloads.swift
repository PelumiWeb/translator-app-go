import Observation
// @preconcurrency: SwiftUI hands `run` its session on the main actor, and Apple's documented use is to call the
// session from there, but the framework does not mark its types Sendable, so Swift 6 would refuse the call.
@preconcurrency import Translation

/// Language pack downloads, which iOS lets only a screen start: it asks the user first, in a sheet of its own.
///
/// `AppleTranslator.prepare` waits in `download` while the app's root view runs the request through SwiftUI's
/// `translationTask`, which gives it a session that may show that sheet. `Core` imports no SwiftUI, so the view
/// does the SwiftUI part and this type does the waiting.
@MainActor
@Observable
final class TranslationDownloads {
    /// The pair to download, watched by the root view. Nil until the first download.
    private(set) var request: TranslationSession.Configuration?

    @ObservationIgnored private var waiting: CheckedContinuation<Void, any Error>?

    /// Returns once iOS has the packs for this pair. Throws if the user declines or the download fails.
    func download(from source: Locale.Language, to target: Locale.Language) async throws {
        // The pipeline handles one recording at a time, so a request still waiting has been abandoned.
        waiting?.resume(throwing: CancellationError())
        try await withCheckedThrowingContinuation { continuation in
            waiting = continuation
            if request?.source == source, request?.target == target {
                // The view starts a task only when the request changes, so asking for the same pair again, after
                // the user declined, needs a new version of it.
                request?.invalidate()
            } else {
                request = TranslationSession.Configuration(source: source, target: target)
            }
        }
    }

    /// Called by the root view with the session iOS gives it for `request`.
    func run(_ session: TranslationSession) async {
        do {
            try await session.prepareTranslation()
            waiting?.resume()
        } catch {
            waiting?.resume(throwing: error)
        }
        waiting = nil
    }
}
