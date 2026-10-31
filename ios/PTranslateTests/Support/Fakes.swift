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

/// Translates "hello" from en to es as "[en>es] hello", and records what it was asked.
final class FakeTranslator: Translator {
    private struct State {
        var ready: Bool
        var prepareError: TranslationError?
        var translateError: TranslationError?
        var calls: [String] = []
    }

    private let languages: [Language]
    private let state: Mutex<State>

    init(
        ready: Bool = true,
        languages: [Language] = [Language("en"), Language("es"), Language("yo")],
        prepareError: TranslationError? = nil,
        translateError: TranslationError? = nil
    ) {
        self.languages = languages
        state = Mutex(State(ready: ready, prepareError: prepareError, translateError: translateError))
    }

    var calls: [String] { state.withLock { $0.calls } }

    var supportedLanguages: [Language] {
        get async { languages }
    }

    func isReady(source: Language, target: Language) async -> Bool {
        state.withLock { $0.ready }
    }

    func prepare(source: Language, target: Language) async throws(TranslationError) {
        let error = state.withLock { state in
            state.calls.append("prepare \(source.tag)>\(target.tag)")
            if state.prepareError == nil {
                state.ready = true
            }
            return state.prepareError
        }
        if let error {
            throw error
        }
    }

    func translate(_ text: String, source: Language, target: Language) async throws(TranslationError) -> String {
        let error = state.withLock { state in
            state.calls.append("translate \(source.tag)>\(target.tag)")
            return state.translateError
        }
        if let error {
            throw error
        }
        return "[\(source.tag)>\(target.tag)] \(text)"
    }
}

/// Records what it was asked to say. Speaking ends at once, unless `holds`: then it lasts until `stop()` or
/// `finish()`.
final class FakeSynthesizer: SpeechSynthesizer {
    private struct State {
        var spoken: [String] = []
        var stops = 0
        var speaking: CheckedContinuation<Void, Never>?
        var waitingForSpeech: CheckedContinuation<Void, Never>?
    }

    private let failure: SpeechError?
    private let holds: Bool
    private let state = Mutex(State())

    init(failure: SpeechError? = nil, holds: Bool = false) {
        self.failure = failure
        self.holds = holds
    }

    var spoken: [String] { state.withLock { $0.spoken } }
    var stops: Int { state.withLock { $0.stops } }

    func speak(_ text: String, language: Language) async throws(SpeechError) {
        if let failure {
            throw failure
        }
        state.withLock { $0.spoken.append("\(language.tag): \(text)") }
        guard holds else { return }
        await withCheckedContinuation { continuation in
            let waiting = state.withLock { state in
                state.speaking = continuation
                defer { state.waitingForSpeech = nil }
                return state.waitingForSpeech
            }
            waiting?.resume()
        }
    }

    func stop() {
        state.withLock { state in
            state.stops += 1
            defer { state.speaking = nil }
            return state.speaking
        }?.resume()
    }

    /// Ends a held utterance as if it had been said to the end.
    func finish() {
        state.withLock { state in
            defer { state.speaking = nil }
            return state.speaking
        }?.resume()
    }

    /// Returns once a held utterance has started.
    func untilSpeaking() async {
        await withCheckedContinuation { continuation in
            let started = state.withLock { state in
                if state.speaking == nil {
                    state.waitingForSpeech = continuation
                }
                return state.speaking != nil
            }
            if started {
                continuation.resume()
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

@MainActor
final class FakeSpeed: DeviceSpeed {
    var realTimeFactor: Double?
    var measuring = false
    private(set) var measurements = 0

    func measureAgain() async {
        measurements += 1
        realTimeFactor = 0.5
    }
}

/// A clock that moves only when the test moves it.
final class FakeClock {
    var elapsed = Duration.zero
}
