import Foundation
import Testing
@testable import PTranslate

@MainActor
struct TranslateViewModelTests {
    private let oneSecond = PCMAudio(samples: Array(repeating: 0, count: 16_000))
    private let models = FakeModels()
    private let route = RouteSetting(.automatic)
    private let speed = FakeSpeed()
    private let clock = FakeClock()

    private func makeViewModel(
        recorder: FakeRecorder,
        transcriber: any Transcriber,
        translator: FakeTranslator = FakeTranslator(),
        synthesizer: FakeSynthesizer = FakeSynthesizer(),
        microphoneAllowed: Bool = true
    ) -> TranslateViewModel {
        TranslateViewModel(
            recorder: recorder,
            pipeline: SpeechTranslationPipeline(transcriber: transcriber, translator: translator, synthesizer: synthesizer),
            translator: translator,
            models: models,
            route: route,
            speed: speed,
            requestMicrophone: { microphoneAllowed },
            now: { [clock] in clock.elapsed }
        )
    }

    /// A transcriber that hears `text`.
    private func saying(_ text: String) -> FakeTranscriber {
        FakeTranscriber(events: [.final(Transcript(text: text, confidence: nil, source: .cloud))])
    }

    /// Taps the button and waits for what the tap started to finish.
    private func tap(_ viewModel: TranslateViewModel) async {
        viewModel.recordTapped()
        await viewModel.work?.value
    }

    @Test func showsTheTranscriptOnceTheRecordingIsStopped() async {
        let transcriber = FakeTranscriber(events: [
            .stageChanged(.uploading),
            .partial("hel"),
            .final(Transcript(text: "hello", confidence: nil, source: .cloud)),
        ])
        let viewModel = makeViewModel(recorder: FakeRecorder(audio: oneSecond), transcriber: transcriber)

        await tap(viewModel)
        #expect(viewModel.phase == .recording)

        await tap(viewModel)
        #expect(viewModel.phase == .idle)
        #expect(viewModel.status == "Done")
        #expect(viewModel.text == "hello")
        #expect(viewModel.error == nil)
    }

    @Test func doesNotSendARecordingThatIsTooShort() async {
        let transcriber = FakeTranscriber(events: [])
        let tooShort = PCMAudio(samples: Array(repeating: 0, count: 1_600)) // 0.1 s
        let viewModel = makeViewModel(recorder: FakeRecorder(audio: tooShort), transcriber: transcriber)

        await tap(viewModel)
        await tap(viewModel)

        #expect(viewModel.phase == .idle)
        #expect(viewModel.error == "That was too short. Hold on a little longer")
        #expect(transcriber.calls == 0)
    }

    @Test func showsWhyATranscriptionFailed() async {
        let transcriber = FakeTranscriber(events: [.stageChanged(.uploading)], error: .failed("provider exploded"))
        let viewModel = makeViewModel(recorder: FakeRecorder(audio: oneSecond), transcriber: transcriber)

        await tap(viewModel)
        await tap(viewModel)

        #expect(viewModel.phase == .idle)
        #expect(viewModel.status == "Failed")
        #expect(viewModel.error == "provider exploded")
    }

    @Test func recordsNothingWithoutMicrophonePermission() async {
        let recorder = FakeRecorder(audio: oneSecond)
        let viewModel = makeViewModel(recorder: recorder, transcriber: FakeTranscriber(events: []), microphoneAllowed: false)

        await tap(viewModel)

        #expect(viewModel.phase == .idle)
        #expect(viewModel.error == "Microphone permission is needed to record")
        #expect(recorder.starts == 0)
    }

    @Test func showsWhyTheMicrophoneCouldNotBeOpened() async {
        let recorder = FakeRecorder(audio: oneSecond, failure: RecorderError(message: "no microphone"))
        let viewModel = makeViewModel(recorder: recorder, transcriber: FakeTranscriber(events: []))

        await tap(viewModel)

        #expect(viewModel.phase == .idle)
        #expect(viewModel.error == "no microphone")
    }

    // MARK: Where the result came from

    @Test func showsWhereTheTranscriptCameFromHowLongItTookAndTheConfidence() async {
        let transcriber = ControlledTranscriber()
        let fourSeconds = PCMAudio(samples: Array(repeating: 0, count: 64_000))
        let viewModel = makeViewModel(recorder: FakeRecorder(audio: fourSeconds), transcriber: transcriber)

        await tap(viewModel)
        viewModel.recordTapped() // stop
        clock.elapsed += .milliseconds(1_200)
        transcriber.send(.final(Transcript(text: "hello", confidence: 0.874, source: .onDevice)))
        await viewModel.work?.value

        #expect(viewModel.details == "On device, 1.2 s for 4.0 s of audio (0.30x real time), confidence 0.87")
    }

