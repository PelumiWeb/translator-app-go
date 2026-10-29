import Foundation

/// Transcribes on the phone with whisper.cpp.
///
/// An actor because it keeps the loaded model between recordings: loading takes about a second.
actor WhisperTranscriber: Transcriber {
    private let modelFile: @Sendable () async -> URL?
    private let threads: Int
    private var context: WhisperContext?

    /// - Parameters:
    ///   - modelFile: the installed model, or nil while there is none. It is asked on every transcription, so a
    ///     model that finishes downloading is picked up without restarting anything.
    ///   - threads: how many CPU threads Whisper itself may use.
    init(modelFile: @escaping @Sendable () async -> URL?, threads: Int) {
        self.modelFile = modelFile
        self.threads = threads
    }

    nonisolated func transcribe(_ audio: PCMAudio, language: Language) -> AsyncThrowingStream<TranscriptEvent, any Error> {
        .producing { events in
            // Given silence, Whisper does not return nothing: it invents a plausible sentence. So silence is caught
            // here, before the model runs. The level is in the message, so a dead microphone can be told apart
            // from words the model did not catch.
            if audio.isSilent {
                let level = Int(audio.levelDB.rounded())
                throw TranscriptionError.silentAudio("The recording was silent (\(level) dB). Check the microphone")
            }

            events.yield(.stageChanged(.processing))
            // Whisper cannot be interrupted: if the reader stops listening, this still runs to its end.
            let result = try await self.run(audio.floatSamples, language: language)

            // The second guard, for audio that is loud enough but is not speech (noise, music): the model's own
            // no-speech estimate.
            let noSpeech = (result.noSpeechProbability ?? 0) > Self.noSpeechThreshold
            if noSpeech || result.text.isEmpty || isOnlySoundLabels(result.text) {
                throw TranscriptionError.noSpeech("No speech was recognised")
            }
            // The third: Whisper stuck in a loop. Its confidence is high for such output, so the text is checked.
            if looksLikeRepetitionLoop(result.text) {
                throw TranscriptionError.garbled("The speech could not be made out. Please try again")
            }
            events.yield(.final(Transcript(text: result.text, confidence: result.meanTokenProbability, source: .onDevice)))
        }
    }

    /// Loads the model now instead of on the first transcription, so that a timing taken afterwards measures
    /// transcribing and not loading.
    func load() async throws {
        _ = try await loadedContext()
    }

    /// Frees the model's memory. The next transcription loads it again.
    func close() {
        context = nil
    }

    private func run(_ samples: [Float], language: Language) async throws -> WhisperContext.Result {
        try await loadedContext().transcribe(samples, language: language, threads: threads)
    }

    private func loadedContext() async throws -> WhisperContext {
        if let context {
            return context
        }
        guard let file = await modelFile(), FileManager.default.fileExists(atPath: file.path(percentEncoded: false)) else {
            throw TranscriptionError.failed("The on-device model is not installed")
        }
        let loaded = try await WhisperContext.load(file)
        context = loaded
        return loaded
    }

    // The threshold OpenAI's reference implementation uses.
    private static let noSpeechThreshold: Float = 0.6
}
