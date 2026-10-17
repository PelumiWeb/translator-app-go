import Foundation
import Testing
@testable import PTranslate

struct VoiceSampleTests {
    /// A fresh directory per test, which does not exist yet.
    private let directory = FileManager.default.temporaryDirectory.appending(path: "voice-\(UUID().uuidString)")

    private func files() throws -> [String] {
        try FileManager.default.contentsOfDirectory(atPath: directory.path(percentEncoded: false)).sorted()
    }

    // MARK: What makes a recording usable

    @Test func aClearRecordingOfThePassageIsAccepted() {
        #expect(VoiceSampleRules.problem(with: speech(seconds: 20)) == nil)
        #expect(VoiceSampleRules.problem(with: speech(seconds: 10)) == nil, "exactly the minimum")
    }

    @Test func aRecordingThatIsTooShortSaysHowLongItWas() {
        #expect(
            VoiceSampleRules.problem(with: speech(seconds: 4))
                == "That was only 4 seconds. About 20 are needed: please read the whole passage"
        )
    }

    @Test func aSilentRecordingIsReportedAsAMicrophoneProblemWhateverItsLength() {
        let silence = PCMAudio(samples: Array(repeating: 0, count: 20 * 16_000))

        #expect(VoiceSampleRules.problem(with: silence) == "Nothing was heard. Check the microphone and try again")
    }

    // MARK: Keeping it on the phone

    @Test func startsWithNoRecording() async throws {
        let store = FileVoiceSampleStore(directory: directory)

        #expect(store.state == .notRecorded)
        #expect(try await store.loadWAV() == nil)
    }

    @Test func savesTheRecordingAsAWAVFileThatCanBeReadBack() async throws {
        let store = FileVoiceSampleStore(directory: directory)
        let audio = speech(seconds: 12)

        try await store.save(audio)

        #expect(store.state == .recorded(.seconds(12)))
        let wav = try #require(try await store.loadWAV())
        #expect(try PCMAudio(wav: wav) == audio)
    }

    @Test func theRecordingIsStillThereAfterTheAppRestarts() async throws {
        try await FileVoiceSampleStore(directory: directory).save(speech(seconds: 15))

        let afterRestart = FileVoiceSampleStore(directory: directory)

        #expect(afterRestart.state == .recorded(.seconds(15)))
        #expect(try await afterRestart.loadWAV()?.count ?? 0 > 44)
    }

    @Test func aNewRecordingReplacesTheOldOne() async throws {
        let store = FileVoiceSampleStore(directory: directory)
        try await store.save(speech(seconds: 12))

        try await store.save(speech(seconds: 20))

        #expect(store.state == .recorded(.seconds(20)))
        let wav = try #require(try await store.loadWAV())
        #expect(try PCMAudio(wav: wav).samples.count == 20 * 16_000)
        // Only the one file: no leftovers from the first, no temporary file.
        #expect(try files() == ["voice-sample.wav"])
    }

    @Test func deletingRemovesItFromThePhone() async throws {
        let store = FileVoiceSampleStore(directory: directory)
        try await store.save(speech(seconds: 12))

        try await store.delete()

        #expect(store.state == .notRecorded)
        #expect(try await store.loadWAV() == nil)
        #expect(try files().isEmpty)
        try await store.delete() // deleting when there is nothing is harmless
    }

    // The screen tells the user the recording stays on this phone.
    @Test func theRecordingIsKeptOutOfICloudBackups() async throws {
        try await FileVoiceSampleStore(directory: directory).save(speech(seconds: 12))

        let values = try directory.resourceValues(forKeys: [.isExcludedFromBackupKey])

        #expect(values.isExcludedFromBackup == true)
    }
}
