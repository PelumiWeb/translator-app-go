import Testing
@testable import PTranslate

@MainActor
struct DeviceBenchmarkTests {
    private final class MemoryStore: SpeedStore {
        var saved: [String: Double] = [:]

        func load(modelVersion: String) -> Double? { saved[modelVersion] }

        func save(modelVersion: String, realTimeFactor: Double) {
            saved[modelVersion] = realTimeFactor
        }
    }

    /// What the benchmark is given, changeable by each test.
    private final class Setup {
        var modelVersion: String? = "base:100"
        var clock = Duration.zero
        var transcription = Duration.seconds(4)
        var failure: TranscriptionError?
        var steps: [String] = []
    }

    private let store = MemoryStore()
    private let setup = Setup()

    // The clip is ten seconds long, so the real-time factor is the transcription time in seconds divided by ten.
    private func makeBenchmark() -> DeviceBenchmark {
        DeviceBenchmark(
            modelVersion: { [setup] in setup.modelVersion },
            sample: { PCMAudio(samples: Array(repeating: 0, count: 160_000)) },
            prepare: { [setup] in
                setup.steps.append("prepare")
                setup.clock += .seconds(9) // loading the model is slow, and must not be counted
            },
            transcribe: { [setup] _ in
                setup.steps.append("transcribe")
                if let failure = setup.failure {
                    throw failure
                }
                setup.clock += setup.transcription
            },
            store: store,
            now: { [setup] in setup.clock }
        )
    }

    @Test func measuresTranscriptionTimeAgainstAudioLengthLeavingOutModelLoading() async {
        let benchmark = makeBenchmark()

        await benchmark.ensureMeasured()

        #expect(benchmark.realTimeFactor == 0.4)
        #expect(setup.steps == ["prepare", "transcribe"])
        #expect(store.saved == ["base:100": 0.4])
        #expect(!benchmark.measuring)
    }

    @Test func usesTheStoredFigureInsteadOfMeasuringAgain() async {
        store.saved["base:100"] = 0.25
        let benchmark = makeBenchmark()

        await benchmark.ensureMeasured()

        #expect(benchmark.realTimeFactor == 0.25)
        #expect(setup.steps.isEmpty)
    }

    @Test func aDifferentModelIsMeasuredAfresh() async {
        store.saved["tiny:50"] = 0.1
        let benchmark = makeBenchmark()

        await benchmark.ensureMeasured()

        #expect(benchmark.realTimeFactor == 0.4)
        #expect(Set(store.saved.keys) == ["tiny:50", "base:100"])
    }

    @Test func measureAgainReplacesTheStoredFigure() async {
        let benchmark = makeBenchmark()
        await benchmark.ensureMeasured()
        setup.transcription = .seconds(12)

        await benchmark.measureAgain()

        #expect(benchmark.realTimeFactor == 1.2)
        #expect(store.saved["base:100"] == 1.2)
    }

    @Test func doesNothingWhileThereIsNoModel() async {
        setup.modelVersion = nil
        let benchmark = makeBenchmark()

        await benchmark.ensureMeasured()
        await benchmark.measureAgain()

        #expect(benchmark.realTimeFactor == nil)
        #expect(setup.steps.isEmpty)
    }

    @Test func aModelThatWillNotRunLeavesTheSpeedUnknown() async {
        setup.failure = .failed("The on-device model failed to run")
        let benchmark = makeBenchmark()

        await benchmark.ensureMeasured()

        #expect(benchmark.realTimeFactor == nil)
        #expect(store.saved.isEmpty)
        #expect(!benchmark.measuring)
    }
}
