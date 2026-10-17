import Foundation
import Testing
@testable import PTranslate

/// Runs the real whisper.cpp build on the simulator. The model is the one `make model-download` puts in the
/// server's folder, and the clip is whisper.cpp's own sample; a simulator can read the Mac's files, so both are
/// used where they are instead of being copied into the test bundle.
struct WhisperContextTests {
    /// The repository root: this file is at ios/PTranslateTests/ in it.
    private static let repository = URL(filePath: #filePath)
        .deletingLastPathComponent()
        .deletingLastPathComponent()
        .deletingLastPathComponent()
    private static let model = repository.appending(path: "server/data/models/ggml-base-q5_1.bin")
    private static let clip = repository.appending(path: "third_party/whisper.cpp/samples/jfk.wav")
    private static let modelDownloaded = FileManager.default.fileExists(atPath: model.path(percentEncoded: false))

    @Test(.enabled(if: modelDownloaded, "no model in server/data/models; run `make model-download`"))
    func transcribesSpeechAndReportsConfidence() async throws {
        let whisper = try await WhisperContext.load(Self.model)
        let speech = try PCMAudio(wav: Data(contentsOf: Self.clip)) // eleven seconds of English

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
            try await WhisperContext.load(Self.clip)
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
