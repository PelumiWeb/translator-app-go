import Testing
@testable import PTranslate

/// The same cases as the Android tests for this screen.
@MainActor
struct VoiceSetupViewModelTests {
    private let recorder = FakeRecorder(audio: speech(seconds: 20))
    private let player = FakePlayer()
    private let store = FakeVoiceStore()
    private let preference = FakePreference()

    private func makeViewModel(
        maxRecording: Duration = .seconds(30),
        microphoneAllowed: Bool = true
    ) -> VoiceSetupViewModel {
        VoiceSetupViewModel(
            recorder: recorder,
            player: player,
            samples: store,
            preference: preference,
            requestMicrophone: { microphoneAllowed },
            maxRecording: maxRecording
        )
    }

    /// Starts a recording and waits until it is running.
    private func startRecording(_ viewModel: VoiceSetupViewModel) async {
        viewModel.recordTapped()
        await viewModel.work?.value
    }

    /// Records and stops, leaving the recording under review.
    private func recordAndStop(_ viewModel: VoiceSetupViewModel) async {
        await startRecording(viewModel)
        viewModel.recordTapped()
    }

    @Test func recordReviewAndKeepTheRecording() async {
        let viewModel = makeViewModel()
        #expect(viewModel.stage == .intro)

        await startRecording(viewModel)
        #expect(viewModel.stage == .recording)
        #expect(viewModel.recordingStarted != nil)

        viewModel.recordTapped() // stop
        #expect(viewModel.stage == .review)
        #expect(viewModel.recorded == .seconds(20))
        #expect(viewModel.problem == nil)
        #expect(store.saved == nil, "nothing is saved until the user says so")

        viewModel.useTapped()
        await viewModel.work?.value

        #expect(store.saved == recorder.audio)
        #expect(viewModel.finished)
        #expect(!preference.skipped)
    }

    @Test func aRecordingThatIsTooShortCannotBeKept() async {
        recorder.audio = speech(seconds: 4)
        let viewModel = makeViewModel()

        await recordAndStop(viewModel)
        #expect(viewModel.problem?.hasPrefix("That was only 4 seconds") == true)

        viewModel.useTapped()
        await viewModel.work?.value

        #expect(store.saved == nil)
        #expect(!viewModel.finished)
    }

    @Test func recordAgainReplacesTheRecordingUnderReview() async {
        recorder.audio = speech(seconds: 4)
        let viewModel = makeViewModel()
        await recordAndStop(viewModel)

        recorder.audio = speech(seconds: 18)
        await startRecording(viewModel) // "Record again"
        #expect(viewModel.stage == .recording)
        #expect(viewModel.problem == nil, "the old problem is gone")
        viewModel.recordTapped()

        #expect(viewModel.recorded == .seconds(18))
        #expect(viewModel.problem == nil)
    }

    @Test func recordingStopsByItselfAtTheTimeLimit() async {
        let viewModel = makeViewModel(maxRecording: .zero)

        await startRecording(viewModel)
        await viewModel.autoStop?.value

        #expect(viewModel.stage == .review)
    }

    @Test func listenPlaysTheRecordingUnderReviewAndStopsItOnASecondTap() async {
        let viewModel = makeViewModel()
        await recordAndStop(viewModel)
        player.holds = true

        viewModel.listenTapped()
        #expect(viewModel.playing)
        // The playback task has to reach the player before it can be stopped.
        for _ in 0..<100 where player.played.isEmpty {
            await Task.yield()
        }
        #expect(player.played == [recorder.audio])

        viewModel.listenTapped()
        await viewModel.playback?.value
        #expect(player.stops >= 1)
        #expect(!viewModel.playing)
    }

    @Test func notNowLeavesWithoutSavingAndDoesNotAskAgain() async {
        let viewModel = makeViewModel()
        await startRecording(viewModel) // mid-recording when they change their mind

        viewModel.skipTapped()

        #expect(viewModel.finished)
        #expect(preference.skipped)
        #expect(store.saved == nil)
        #expect(recorder.cancelled == 1, "the microphone is let go")
    }

    @Test func withAVoiceAlreadySavedLeavingKeepsItAndChangesNothing() async throws {
        try await store.save(speech(seconds: 20))
        let viewModel = makeViewModel()
        #expect(viewModel.saved == .seconds(20))

        viewModel.skipTapped() // shown as "Keep it"

        #expect(viewModel.finished)
        #expect(!preference.skipped)
        #expect(store.state == .recorded(.seconds(20)))
    }

    @Test func deletingRemovesTheSavedVoiceAndDoesNotAskAgain() async throws {
        try await store.save(speech(seconds: 20))
        let viewModel = makeViewModel()

        viewModel.deleteTapped()
        await viewModel.work?.value

        #expect(store.state == .notRecorded)
        #expect(viewModel.saved == nil)
        #expect(preference.skipped)
        #expect(viewModel.finished)
    }

    @Test func startsFreshTheNextTimeItIsOpened() async {
        let viewModel = makeViewModel()
        await recordAndStop(viewModel)
        viewModel.useTapped()
        await viewModel.work?.value

        viewModel.left()

        #expect(viewModel.stage == .intro)
        #expect(!viewModel.finished)
        #expect(viewModel.saved == .seconds(20), "but it knows a voice is now saved")
    }

    @Test func recordsNothingWithoutMicrophonePermission() async {
        let viewModel = makeViewModel(microphoneAllowed: false)

        await startRecording(viewModel)

        #expect(viewModel.stage == .intro)
        #expect(viewModel.error == "Microphone permission is needed to record your voice")
        #expect(recorder.starts == 0)
    }
}