    @Test func detailsLeaveOutTheConfidenceWhenThereIsNone() async {
        let transcriber = ControlledTranscriber()
        let viewModel = makeViewModel(recorder: FakeRecorder(audio: oneSecond), transcriber: transcriber)

        await tap(viewModel)
        viewModel.recordTapped()
        clock.elapsed += .seconds(3)
        transcriber.send(.final(Transcript(text: "hello", confidence: nil, source: .cloud)))
        await viewModel.work?.value

        #expect(viewModel.details == "Cloud, 3.0 s for 1.0 s of audio (3.00x real time)")
    }

    @Test func theRouteCanBeChosenButNotDuringARecording() async {
        let viewModel = makeViewModel(recorder: FakeRecorder(audio: oneSecond), transcriber: FakeTranscriber(events: []))
        #expect(viewModel.route == .automatic)

        viewModel.routeSelected(.cloud)
        #expect(route.current == .cloud)
        #expect(viewModel.route == .cloud)

        await tap(viewModel) // now recording
        viewModel.routeSelected(.onDevice)
        #expect(route.current == .cloud)
        #expect(viewModel.route == .cloud)
    }

    @Test func saysWhyAResultWasSentToTheServer() async {
        let transcriber = ControlledTranscriber()
        let viewModel = makeViewModel(recorder: FakeRecorder(audio: oneSecond), transcriber: transcriber)

        await tap(viewModel)
        viewModel.recordTapped() // stop
        transcriber.send(.final(Transcript(text: "hello", confidence: nil, source: .cloud, note: .lowConfidence)))
        await viewModel.work?.value

        #expect(viewModel.routing == "Sent to the server: this device was not confident in its own result")

        // The explanation belongs to that result and goes with it.
        await tap(viewModel)
        #expect(viewModel.routing == nil)
    }

    @Test func aResultThatTookTheFirstChoiceNeedsNoExplanation() async {
        let viewModel = makeViewModel(recorder: FakeRecorder(audio: oneSecond), transcriber: saying("good morning"))

        await tap(viewModel)
        await tap(viewModel)

        #expect(viewModel.routing == nil)
    }

    // MARK: The device's speed

    @Test func showsTheDevicesSpeedAndWhetherItIsTooSlow() {
        let viewModel = makeViewModel(recorder: FakeRecorder(audio: oneSecond), transcriber: FakeTranscriber(events: []))
        #expect(viewModel.deviceSpeed == nil)
        #expect(!viewModel.deviceTooSlow)

        speed.realTimeFactor = 0.4
        #expect(viewModel.deviceSpeed == 0.4)
        #expect(!viewModel.deviceTooSlow)

        speed.realTimeFactor = 1.7 // over the default limit of 1.0
        #expect(viewModel.deviceTooSlow)

        speed.measuring = true
        #expect(viewModel.measuringSpeed)
    }

    @Test func measureAgainAsksForANewMeasurementOnceAtATime() async {
        let viewModel = makeViewModel(recorder: FakeRecorder(audio: oneSecond), transcriber: FakeTranscriber(events: []))

        viewModel.measureSpeedTapped()
        await viewModel.measurement?.value
        #expect(speed.measurements == 1)
        #expect(viewModel.deviceSpeed == 0.5)

        speed.measuring = true
        viewModel.measureSpeedTapped()
        await viewModel.measurement?.value
        #expect(speed.measurements == 1)
    }

    // MARK: Translation

    @Test func showsTheTranscriptAndItsTranslationIntoTheChosenLanguage() async {
        let viewModel = makeViewModel(recorder: FakeRecorder(audio: oneSecond), transcriber: saying("good morning"))
        viewModel.targetSelected(Language("fr"))

        await tap(viewModel)
        await tap(viewModel)

        #expect(viewModel.text == "good morning")
        #expect(viewModel.translation == "[en>fr] good morning")
        #expect(viewModel.status == "Done")
        #expect(viewModel.phase == .idle)
    }

    @Test(.timeLimit(.minutes(1)))
    func speaksTheTranslationAndShowsThatItIsSpeaking() async {
        let synthesizer = FakeSynthesizer(holds: true) // keeps talking until told otherwise
        let viewModel = makeViewModel(
            recorder: FakeRecorder(audio: oneSecond),
            transcriber: saying("good morning"),
            synthesizer: synthesizer
        )

        await tap(viewModel)
        viewModel.recordTapped() // stop
        await synthesizer.untilSpeaking()
        // The view model reads the events on the main actor; let it catch up with the ones already sent.
        for _ in 0..<100 where viewModel.status != "Speaking" {
            await Task.yield()
        }

        #expect(synthesizer.spoken == ["es: [en>es] good morning"])
        #expect(viewModel.status == "Speaking")
        #expect(viewModel.translation == "[en>es] good morning")
        #expect(viewModel.phase == .working)

        synthesizer.finish()
        await viewModel.work?.value

        #expect(viewModel.status == "Done")
        #expect(viewModel.phase == .idle)
    }

    @Test func aTranslationThatCannotBeSpokenStaysOnScreenWithTheReason() async {
        let viewModel = makeViewModel(
            recorder: FakeRecorder(audio: oneSecond),
            transcriber: saying("good morning"),
            synthesizer: FakeSynthesizer(failure: SpeechError(message: "This iPhone has no voice for Spanish"))
        )

        await tap(viewModel)
        await tap(viewModel)

        #expect(viewModel.translation == "[en>es] good morning")
        #expect(viewModel.status == "Done")
        #expect(viewModel.error == "This iPhone has no voice for Spanish")
        #expect(viewModel.phase == .idle)
    }

