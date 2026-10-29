import Foundation
import Observation

/// How fast this phone runs the speech model, for the screen and for routing.
@MainActor
protocol DeviceSpeed: AnyObject {
    /// Time taken divided by audio length for the benchmark clip: 0.5 means twice as fast as the speech itself. Nil
    /// until it has been measured. Observable, like everything here.
    var realTimeFactor: Double? { get }

    var measuring: Bool { get }

    /// Measures again, replacing the stored figure.
    func measureAgain() async
}

/// Where a measurement is kept between runs of the app.
protocol SpeedStore {
    func load(modelVersion: String) -> Double?
    func save(modelVersion: String, realTimeFactor: Double)
}

final class UserDefaultsSpeedStore: SpeedStore {
    private let defaults: UserDefaults

    init(defaults: UserDefaults) {
        self.defaults = defaults
    }

    func load(modelVersion: String) -> Double? {
        let stored = defaults.double(forKey: Self.key(modelVersion))
        return stored > 0 ? stored : nil
    }

    func save(modelVersion: String, realTimeFactor: Double) {
        defaults.set(realTimeFactor, forKey: Self.key(modelVersion))
    }

    private static func key(_ modelVersion: String) -> String {
        "real_time_factor:\(modelVersion)"
    }
}

/// Times the on-device model on a fixed clip of real speech.
///
/// The figure is stored per model version, because a different model has a different speed, and measured once: it
/// costs several seconds of full CPU.
@MainActor
@Observable
final class DeviceBenchmark: DeviceSpeed {
    private(set) var realTimeFactor: Double?
    private(set) var measuring = false

    @ObservationIgnored private let modelVersion: () -> String?
    @ObservationIgnored private let sample: () async throws -> PCMAudio
    @ObservationIgnored private let prepare: () async throws -> Void
    @ObservationIgnored private let transcribe: (PCMAudio) async throws -> Void
    @ObservationIgnored private let store: any SpeedStore
    @ObservationIgnored private let now: () -> Duration

    /// - Parameters:
    ///   - modelVersion: identifies the installed model, or nil if there is none.
    ///   - sample: the benchmark clip.
    ///   - prepare: loads the model, so loading is not counted in the timing.
    ///   - transcribe: runs the model on the clip; its result is ignored.
    ///   - now: a clock, as time since any fixed point.
    init(
        modelVersion: @escaping () -> String?,
        sample: @escaping () async throws -> PCMAudio,
        prepare: @escaping () async throws -> Void,
        transcribe: @escaping (PCMAudio) async throws -> Void,
        store: any SpeedStore,
        now: @escaping () -> Duration = { ContinuousClock.now - clockOrigin }
    ) {
        self.modelVersion = modelVersion
        self.sample = sample
        self.prepare = prepare
        self.transcribe = transcribe
        self.store = store
        self.now = now
    }

    /// Uses the stored figure for the installed model, or measures if there is none.
    func ensureMeasured() async {
        guard let version = modelVersion() else { return }
        if let stored = store.load(modelVersion: version) {
            realTimeFactor = stored
        } else {
            await measure(version)
        }
    }

    func measureAgain() async {
        guard let version = modelVersion() else { return }
        await measure(version)
    }

    private func measure(_ version: String) async {
        // One measurement at a time: two at once would halve each other's speed. A call during one is dropped,
        // since the running one will produce the figure.
        guard !measuring else { return }
        measuring = true
        defer { measuring = false }

        do {
            let audio = try await sample()
            try await prepare()
            let started = now()
            try await transcribe(audio)
            let factor = (now() - started) / audio.duration

            store.save(modelVersion: version, realTimeFactor: factor)
            realTimeFactor = factor
        } catch {
            // The model would not run. Leave the speed unknown: routing then tries the phone anyway and falls back
            // if that fails too.
            realTimeFactor = nil
        }
    }
}

/// The fixed point the default clock measures from.
private let clockOrigin = ContinuousClock.now
