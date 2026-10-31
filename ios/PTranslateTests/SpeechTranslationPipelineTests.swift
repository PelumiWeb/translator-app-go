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

    private func pipeline(
        transcriber: FakeTranscriber? = nil,
        translator: FakeTranslator = FakeTranslator(),
        synthesizer: FakeSynthesizer = FakeSynthesizer()
    ) -> SpeechTranslationPipeline {
        SpeechTranslationPipeline(transcriber: transcriber ?? speaking, translator: translator, synthesizer: synthesizer)
    }

    /// Every event, and the error the run ended with, if any.
    private func run(
        _ pipeline: SpeechTranslationPipeline,
        target: Language,
        speak: Bool = true
    ) async -> (events: [PipelineEvent], error: (any Error)?) {
        var events: [PipelineEvent] = []
        do {
            for try await event in pipeline.process(audio, source: english, target: target, speak: speak) {
                events.append(event)
            }
            return (events, nil)
        } catch {
            return (events, error)
        }
    }

    @Test func transcribesThenTranslatesThenSpeaks() async {
        let translator = FakeTranslator()
        let synthesizer = FakeSynthesizer()

        let (events, error) = await run(pipeline(translator: translator, synthesizer: synthesizer), target: spanish)

        #expect(error == nil)
        #expect(events == [
            .transcribing(.processing),
            .partialTranscript("hello"),
            .transcribed(transcript),
            .translating,
            .translated("[en>es] hello world"),
            .preparingSpeech,
            .speaking,
            .spoken,
        ])
        #expect(translator.calls == ["translate en>es"])
        // Spoken in the listener's language, not the speaker's.
        #expect(synthesizer.spoken == ["es: [en>es] hello world"])
    }

    @Test func downloadsTheLanguagePackFirstWhenItIsMissing() async {
        let translator = FakeTranslator(ready: false)

        let (events, _) = await run(pipeline(translator: translator), target: spanish)

        #expect(Array(events.dropFirst(2)) == [
            .transcribed(transcript),
            .downloadingLanguages,
            .translating,
            .translated("[en>es] hello world"),
            .preparingSpeech,
            .speaking,
            .spoken,
        ])
        #expect(translator.calls == ["prepare en>es", "translate en>es"])
    }

    @Test func theSameSourceAndTargetLanguageSkipsTheTranslator() async {
        let translator = FakeTranslator(ready: false)
        let synthesizer = FakeSynthesizer()

        let (events, _) = await run(pipeline(translator: translator, synthesizer: synthesizer), target: english)

        #expect(events.last == .translated("hello world"))
        #expect(translator.calls.isEmpty)
        // Repeating someone's words back in their own language is not interpreting.
        #expect(synthesizer.spoken.isEmpty)
    }

    @Test func aFailedTranslationStillDeliversTheTranscriptFirst() async {
        let translator = FakeTranslator(translateError: TranslationError(message: "Translation failed"))

        let (events, error) = await run(pipeline(translator: translator), target: spanish)

        #expect(events[2] == .transcribed(transcript))
        #expect(events.last == .translating)
        #expect(error as? TranslationError == TranslationError(message: "Translation failed"))
    }

    @Test func aFailedLanguagePackDownloadIsReported() async {
        let translator = FakeTranslator(
            ready: false,
            prepareError: TranslationError(message: "The language pack was not downloaded")
        )

        let (events, error) = await run(pipeline(translator: translator), target: spanish)

        #expect(events.last == .downloadingLanguages)
        #expect(error as? TranslationError == TranslationError(message: "The language pack was not downloaded"))
        #expect(translator.calls == ["prepare en>es"])
    }

    @Test func withSpeakingTurnedOffItEndsAtTheTranslatedText() async {
        let synthesizer = FakeSynthesizer()

        let (events, _) = await run(pipeline(synthesizer: synthesizer), target: spanish, speak: false)

        #expect(events.last == .translated("[en>es] hello world"))
        #expect(synthesizer.spoken.isEmpty)
    }

    @Test func aTranslationThatCannotBeSpokenIsStillDeliveredAsText() async {
        let synthesizer = FakeSynthesizer(failure: SpeechError(message: "This iPhone has no voice for Spanish"))

        let (events, error) = await run(pipeline(synthesizer: synthesizer), target: spanish)

        // Not being able to speak is not a failed run.
        #expect(error == nil)
        #expect(Array(events.suffix(3)) == [
            .translated("[en>es] hello world"),
            .preparingSpeech,
            .speechFailed("This iPhone has no voice for Spanish"),
        ])
    }

    @Test func aFailedTranscriptionNeverReachesTheTranslator() async {
        let failing = FakeTranscriber(events: [], error: .noSpeech("No speech was recognised"))
        let translator = FakeTranslator()
        let synthesizer = FakeSynthesizer()

        let (_, error) = await run(
            pipeline(transcriber: failing, translator: translator, synthesizer: synthesizer),
            target: spanish
        )

        #expect(error as? TranscriptionError == .noSpeech("No speech was recognised"))
        #expect(translator.calls.isEmpty)
        #expect(synthesizer.spoken.isEmpty)
    }
}
