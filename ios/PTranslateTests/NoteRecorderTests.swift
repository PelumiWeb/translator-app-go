import Foundation
import Testing
@testable import PTranslate

/// The composer's voice note: pause, stop to listen, send, and the limit. The same cases as the Android tests.
@MainActor
struct NoteRecorderTests {
    private let recorder = FakeRecorder(audio: speech(seconds: 3))
    private let clock = FakeClock()

    private func notes(maxNote: Duration = .seconds(30)) -> NoteRecorder {
        NoteRecorder(recorder: recorder, maxNote: maxNote, now: { [clock] in clock.elapsed })
    }

    @Test func pausingStopsTheClockAndResumingCarriesItOn() throws {
        let notes = notes()
        try notes.start()
        clock.elapsed = .seconds(3)

        notes.pause()
        clock.elapsed = .seconds(8)
        #expect(notes.phase == .paused)
        #expect(notes.elapsed == .seconds(3))

        notes.resume()
        clock.elapsed = .seconds(10)
        #expect(notes.phase == .recording)
        #expect(notes.elapsed == .seconds(5))
        #expect(recorder.asked == ["start", "pause", "resume"])
    }

    @Test func stoppingEndsTheRecordingToListenToBeforeSending() throws {
        let notes = notes()
        try notes.start()
        clock.elapsed = .seconds(3)

        notes.stop()

        #expect(notes.phase == .stopped)
        #expect(notes.audio == speech(seconds: 3))
        #expect(notes.elapsed == .seconds(3))
        #expect(recorder.asked == ["start", "stop"])
    }

    @Test func finishingHandsOverTheNoteFromAnyPhase() throws {
        let notes = notes()
        try notes.start()
        #expect(notes.finish() == speech(seconds: 3))
        #expect(notes.phase == .idle)

        try notes.start()
        notes.pause()
        #expect(notes.finish() == speech(seconds: 3))

        try notes.start()
        notes.stop()
        #expect(notes.finish() == speech(seconds: 3))
        #expect(notes.audio == nil)
    }

    @Test func atTheLimitTheNoteStopsAndWaitsToBeSent() async throws {
        let notes = notes(maxNote: .milliseconds(20))
        try notes.start()

        // Waited for rather than slept past: other tests on the main actor can delay the limit's task.
        let deadline = ContinuousClock.now + .seconds(5)
        while notes.phase != .stopped, ContinuousClock.now < deadline {
            try await Task.sleep(for: .milliseconds(10))
        }

        #expect(notes.phase == .stopped)
        #expect(notes.audio == speech(seconds: 3))
    }

    @Test func timePausedDoesNotCountTowardTheLimit() throws {
        let notes = notes()
        try notes.start()
        clock.elapsed = .seconds(20)
        notes.pause()
        clock.elapsed = .seconds(40)
        notes.resume()
        clock.elapsed = .seconds(45)

        #expect(notes.phase == .recording)
        #expect(notes.elapsed == .seconds(25))
    }

    @Test func cancellingThrowsTheNoteAway() throws {
        let notes = notes()
        try notes.start()
        notes.pause()

        notes.cancel()

        #expect(notes.phase == .idle)
        #expect(notes.audio == nil)
        #expect(recorder.asked.contains("cancel"))
        #expect(notes.finish() == nil)
    }
}
