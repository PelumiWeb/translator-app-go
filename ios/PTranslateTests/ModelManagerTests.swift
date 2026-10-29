import CryptoKit
import Foundation
import Observation
import Synchronization
import Testing
@testable import PTranslate

/// Runs the real `BackendClient` and real files against a stub server that behaves like the Go one: a manifest,
/// and a model endpoint that honours Range and If-Range. The same cases as the Android tests.
@MainActor
struct ModelManagerTests {
    private let server = StubServer()
    private let script: ModelServer
    private let directory = FileManager.default.temporaryDirectory.appending(path: "models-\(UUID().uuidString)")

    // 600 KB of non-repeating bytes: enough for several progress updates.
    private let model = Data((0..<600_000).map { UInt8(truncatingIfNeeded: $0 * 31 + $0 / 7) })

    private var installed: URL { directory.appending(path: "base.bin") }
    private var partial: URL { directory.appending(path: "base.part") }
    private var partialSHA: URL { directory.appending(path: "base.part.sha256") }

    init() throws {
        script = ModelServer(model: model)
        server.respond { [script] request in script.answer(request) }
        try FileManager.default.createDirectory(at: directory, withIntermediateDirectories: true)
    }

    private func manager(modelID: String = "base") -> ModelManager {
        ModelManager(
            backend: BackendClient(baseURL: server.baseURL, session: server.session),
            directory: directory,
            modelID: modelID
        )
    }

    private var modelRequests: [StubServer.Request] {
        server.requests.filter { $0.path == "/v1/models/base" }
    }

    private func exists(_ file: URL) -> Bool {
        FileManager.default.fileExists(atPath: file.path(percentEncoded: false))
    }

    private func size(of file: URL) -> Int {
        ((try? FileManager.default.attributesOfItem(atPath: file.path(percentEncoded: false)))?[.size] as? Int) ?? 0
    }

    @Test func downloadsVerifiesAndInstallsTheModel() async throws {
        let manager = manager()
        #expect(manager.state == .missing)

        let file = try await manager.ensureInstalled()

        #expect(file == installed)
        #expect(try Data(contentsOf: installed) == model)
        #expect(manager.state == .ready(installed))
        #expect(!exists(partial) && !exists(partialSHA), "partial file left behind")
        #expect(modelRequests.count == 1)
        #expect(modelRequests.first?.header("Range") == nil, "a fresh download must not ask for a range")
    }

    @Test func rejectsADownloadWhoseChecksumDoesNotMatchTheManifest() async {
        script.update { $0.manifestSHA = String(repeating: "0", count: 64) }
        let manager = manager()

        let error = await #expect(throws: ModelError.self) { try await manager.ensureInstalled() }

        #expect(error?.message == "The downloaded model is corrupt (checksum mismatch)")
        #expect(manager.state == .failed("The downloaded model is corrupt (checksum mismatch)"))
        #expect(!exists(installed), "a corrupt model must never be installed")
        #expect(!exists(partial), "a corrupt download must not be kept for resuming")
    }

    @Test func resumesAPartialDownloadFromWhereItStopped() async throws {
        try model.prefix(200_000).write(to: partial)
        try sha256(model).write(to: partialSHA, atomically: true, encoding: .utf8)

        _ = try await manager().ensureInstalled()

        #expect(modelRequests.count == 1)
        #expect(modelRequests.first?.header("Range") == "bytes=200000-")
        #expect(modelRequests.first?.header("If-Range") == "\"\(sha256(model))\"")
        #expect(try Data(contentsOf: installed) == model)
    }

    @Test func startsOverWhenTheServerIgnoresTheRangeAndSendsEverything() async throws {
        script.update { $0.honourRanges = false }
        try model.prefix(200_000).write(to: partial)
        try sha256(model).write(to: partialSHA, atomically: true, encoding: .utf8)

        _ = try await manager().ensureInstalled()

        // Had the full body been appended to the 200 KB, the file would be 800 KB and fail its checksum.
        #expect(try Data(contentsOf: installed) == model)
    }

    @Test func discardsAPartialDownloadThatBelongsToADifferentVersion() async throws {
        try Data(repeating: 7, count: 200_000).write(to: partial) // the start of some older model
        try String(repeating: "f", count: 64).write(to: partialSHA, atomically: true, encoding: .utf8)

        _ = try await manager().ensureInstalled()

        #expect(modelRequests.first?.header("Range") == nil, "must not resume onto another file's bytes")
        #expect(try Data(contentsOf: installed) == model)
    }

    @Test func discardsAPartialDownloadWithNoRecordOfWhatItBelongsTo() async throws {
        try model.prefix(200_000).write(to: partial)

        _ = try await manager().ensureInstalled()

        #expect(modelRequests.first?.header("Range") == nil)
        #expect(try Data(contentsOf: installed) == model)
    }

