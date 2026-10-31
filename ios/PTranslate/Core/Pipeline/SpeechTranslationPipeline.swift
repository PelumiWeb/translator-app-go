/// Everything that happens to a recording after it is made: transcribe it, translate the transcript, and say the
/// translation aloud. This is the entry point any front end uses: the app's screen today, a keyboard later.
struct SpeechTranslationPipeline: Sendable {
    let transcriber: any Transcriber
    let translator: any Translator
    let synthesizer: any SpeechSynthesizer

    /// A stream that ends once the translation has been spoken, or after `.translated` when `speak` is false, as a
    /// keyboard would want.
    ///
    /// It fails with `TranscriptionError` or `TranslationError`; when translation fails the transcript has already
    /// been delivered, so it is not lost. A failure to speak is not a failure of the run: the text is already
    /// delivered, so it is reported as `.speechFailed`.
    func process(
        _ audio: PCMAudio,
        source: Language,
        target: Language,
        speak: Bool = true
    ) -> AsyncThrowingStream<PipelineEvent, any Error> {
        .producing { events in
            var transcript: Transcript?
            for try await event in transcriber.transcribe(audio, language: source) {
                switch event {
                case .stageChanged(let stage):
                    events.yield(.transcribing(stage))
                case .partial(let text):
                    events.yield(.partialTranscript(text))
                case .final(let final):
                    transcript = final
                    events.yield(.transcribed(final))
                }
            }
            // A transcriber always ends with `.final` or fails, so this is set.
            guard let text = transcript?.text else {
                throw TranscriptionError.failed("The transcriber finished without a transcript")
            }

            if source == target {
                events.yield(.translated(text))
                return
            }

            if await !translator.isReady(source: source, target: target) {
                events.yield(.downloadingLanguages)
                try await translator.prepare(source: source, target: target)
            }
            events.yield(.translating)
            let translation = try await translator.translate(text, source: source, target: target)
            events.yield(.translated(translation))

            guard speak else { return }
            events.yield(.preparingSpeech)
            do throws(SpeechError) {
                try await synthesizer.speak(translation, language: target, onStart: { events.yield(.speaking) })
                events.yield(.spoken)
            } catch {
                events.yield(.speechFailed(error.message))
            }
        }
    }
}

enum PipelineEvent: Equatable, Sendable {
    case transcribing(TranscriptionStage)
    case partialTranscript(String)
    case transcribed(Transcript)

    /// A language pack is being fetched; this can take a while the first time.
    case downloadingLanguages

    case translating
    case translated(String)

    /// The speech is being made, before anything can be heard. Seconds, for a voice made on the server.
    case preparingSpeech

    /// The translation is being said aloud.
    case speaking

    /// It has been said, or was stopped part way.
    case spoken

    /// The text is fine but could not be spoken, for example no voice for the language.
    case speechFailed(String)
}
