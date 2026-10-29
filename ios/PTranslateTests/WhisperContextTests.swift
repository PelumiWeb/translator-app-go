import Foundation
import Testing
@testable import PTranslate

/// Runs the real whisper.cpp build on the simulator, with the model and clip from `Fixtures`.
struct WhisperContextTests {
    @Test(.enabled(if: Fixtures.modelDownloaded, Comment(rawValue: Fixtures.noModel)))
    func transcribesSpeechAndReportsConfidence() async throws {
        let whisper = try await WhisperContext.load(Fixtures.model)
        let speech = try PCMAudio(wav: Data(contentsOf: Fixtures.clip)) // eleven seconds of English

        let result = try await whisper.transcribe(speech.floatSamples, language: .english, threads: 4)

        #expect(result.text.localizedCaseInsensitiveContains("ask not what your country"), "transcript: \(result.text)")
        #expect((result.meanTokenProbability ?? 0) > 0.5, "confidence: \(String(describing: result.meanTokenProbability))")
        #expect((result.noSpeechProbability ?? 1) < 0.5, "no-speech: \(String(describing: result.noSpeechProbability))")
    }

    @Test func aMissingModelIsReportedPlainly() async {
        await #expect(throws: TranscriptionError.failed("No model file at /does/not/exist.bin")) {
            try await WhisperContext.load(URL(filePath: "/does/not/exist.bin"))
        }
    }

    @Test func aFileThatIsNotAModelIsRefused() async {
        await #expect(throws: TranscriptionError.failed("jfk.wav is not a usable Whisper model")) {
            try await WhisperContext.load(Fixtures.clip)
        }
    }

    // The build script targets the A13, the oldest chip iOS 26 runs on. Without that the compiler assumes a far
    // older one, and the instructions below, which the quantised model's fast paths use, are left out.
    @Test func isBuiltWithTheFastArmInstructions() {
        let info = WhisperContext.systemInfo

        #expect(info.contains("NEON = 1"), "\(info)")
        #expect(info.contains("DOTPROD = 1"), "\(info)")
    }
}
