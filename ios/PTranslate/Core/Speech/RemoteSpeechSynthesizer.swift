import Foundation

/// Speaks in the user's own voice: asks the backend to say the text in the voice it keeps for them (ADR 0016), then
/// plays the audio that comes back. Every step survives a connection that drops, as transcription does. Cancelling
/// the caller stops the requests; a clip that is already playing is heard to its end.
///
/// The server's copy is used, not the one on the phone: it knows the words read in it, in the user's language
/// (`VoicePassage`), which the voice engine needs, and nothing has to be uploaded.
struct RemoteSpeechSynthesizer: SpeechSynthesizer {
    private let backend: BackendClient
    private let hasVoice: @Sendable () async -> Bool
    private let play: @Sendable (PCMAudio) async throws -> Void
    private let stopPlaying: @Sendable () async -> Void
    private let maxAttempts: Int
    private let retryDelay: @Sendable (_ attempt: Int) -> Duration

    /// The voice store and the player belong to the main actor, where the screens use them too, so they come in as
    /// functions rather than as the objects themselves.
    ///
    /// - Parameters:
    ///   - hasVoice: whether the user has recorded their voice; with none, nothing is asked of the server.
    ///   - play: plays audio and returns when it has finished or was stopped.
    ///   - stopPlaying: stops what `play` is playing.
    ///   - maxAttempts: how many times in a row a step may fail before giving up.
    ///   - retryDelay: how long to wait before try number `attempt + 1`.
    init(
        backend: BackendClient,
        hasVoice: @escaping @Sendable () async -> Bool,
        play: @escaping @Sendable (PCMAudio) async throws -> Void,
        stopPlaying: @escaping @Sendable () async -> Void,
        maxAttempts: Int = 5,
        retryDelay: @escaping @Sendable (_ attempt: Int) -> Duration = { .seconds($0) }
    ) {
        self.backend = backend
        self.hasVoice = hasVoice
        self.play = play
        self.stopPlaying = stopPlaying
        self.maxAttempts = maxAttempts
        self.retryDelay = retryDelay
    }

    func speak(_ text: String, language: Language, onStart: @escaping @Sendable (SpeechVoice) -> Void) async throws(SpeechError) {
        guard await hasVoice() else {
            throw SpeechError(message: String(localized: "No voice has been recorded"))
        }

        let wav: Data
        do {
            // The same key on every attempt, as for an upload to transcribe.
            let idempotencyKey = UUID().uuidString
            let job = try await retrying(maxAttempts: maxAttempts, retryDelay: retryDelay) {
                try await backend.createSpeech(text: text, language: language, idempotencyKey: idempotencyKey)
            }
            try await backend.followJob(job.id, maxAttempts: maxAttempts, retryDelay: retryDelay) { _ in }
            wav = try await retrying(maxAttempts: maxAttempts, retryDelay: retryDelay) {
                try await backend.speechAudio(job.audioURL)
            }
        } catch let error as BackendError {
            throw SpeechError(message: error.message)
        } catch is CancellationError {
            return // the caller stopped listening, which for speaking is the same as being stopped
        } catch {
            throw SpeechError(message: error.localizedDescription)
        }

        let audio: PCMAudio
        do {
            audio = try PCMAudio(wav: wav)
        } catch {
            throw SpeechError(message: String(localized: "The server sent audio that cannot be played"))
        }
        onStart(.own)
        do {
            try await play(audio)
        } catch {
            throw SpeechError(message: String(localized: "Could not play the translation: \(error.localizedDescription)"))
        }
    }

    func stop() async {
        await stopPlaying()
    }
}
