import AVFoundation
import Synchronization

/// Speaks with the phone's built-in voices through `AVSpeechSynthesizer`: offline, and nothing to install.
///
/// iOS ships a voice for each language it speaks, so unlike Android there is no first use that waits for a voice
/// to download, and nothing to retry. The synthesizer reports the end of each utterance to its delegate; this type
/// turns that into one `async` call per utterance.
final class SystemSpeechSynthesizer: NSObject, SpeechSynthesizer, AVSpeechSynthesizerDelegate {
    // AVSpeechSynthesizer is not Sendable, so every call to it goes through this lock. The callers waiting for their
    // utterance to start and to end have a lock of their own: the synthesizer may report an end from inside a call
    // made under the first one.
    private let synthesizer: Mutex<AVSpeechSynthesizer>
    private let waiting = Mutex<[ObjectIdentifier: Waiting]>([:])

    private struct Waiting {
        let ended: CheckedContinuation<Void, Never>
        var started: (@Sendable (SpeechVoice) -> Void)?
    }

    override init() {
        synthesizer = Mutex(AVSpeechSynthesizer())
        super.init()
        synthesizer.withLock { $0.delegate = self }
    }

    func speak(_ text: String, language: Language, onStart: @escaping @Sendable (SpeechVoice) -> Void) async throws(SpeechError) {
        guard let voice = Self.voice(for: language) else {
            throw SpeechError(message: "This iPhone has no voice for \(Self.name(of: language))")
        }
        // One utterance at a time: a new one replaces whatever is still being said.
        stop()

        let session = AVAudioSession.sharedInstance()
        do {
            // The recorder leaves the session set up for recording, which plays nothing.
            try session.setCategory(.playback, mode: .spokenAudio)
            try session.setActive(true)
        } catch {
            throw SpeechError(message: "Could not play the translation: \(error.localizedDescription)")
        }
        defer { try? session.setActive(false, options: .notifyOthersOnDeactivation) }

        await withTaskCancellationHandler {
            await withCheckedContinuation { continuation in
                synthesizer.withLock { synthesizer in
                    let utterance = AVSpeechUtterance(string: text)
                    utterance.voice = voice
                    // Waiting before speaking starts, so that an utterance that ends at once still finds its caller.
                    waiting.withLock { $0[ObjectIdentifier(utterance)] = Waiting(ended: continuation, started: onStart) }
                    synthesizer.speak(utterance)
                }
            }
        } onCancel: {
            stop()
        }
    }

    func stop() {
        // The synthesizer then reports the utterance as cancelled, which ends `speak`.
        _ = synthesizer.withLock { $0.stopSpeaking(at: .immediate) }
    }

    /// The voice iOS uses for the language's default region (American English, Brazilian Portuguese), or else for
    /// the first region it has a voice for. Nil if it has none.
    static func voice(for language: Language) -> AVSpeechSynthesisVoice? {
        let installed = AVSpeechSynthesisVoice.speechVoices().map(\.language)
        guard
            let chosen = language.match(in: installed.map { Locale.Language(identifier: $0) }),
            let code = installed.first(where: { Locale.Language(identifier: $0) == chosen })
        else { return nil }
        // Asked by language rather than taken from the list: this way it is the voice the user picked for that
        // language in Settings, never one of the novelty voices the list also holds.
        return AVSpeechSynthesisVoice(language: code)
    }

    private static func name(of language: Language) -> String {
        Locale.current.localizedString(forLanguageCode: language.tag) ?? language.tag
    }

    // MARK: AVSpeechSynthesizerDelegate

    func speechSynthesizer(_ synthesizer: AVSpeechSynthesizer, didStart utterance: AVSpeechUtterance) {
        let started = waiting.withLock { waiting in
            defer { waiting[ObjectIdentifier(utterance)]?.started = nil }
            return waiting[ObjectIdentifier(utterance)]?.started
        }
        started?(.phone)
    }

    func speechSynthesizer(_ synthesizer: AVSpeechSynthesizer, didFinish utterance: AVSpeechUtterance) {
        ended(utterance)
    }

    // Stopped on request: for the caller that is a normal end.
    func speechSynthesizer(_ synthesizer: AVSpeechSynthesizer, didCancel utterance: AVSpeechUtterance) {
        ended(utterance)
    }

    private func ended(_ utterance: AVSpeechUtterance) {
        waiting.withLock { $0.removeValue(forKey: ObjectIdentifier(utterance)) }?.ended.resume()
    }
}
