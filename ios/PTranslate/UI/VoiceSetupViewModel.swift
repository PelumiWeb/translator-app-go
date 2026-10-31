import Foundation
import Observation

enum VoiceSetupStage {
    /// Explaining, and waiting for the user to start.
    case intro
    case recording
    /// A recording has been made and is waiting to be kept or redone.
    case review
}

/// Records, reviews and saves the sample of the user's voice.
@MainActor
@Observable
final class VoiceSetupViewModel {
    private(set) var stage = VoiceSetupStage.intro

    /// When the recording in progress began; the screen counts up from it.
    private(set) var recordingStarted: Date?

    /// Length of the recording under review.
    private(set) var recorded = Duration.zero

    /// What is wrong with the recording under review; nil means it will do.
    private(set) var problem: String?

    /// How loud each part of the recording under review is, for drawing its waveform.
    private(set) var levels: [Float] = []

    private(set) var playing = false
    private(set) var error: String?

    /// Set when the user is finished here, one way or another.
    private(set) var finished = false

    /// Length of the recording already saved on this phone, if there is one.
    var saved: Duration? { samples.state.duration }

    /// The work the last action started, the timer that ends a recording, and the playback. Tests wait on them.
    @ObservationIgnored private(set) var work: Task<Void, Never>?
    @ObservationIgnored private(set) var autoStop: Task<Void, Never>?
    @ObservationIgnored private(set) var playback: Task<Void, Never>?

    @ObservationIgnored private let recorder: any AudioRecorder
    @ObservationIgnored private let player: any AudioPlayer
    @ObservationIgnored private let samples: any VoiceSampleStore
    @ObservationIgnored private let preference: any VoiceSetupPreference
    @ObservationIgnored private let requestMicrophone: () async -> Bool
    @ObservationIgnored private let maxRecording: Duration

    @ObservationIgnored private var recording: (any Recording)?

    /// The recording under review. Kept out of the observed state: it is half a megabyte.
    @ObservationIgnored private var candidate: PCMAudio?

    init(
        recorder: any AudioRecorder,
        player: any AudioPlayer,
        samples: any VoiceSampleStore,
        preference: any VoiceSetupPreference,
        requestMicrophone: @escaping () async -> Bool,
        maxRecording: Duration = .seconds(30)
    ) {
        self.recorder = recorder
        self.player = player
        self.samples = samples
        self.preference = preference
        self.requestMicrophone = requestMicrophone
        self.maxRecording = maxRecording
    }

    isolated deinit {
        recording?.cancel()
        player.stop()
    }

    /// Starts recording, or stops it and moves on to the review.
    func recordTapped() {
        switch stage {
        case .intro, .review: work = Task { await startRecording() }
        case .recording: stopRecording()
        }
    }

    /// Plays the recording under review, or stops it if it is playing.
    func listenTapped() {
        if playing {
            stopPlayback()
            return
        }
        guard let audio = candidate else { return }
        playing = true
        playback = Task {
            defer { playing = false }
            do {
                try await player.play(audio)
            } catch {
                self.error = error.localizedDescription
            }
        }
    }

    /// Keeps the recording under review as the user's voice.
    func useTapped() {
        guard let audio = candidate, problem == nil else { return }
        stopPlayback()
        work = Task {
            do {
                try await samples.save(audio)
            } catch {
                self.error = "The recording could not be saved"
                return
            }
            preference.skipped = false
            candidate = nil
            finished = true
        }
    }

    /// Leaves without recording. The phone's own voice is used, and the app does not ask again.
    func skipTapped() {
        // Only "do not ask again" if there is nothing saved; with a saved voice this button just means "keep it
        // as it is".
        if saved == nil {
            preference.skipped = true
        }
        discardRecording()
        finished = true
    }

    /// Deletes the saved recording from the phone.
    func deleteTapped() {
        discardRecording()
        work = Task {
            do {
                try await samples.delete()
            } catch {
                self.error = "The recording could not be deleted"
                return
            }
            preference.skipped = true
            finished = true
        }
    }

    /// Called when the screen has been left, so it starts fresh next time.
    func left() {
        stage = .intro
        recordingStarted = nil
        recorded = .zero
        problem = nil
        playing = false
        error = nil
        finished = false
    }

    private func startRecording() async {
        stopPlayback()
        guard await requestMicrophone() else {
            error = "Microphone permission is needed to record your voice"
            return
        }
        // A second tap while iOS was asking for permission has already started one.
        guard stage != .recording else { return }

        do {
            recording = try recorder.start()
        } catch {
            self.error = error.localizedDescription
            return
        }
        candidate = nil
        stage = .recording
        recordingStarted = .now
        problem = nil
        error = nil

        let limit = maxRecording
        autoStop = Task { [weak self] in
            try? await Task.sleep(for: limit)
            guard !Task.isCancelled else { return }
            self?.stopRecording()
        }
    }

    private func stopRecording() {
        guard let finished = recording else { return }
        recording = nil
        autoStop?.cancel()
        autoStop = nil

        let audio = finished.stop()
        candidate = audio
        stage = .review
        recordingStarted = nil
        recorded = audio.duration
        problem = VoiceSampleRules.problem(with: audio)
        levels = audio.levels(Self.waveformDots)
    }

    /// As many dots as the design's waveform has.
    static let waveformDots = 22

    private func discardRecording() {
        stopPlayback()
        autoStop?.cancel()
        autoStop = nil
        recording?.cancel()
        recording = nil
        candidate = nil
    }

    private func stopPlayback() {
        player.stop()
        playback?.cancel()
    }
}
