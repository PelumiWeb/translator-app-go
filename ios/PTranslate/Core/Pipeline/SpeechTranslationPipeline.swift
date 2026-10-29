/// Everything that happens to a recording after it is made: transcribe it, then translate the transcript. This is
/// the entry point any front end uses: the app's screen today, a keyboard later.
///
/// Speaking the translation joins at checkpoint 10.7 and adds events here without changing how callers read them.
struct SpeechTranslationPipeline: Sendable {
    let transcriber: any Transcriber
    let translator: any Translator

    /// Fails with `TranscriptionError` or `TranslationError`. When translation fails the transcript has already been
    /// delivered, so it is not lost.
    func process(_ audio: PCMAudio, source: Language, target: Language) -> AsyncThrowingStream<PipelineEvent, any Error> {
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
            events.yield(.translated(try await translator.translate(text, source: source, target: target)))
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
}
