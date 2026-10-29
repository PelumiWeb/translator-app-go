import Foundation
import Testing
@testable import PTranslate

@MainActor
struct TranslateViewModelTests {
    private let oneSecond = PCMAudio(samples: Array(repeating: 0, count: 16_000))
    private let models = FakeModels()
    private let route = RouteSetting(.cloud)
    private let clock = FakeClock()

    private func makeViewModel(
        recorder: FakeRecorder,
        transcriber: any Transcriber,
        microphoneAllowed: Bool = true
    ) -> TranslateViewModel {
        TranslateViewModel(
            recorder: recorder,
            pipeline: SpeechTranslationPipeline(transcriber: transcriber),
            models: models,
            route: route,
            requestMicrophone: { microphoneAllowed },
            now: { [clock] in clock.elapsed }
        )
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
        #expect(viewModel.route == .cloud)

        viewModel.routeSelected(.onDevice)
        #expect(route.current == .onDevice)
        #expect(viewModel.route == .onDevice)

        await tap(viewModel) // now recording
        viewModel.routeSelected(.cloud)
        #expect(route.current == .onDevice)
        #expect(viewModel.route == .onDevice)
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
