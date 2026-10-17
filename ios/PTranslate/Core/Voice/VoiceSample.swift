import Foundation
import Observation

/// Whether this phone holds a recording of its owner's voice.
enum VoiceSampleState: Equatable, Sendable {
    case notRecorded
    case recorded(Duration)

    var duration: Duration? {
        if case .recorded(let duration) = self { duration } else { nil }
    }
}

/// Keeps the one recording of the user's voice that translations are spoken in. It lives on the phone and
/// nowhere else: it is sent to the server with each request to speak, and the server deletes it afterwards.
protocol VoiceSampleStore: AnyObject {
    /// What is saved now. Implementations are observable, so a view that reads this is redrawn when it changes.
    var state: VoiceSampleState { get }

    /// Replaces any earlier recording.
    func save(_ audio: PCMAudio) async throws

    /// The recording as a WAV file, ready to upload, or nil if there is none.
    func loadWAV() async throws -> Data?

    func delete() async throws
}

/// What a recording must be like to clone a voice from.
enum VoiceSampleRules {
    /// Below this a voice-cloning engine has too little to go on.
    static let minimum = Duration.seconds(10)

    /// What the passage on screen is written to take when read aloud.
    static let target = Duration.seconds(20)

    /// What is wrong with the recording, in words for the user, or nil if it will do.
    static func problem(with audio: PCMAudio) -> String? {
        if audio.isSilent {
            return "Nothing was heard. Check the microphone and try again"
        }
        if audio.duration < minimum {
            return "That was only \(audio.duration.components.seconds) seconds. About 20 are needed: please read the whole passage"
        }
        return nil
    }
}

/// Stores the recording as one WAV file in a directory of its own.
@Observable
final class FileVoiceSampleStore: VoiceSampleStore {
    private(set) var state: VoiceSampleState

    private let directory: URL
    private let file: URL

    init(directory: URL) {
        self.directory = directory
        file = directory.appending(path: "voice-sample.wav")
        state = Self.readState(of: file)
    }

    func save(_ audio: PCMAudio) async throws {
        try await Self.write(audio.wavData(), to: file, in: directory)
        state = .recorded(audio.duration)
    }

    func loadWAV() async throws -> Data? {
        try await Self.read(file)
    }

    func delete() async throws {
        try await Self.remove(file)
        state = .notRecorded
    }

    // The file work is @concurrent so it runs on the shared pool, not on the caller's actor, which is usually the
    // main one. The state is set back on the caller's actor, where views read it.

    @concurrent
    private static func write(_ wav: Data, to file: URL, in directory: URL) async throws {
        try FileManager.default.createDirectory(at: directory, withIntermediateDirectories: true)
        // The screen promises the recording stays on this phone, so it is kept out of iCloud backups too.
        var excluded = URLResourceValues()
        excluded.isExcludedFromBackup = true
        var directory = directory
        try directory.setResourceValues(excluded)
        // Written to a temporary file and renamed over the old one, so a crash half way leaves the previous
        // recording intact instead of a broken one.
        try wav.write(to: file, options: .atomic)
    }

    @concurrent
    private static func read(_ file: URL) async throws -> Data? {
        guard FileManager.default.fileExists(atPath: file.path(percentEncoded: false)) else { return nil }
        return try Data(contentsOf: file)
    }

    @concurrent
    private static func remove(_ file: URL) async throws {
        guard FileManager.default.fileExists(atPath: file.path(percentEncoded: false)) else { return }
        try FileManager.default.removeItem(at: file)
    }

    private static func readState(of file: URL) -> VoiceSampleState {
        let size = (try? file.resourceValues(forKeys: [.fileSizeKey]).fileSize) ?? 0
        guard size > wavHeaderBytes else { return .notRecorded }
        return .recorded(.milliseconds((size - wavHeaderBytes) / bytesPerMillisecond))
    }

    private static let wavHeaderBytes = 44

    // 16 kHz, 16-bit mono: 32 bytes per millisecond.
    private static let bytesPerMillisecond = PCMAudio.standardSampleRate * 2 / 1000
}

/// Whether the user chose to go without their own voice, so they are not asked on every launch.
protocol VoiceSetupPreference: AnyObject {
    var skipped: Bool { get set }
}

final class UserDefaultsVoiceSetup: VoiceSetupPreference {
    private let defaults: UserDefaults

    init(defaults: UserDefaults) {
        self.defaults = defaults
    }

    var skipped: Bool {
        get { defaults.bool(forKey: Self.key) }
        set { defaults.set(newValue, forKey: Self.key) }
    }

    private static let key = "voice_setup_skipped"
}