    @Test func theSameLanguageOnBothSidesShowsTheTranscriptOnly() async {
        let viewModel = makeViewModel(recorder: FakeRecorder(audio: oneSecond), transcriber: saying("good morning"))
        viewModel.targetSelected(Language("en"))

        await tap(viewModel)
        await tap(viewModel)

        #expect(viewModel.text == "good morning")
        #expect(viewModel.translation == "")
        #expect(viewModel.status == "Done")
    }

    @Test func aFailedTranslationKeepsTheTranscriptOnScreen() async {
        let translator = FakeTranslator(translateError: TranslationError(message: "Translation failed"))
        let viewModel = makeViewModel(
            recorder: FakeRecorder(audio: oneSecond),
            transcriber: saying("good morning"),
            translator: translator
        )

        await tap(viewModel)
        await tap(viewModel)

        #expect(viewModel.text == "good morning")
        #expect(viewModel.translation == "")
        #expect(viewModel.error == "Translation failed")
        #expect(viewModel.phase == .idle)
    }

    @Test func aNewRecordingClearsTheLastResultButKeepsTheLanguages() async {
        let viewModel = makeViewModel(recorder: FakeRecorder(audio: oneSecond), transcriber: saying("good morning"))
        viewModel.sourceSelected(Language("es"))
        viewModel.targetSelected(Language("fr"))
        await tap(viewModel)
        await tap(viewModel)
        #expect(viewModel.translation == "[es>fr] good morning")

        await tap(viewModel)

        #expect(viewModel.text == "")
        #expect(viewModel.translation == "")
        #expect(viewModel.details == nil)
        #expect(viewModel.source == Language("es"))
        #expect(viewModel.target == Language("fr"))
    }

    @Test func languagesCannotBeChangedDuringARecording() async {
        let viewModel = makeViewModel(recorder: FakeRecorder(audio: oneSecond), transcriber: FakeTranscriber(events: []))

        await tap(viewModel)
        viewModel.sourceSelected(Language("es"))
        viewModel.targetSelected(Language("fr"))

        #expect(viewModel.source == .english)
        #expect(viewModel.target == .spanish)
    }

    @Test func offersEveryLanguageToTranslateIntoButOnlyThoseWhisperHearsToSpeakIn() async {
        // Whisper cannot transcribe Esperanto.
        let translator = FakeTranslator(languages: [Language("en"), Language("eo"), Language("es")])
        let viewModel = makeViewModel(
            recorder: FakeRecorder(audio: oneSecond),
            transcriber: FakeTranscriber(events: []),
            translator: translator
        )
        #expect(viewModel.sourceLanguages.isEmpty)

        await viewModel.loadLanguages()

        #expect(viewModel.sourceLanguages == [Language("en"), Language("es")])
        #expect(viewModel.targetLanguages == [Language("en"), Language("eo"), Language("es")])
    }

    // MARK: The speech model

    @Test func showsTheStateOfTheSpeechModelAsItChanges() {
        let viewModel = makeViewModel(recorder: FakeRecorder(audio: oneSecond), transcriber: FakeTranscriber(events: []))
        #expect(viewModel.model == .missing)

        models.state = .downloading(bytes: 10, total: 100)
        #expect(viewModel.model == .downloading(bytes: 10, total: 100))

        models.state = .verifying
        #expect(viewModel.model == .verifying)
    }

    @Test func theDownloadButtonInstallsTheModel() async {
        let viewModel = makeViewModel(recorder: FakeRecorder(audio: oneSecond), transcriber: FakeTranscriber(events: []))

        viewModel.downloadModelTapped()
        await viewModel.download?.value

        #expect(models.installCalls == 1)
        #expect(viewModel.model == .ready(FakeModels.file))
    }

    @Test func aFailedDownloadShowsItsReasonAndCanBeTriedAgain() async {
        models.failure = "The download was interrupted"
        let viewModel = makeViewModel(recorder: FakeRecorder(audio: oneSecond), transcriber: FakeTranscriber(events: []))

        viewModel.downloadModelTapped()
        await viewModel.download?.value
        #expect(viewModel.model == .failed("The download was interrupted"))

        models.failure = nil
        viewModel.downloadModelTapped()
        await viewModel.download?.value

        #expect(models.installCalls == 2)
        #expect(viewModel.model == .ready(FakeModels.file))
    }

    @Test func theDownloadButtonDoesNothingWhileADownloadIsRunning() async {
        let viewModel = makeViewModel(recorder: FakeRecorder(audio: oneSecond), transcriber: FakeTranscriber(events: []))
        models.state = .downloading(bytes: 10, total: 100)

        viewModel.downloadModelTapped()

        #expect(viewModel.download == nil)
        #expect(models.installCalls == 0)
    }
}
