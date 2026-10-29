import Foundation
import Synchronization
@testable import PTranslate

/// Audio of the given length that is loud enough to count as speech.
func speech(seconds: Int) -> PCMAudio {
    PCMAudio(samples: (0..<seconds * 16_000).map { $0 % 2 == 0 ? 2000 : -2000 })
}

final class FakeRecorder: AudioRecorder {
    /// What the next recording returns when stopped.
    var audio: PCMAudio
    var failure: RecorderError?
    private(set) var starts = 0
    var cancelled = 0

    init(audio: PCMAudio, failure: RecorderError? = nil) {
        self.audio = audio
        self.failure = failure
    }

    func start() throws -> any Recording {
        if let failure {
            throw failure
        }
        starts += 1
        return FakeRecording(recorder: self, audio: audio)
    }
}

final class FakeRecording: Recording {
    private let recorder: FakeRecorder
    private let audio: PCMAudio

    init(recorder: FakeRecorder, audio: PCMAudio) {
        self.recorder = recorder
        self.audio = audio
    }

    func stop() -> PCMAudio { audio }

    func cancel() {
        recorder.cancelled += 1
    }
}

final class FakePlayer: AudioPlayer {
    private(set) var played: [PCMAudio] = []
    private(set) var stops = 0

    /// When true, `play` does not return until `stop()` is called, like a clip that is still playing.
    var holds = false
    private var pending: CheckedContinuation<Void, Never>?

    func play(_ audio: PCMAudio) async throws {
        played.append(audio)
        guard holds else { return }
        await withCheckedContinuation { pending = $0 }
    }

    func stop() {
        stops += 1
        pending?.resume()
        pending = nil
    }
}

final class FakeVoiceStore: VoiceSampleStore {
    private(set) var state = VoiceSampleState.notRecorded
    private(set) var saved: PCMAudio?

    func save(_ audio: PCMAudio) async throws {
        saved = audio
        state = .recorded(audio.duration)
    }

    func loadWAV() async throws -> Data? {
        saved?.wavData()
    }

    func delete() async throws {
        saved = nil
        state = .notRecorded
    }
}

final class FakePreference: VoiceSetupPreference {
    var skipped = false
}

/// Emits the scripted events, then ends or fails.
final class FakeTranscriber: Transcriber {
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

/// Answers with whatever the test sends, when it sends it, so that time can pass between the start and the result.
/// For one transcription only.
final class ControlledTranscriber: Transcriber {
    private let stream: AsyncThrowingStream<TranscriptEvent, any Error>
    private let continuation: AsyncThrowingStream<TranscriptEvent, any Error>.Continuation

    init() {
        (stream, continuation) = AsyncThrowingStream.makeStream()
    }

    func transcribe(_ audio: PCMAudio, language: Language) -> AsyncThrowingStream<TranscriptEvent, any Error> {
        stream
    }

    /// Sends the last event, and ends the transcription.
    func send(_ event: TranscriptEvent) {
        continuation.yield(event)
        continuation.finish()
    }
}

@MainActor
final class FakeModels: ModelInstaller {
    static let file = URL(filePath: "/model.bin")

    var state = ModelState.missing
    private(set) var installCalls = 0

    /// When set, installing fails with this reason.
    var failure: String?

    func ensureInstalled() async throws(ModelError) -> URL {
        installCalls += 1
        if let failure {
            state = .failed(failure)
            throw ModelError(message: failure)
        }
        state = .ready(Self.file)
        return Self.file
    }
}

/// A clock that moves only when the test moves it.
final class FakeClock {
    var elapsed = Duration.zero
}
