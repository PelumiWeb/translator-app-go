import Foundation
import Testing
@testable import PTranslate

/// The same cases as Android's device tests, run on the simulator with the real model.
struct WhisperTranscriberTests {
    private func transcriber(model: URL?) -> WhisperTranscriber {
        WhisperTranscriber(modelFile: { model }, threads: 4)
    }

    private func events(_ transcriber: WhisperTranscriber, _ audio: PCMAudio) async throws -> [TranscriptEvent] {
        var events: [TranscriptEvent] = []
        for try await event in transcriber.transcribe(audio, language: .english) {
            events.append(event)
        }
        return events
    }

    @Test(.enabled(if: Fixtures.modelDownloaded, Comment(rawValue: Fixtures.noModel)))
    func transcribesSpeechAndReportsConfidence() async throws {
        let speech = try PCMAudio(wav: Data(contentsOf: Fixtures.clip))

        let events = try await events(transcriber(model: Fixtures.model), speech)

        #expect(events.first == .stageChanged(.processing))
        guard case .final(let transcript)? = events.last else {
            Issue.record("no transcript: \(events)")
            return
        }
        #expect(transcript.text.localizedCaseInsensitiveContains("ask not what your country"), "transcript: \(transcript.text)")
        #expect(transcript.source == .onDevice)
        #expect((transcript.confidence ?? 0) > 0.5, "confidence: \(String(describing: transcript.confidence))")
    }

    @Test func silenceIsReportedWithoutRunningTheModel() async {
        // No model at all: the silence check must answer before one is needed.
        let twoSilentSeconds = PCMAudio(samples: Array(repeating: 0, count: 32_000))

        await #expect(throws: TranscriptionError.silentAudio("The recording was silent (-100 dB). Check the microphone")) {
            try await events(transcriber(model: URL(filePath: "/does/not/exist.bin")), twoSilentSeconds)
        }
    }

    // Loud enough to get past the silence check, so the model itself has to recognise that there are no words in it.
    @Test(.enabled(if: Fixtures.modelDownloaded, Comment(rawValue: Fixtures.noModel)))
    func noiseIsReportedAsNoSpeech() async {
        var random = SeededRandom(seed: 1)
        let threeSecondsOfHiss = PCMAudio(samples: (0..<48_000).map { _ in Int16.random(in: -400..<400, using: &random) })
        #expect(!threeSecondsOfHiss.isSilent, "the hiss must be above the silence threshold")

        await #expect(throws: TranscriptionError.noSpeech("No speech was recognised")) {
            try await events(transcriber(model: Fixtures.model), threeSecondsOfHiss)
        }
    }

    @Test func aMissingModelIsReportedPlainly() async {
        await #expect(throws: TranscriptionError.failed("The on-device model is not installed")) {
            try await events(transcriber(model: nil), speech(seconds: 2))
        }
    }
}

/// The same numbers on every run, so the hiss test does not change from one run to the next. SplitMix64.
private struct SeededRandom: RandomNumberGenerator {
    private var state: UInt64

    init(seed: UInt64) {
        state = seed
    }

    mutating func next() -> UInt64 {
        state &+= 0x9E37_79B9_7F4A_7C15
        var z = state
        z = (z ^ (z >> 30)) &* 0xBF58_476D_1CE4_E5B9
        z = (z ^ (z >> 27)) &* 0x94D0_49BB_1331_11EB
        return z ^ (z >> 31)
    }
}
