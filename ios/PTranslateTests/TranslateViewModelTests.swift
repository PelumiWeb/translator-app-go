import Synchronization
import Testing
@testable import PTranslate

@MainActor
struct TranslateViewModelTests {
    private let oneSecond = PCMAudio(samples: Array(repeating: 0, count: 16_000))

    private func makeViewModel(
        recorder: FakeRecorder,
        transcriber: FakeTranscriber,
        microphoneAllowed: Bool = true
    ) -> TranslateViewModel {
        TranslateViewModel(
            recorder: recorder,
            pipeline: SpeechTranslationPipeline(transcriber: transcriber),
            requestMicrophone: { microphoneAllowed }
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
}

private final class FakeRecorder: AudioRecorder {
    private let audio: PCMAudio
    private let failure: RecorderError?
    private(set) var starts = 0

    init(audio: PCMAudio, failure: RecorderError? = nil) {
        self.audio = audio
        self.failure = failure
    }

    func start() throws -> any Recording {
        if let failure {
            throw failure
        }
        starts += 1
        return FakeRecording(audio: audio)
    }
}

private final class FakeRecording: Recording {
    private let audio: PCMAudio

    init(audio: PCMAudio) {
        self.audio = audio
    }

    func stop() -> PCMAudio { audio }

    func cancel() {}
}

/// Emits the scripted events, then ends or fails.
private final class FakeTranscriber: Transcriber {
    private let events: [TranscriptEvent]
    private let error: TranscriptionError?
    private let count = Mutex(0)

    init(events: [TranscriptEvent], error: TranscriptionError? = nil) {
        self.events = events
        self.error = error
    }

    var calls: Int { count.withLock { $0 } }

    func transcribe(_ audio: PCMAudio, language: Language) -> AsyncThrowingStream<TranscriptEvent, any Error> {
        count.withLock { $0 += 1 }
        return .producing { [events, error] continuation in
            for event in events {
                continuation.yield(event)
            }
            if let error {
                throw error
            }
        }
    }
}
