import Foundation
import Observation

/// The voice note in a room's composer: recording, paused, or stopped to be listened to before it is sent. Only the
/// time spent recording counts toward the limit; at the limit the note stops and waits to be sent, rather than being
/// sent for the person.
@MainActor
@Observable
final class NoteRecorder {
    enum Phase: Equatable {
        case idle, recording
        /// The microphone is off; recording can go on.
        case paused
        /// Finished, to listen to, then send or throw away.
        case stopped
    }

    private(set) var phase = Phase.idle
    /// The finished note, while stopped.
    private(set) var audio: PCMAudio?

    /// Time recorded before the current stretch, and when that stretch began on `now`'s clock, while recording.
    private var banked = Duration.zero
    private var stretchFrom: Duration?

    @ObservationIgnored private let recorder: any AudioRecorder
    @ObservationIgnored private let maxNote: Duration
    @ObservationIgnored private let now: () -> Duration
    @ObservationIgnored private var recording: (any Recording)?
    @ObservationIgnored private var limit: Task<Void, Never>?

    /// - Parameter now: a clock, as time since any fixed point.
    init(recorder: any AudioRecorder, maxNote: Duration, now: @escaping () -> Duration = { noteClockOrigin.duration(to: .now) }) {
        self.recorder = recorder
        self.maxNote = maxNote
        self.now = now
    }

    isolated deinit {
        limit?.cancel()
        recording?.cancel()
    }

    /// Time recorded so far; for a stopped note, its length.
    var elapsed: Duration {
        guard let stretchFrom else { return banked }
        return banked + (now() - stretchFrom)
    }

    /// Starts a new note. Throws `RecorderError` when the microphone cannot be opened.
    func start() throws {
        guard phase == .idle else { return }
        recording = try recorder.start()
        banked = .zero
        stretchFrom = now()
        phase = .recording
        runLimit()
    }

    func pause() {
        guard phase == .recording, let recording else { return }
        limit?.cancel()
        banked = elapsed
        stretchFrom = nil
        recording.pause()
        phase = .paused
    }

    func resume() {
        guard phase == .paused, let recording else { return }
        recording.resume()
        stretchFrom = now()
        phase = .recording
        runLimit()
    }

    /// Ends the recording, to be listened to before it is sent.
    func stop() {
        guard let live = recording else { return }
        recording = nil
        limit?.cancel()
        let audio = live.stop()
        self.audio = audio
        banked = audio.duration
        stretchFrom = nil
        phase = .stopped
    }

    /// Ends the note, in whatever phase, and hands it over; the composer is empty again.
    func finish() -> PCMAudio? {
        stop()
        defer { clear() }
        return audio
    }

    /// Throws the note away.
    func cancel() {
        limit?.cancel()
        recording?.cancel()
        recording = nil
        clear()
    }

    private func clear() {
        audio = nil
        banked = .zero
        stretchFrom = nil
        phase = .idle
    }

    /// Stops the note when the time left is used up. Restarted on each resume, from what is left then.
    private func runLimit() {
        let left = maxNote - banked
        limit = Task { [weak self] in
            try? await Task.sleep(for: left)
            guard !Task.isCancelled else { return }
            self?.stop()
        }
    }
}

private let noteClockOrigin = ContinuousClock.now