    @Test func aDroppedConnectionKeepsThePartialFileAndTheNextAttemptFinishesIt() async throws {
        let manager = manager()
        script.update { $0.dropAfterBytes = 250_000 }

        let error = await #expect(throws: ModelError.self) { try await manager.ensureInstalled() }

        #expect(error?.message == "The download was interrupted")
        #expect(!exists(installed))
        let kept = size(of: partial)
        #expect((1...250_000).contains(kept), "kept \(kept) bytes")

        script.update { $0.dropAfterBytes = nil }
        _ = try await manager.ensureInstalled()

        #expect(modelRequests.last?.header("Range") == "bytes=\(kept)-")
        #expect(try Data(contentsOf: installed) == model)
        #expect(manager.state == .ready(installed))
    }

    @Test func aFullyDownloadedButUnverifiedFileIsVerifiedWithoutDownloadingAgain() async throws {
        try model.write(to: partial)
        try sha256(model).write(to: partialSHA, atomically: true, encoding: .utf8)

        _ = try await manager().ensureInstalled()

        #expect(modelRequests.isEmpty, "no model request expected")
        #expect(try Data(contentsOf: installed) == model)
    }

    @Test func anInstalledModelIsUsedWithoutContactingTheServer() async throws {
        try model.write(to: installed)
        server.shutdown() // offline

        let manager = manager()

        #expect(manager.state == .ready(installed))
        #expect(try await manager.ensureInstalled() == installed)
    }

    @Test func failsWhenTheServerDoesNotOfferTheModel() async {
        let error = await #expect(throws: ModelError.self) { try await manager(modelID: "large").ensureInstalled() }

        #expect(error?.message == "The server does not offer the model large")
    }

    @Test func failsWhenTheServerCannotBeReached() async {
        server.shutdown()

        let error = await #expect(throws: ModelError.self) { try await manager().ensureInstalled() }

        #expect(error?.message.hasPrefix("Could not reach the server") == true)
        #expect(!exists(installed))
    }

    @Test func reportsProgressThenVerifyingThenReady() async throws {
        let manager = manager()
        let watcher = Task {
            var seen: [ModelState] = []
            for await state in Observations({ manager.state }) {
                seen.append(state)
                if case .ready = state { break }
            }
            return seen
        }

        _ = try await manager.ensureInstalled()
        let seen = await watcher.value

        let progress = seen.compactMap { state -> (bytes: Int64, total: Int64)? in
            if case .downloading(let bytes, let total) = state { (bytes, total) } else { nil }
        }
        #expect(!progress.isEmpty, "progress updates: \(seen)")
        #expect(zip(progress, progress.dropFirst()).allSatisfy { $0.bytes <= $1.bytes }, "progress never goes backwards")
        #expect(progress.allSatisfy { $0.total == Int64(model.count) }, "totals are the model size")
        #expect(seen.first == .missing)
        #expect(seen.last == .ready(installed))
    }

    // The model can be downloaded again at any time, so it is not worth a place in the user's iCloud backup.
    @Test func theModelIsKeptOutOfICloudBackups() async throws {
        _ = try await manager().ensureInstalled()

        #expect(try directory.resourceValues(forKeys: [.isExcludedFromBackupKey]).isExcludedFromBackup == true)
    }
}

private func sha256(_ data: Data) -> String {
    SHA256.hash(data: data).map { String(format: "%02x", $0) }.joined()
}

/// What the stub server does; tests change parts of it.
private final class ModelServer: Sendable {
    struct Settings {
        var manifestSHA: String
        var served: Data
        var honourRanges = true
        var dropAfterBytes: Int?
    }

    private let settings: Mutex<Settings>

    init(model: Data) {
        settings = Mutex(Settings(manifestSHA: sha256(model), served: model))
    }

    func update(_ change: (inout Settings) -> Void) {
        settings.withLock { change(&$0) }
    }

    func answer(_ request: StubServer.Request) -> StubServer.Response {
        let settings = settings.withLock { $0 }
        switch request.path {
        case "/v1/models/manifest":
            let model = #"{"id":"base","version":"v1","size":\#(settings.served.count),"sha256":"\#(settings.manifestSHA)","url":"/v1/models/base"}"#
            return .json(status: 200, body: #"{"models":[\#(model)]}"#)
        case "/v1/models/base":
            return serveModel(request, settings)
        default:
            return .status(404)
        }
    }

    private func serveModel(_ request: StubServer.Request, _ settings: Settings) -> StubServer.Response {
        let served = settings.served
        if let limit = settings.dropAfterBytes {
            // Send only the first part of the file, then cut the connection.
            return .dataThenDrop(status: 200, body: served.prefix(limit))
        }
        let offset = request.header("Range").flatMap { Int($0.replacing("bytes=", with: "").replacing("-", with: "")) }
        let sameFile = request.header("If-Range") == "\"\(sha256(served))\""
        if settings.honourRanges, let offset, sameFile {
            return .data(
                status: 206,
                headers: ["Content-Range": "bytes \(offset)-\(served.count - 1)/\(served.count)"],
                body: served.subdata(in: offset..<served.count)
            )
        }
        return .data(status: 200, body: served)
    }
}
