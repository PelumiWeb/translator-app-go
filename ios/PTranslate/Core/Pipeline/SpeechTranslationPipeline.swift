/// Everything that happens to a recording after it is made. This is the entry point any front end uses: the app's
/// screen today, a keyboard later.
///
/// It only transcribes for now. Translation joins at checkpoint 10.4 and adds events here without changing how
/// callers read them.
struct SpeechTranslationPipeline: Sendable {
    let transcriber: any Transcriber

    func process(_ audio: PCMAudio, source: Language) -> AsyncThrowingStream<PipelineEvent, any Error> {
        .producing { events in
            for try await event in transcriber.transcribe(audio, language: source) {
                switch event {
                case .stageChanged(let stage): events.yield(.transcribing(stage))
                case .partial(let text): events.yield(.partialTranscript(text))
                case .final(let transcript): events.yield(.transcribed(transcript))
                }
            }
        }
    }
}

enum PipelineEvent: Equatable, Sendable {
    case transcribing(TranscriptionStage)
    case partialTranscript(String)
    case transcribed(Transcript)
}
