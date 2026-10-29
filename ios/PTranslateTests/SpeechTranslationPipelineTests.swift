import Testing
@testable import PTranslate

struct SpeechTranslationPipelineTests {
    private let english = Language("en")
    private let spanish = Language("es")
    private let audio = PCMAudio(samples: Array(repeating: 0, count: 16_000))
    private let transcript = Transcript(text: "hello world", confidence: 0.9, source: .onDevice)

    private var speaking: FakeTranscriber {
        FakeTranscriber(events: [.stageChanged(.processing), .partial("hello"), .final(transcript)])
    }

    /// Every event, and the error the run ended with, if any.
    private func run(
        _ pipeline: SpeechTranslationPipeline,
        target: Language
    ) async -> (events: [PipelineEvent], error: (any Error)?) {
        var events: [PipelineEvent] = []
        do {
            for try await event in pipeline.process(audio, source: english, target: target) {
                events.append(event)
            }
            return (events, nil)
        } catch {
            return (events, error)
        }
    }

    @Test func transcribesThenTranslates() async {
        let translator = FakeTranslator()

        let (events, error) = await run(SpeechTranslationPipeline(transcriber: speaking, translator: translator), target: spanish)

        #expect(error == nil)
        #expect(events == [
            .transcribing(.processing),
            .partialTranscript("hello"),
            .transcribed(transcript),
            .translating,
            .translated("[en>es] hello world"),
        ])
        #expect(translator.calls == ["translate en>es"])
    }

    @Test func downloadsTheLanguagePackFirstWhenItIsMissing() async {
        let translator = FakeTranslator(ready: false)

        let (events, _) = await run(SpeechTranslationPipeline(transcriber: speaking, translator: translator), target: spanish)

        #expect(Array(events.dropFirst(2)) == [
            .transcribed(transcript),
            .downloadingLanguages,
            .translating,
            .translated("[en>es] hello world"),
        ])
        #expect(translator.calls == ["prepare en>es", "translate en>es"])
    }

    @Test func theSameSourceAndTargetLanguageSkipsTheTranslator() async {
        let translator = FakeTranslator(ready: false)

        let (events, _) = await run(SpeechTranslationPipeline(transcriber: speaking, translator: translator), target: english)

        #expect(events.last == .translated("hello world"))
        #expect(translator.calls.isEmpty)
    }

    @Test func aFailedTranslationStillDeliversTheTranscriptFirst() async {
        let translator = FakeTranslator(translateError: TranslationError(message: "Translation failed"))

        let (events, error) = await run(SpeechTranslationPipeline(transcriber: speaking, translator: translator), target: spanish)

        #expect(events[2] == .transcribed(transcript))
        #expect(events.last == .translating)
        #expect(error as? TranslationError == TranslationError(message: "Translation failed"))
    }

    @Test func aFailedLanguagePackDownloadIsReported() async {
        let translator = FakeTranslator(
            ready: false,
            prepareError: TranslationError(message: "The language pack was not downloaded")
        )

        let (events, error) = await run(SpeechTranslationPipeline(transcriber: speaking, translator: translator), target: spanish)

        #expect(events.last == .downloadingLanguages)
        #expect(error as? TranslationError == TranslationError(message: "The language pack was not downloaded"))
        #expect(translator.calls == ["prepare en>es"])
    }

    @Test func aFailedTranscriptionNeverReachesTheTranslator() async {
        let failing = FakeTranscriber(events: [], error: .noSpeech("No speech was recognised"))
        let translator = FakeTranslator()

        let (_, error) = await run(SpeechTranslationPipeline(transcriber: failing, translator: translator), target: spanish)

        #expect(error as? TranscriptionError == .noSpeech("No speech was recognised"))
        #expect(translator.calls.isEmpty)
    }
}
