import Foundation
import Observation

enum ModelState: Equatable, Sendable {
    /// Not on this phone, and no download running.
    case missing
    case downloading(bytes: Int64, total: Int64)
    case verifying
    case ready(URL)
    case failed(String)
}

struct ModelError: LocalizedError, Equatable {
    let message: String

    var errorDescription: String? { message }
}

/// What the rest of the app needs from the model manager.
@MainActor
protocol ModelInstaller: AnyObject {
    /// Observable: a view that reads it is redrawn when it changes.
    var state: ModelState { get }

    /// Returns the installed model, downloading it first if needed. When it fails, `state` holds `.failed` with
    /// the same reason.
    func ensureInstalled() async throws(ModelError) -> URL
}

extension ModelInstaller {
    /// The installed model, or nil while there is none.
    var installedModel: URL? {
        if case .ready(let file) = state { file } else { nil }
    }
}

/// Gets one model from the backend onto the phone and keeps it there.
///
/// Files in `directory`, for a model with id "base":
/// - `base.bin`: the installed model. It only ever appears by being renamed from a fully downloaded,
///   checksum-verified file, so its presence alone means "safe to load".
/// - `base.part`: a download in progress, kept across failures and restarts so the next attempt continues from
///   where this one stopped.
/// - `base.part.sha256`: the checksum the partial file belongs to. If the server's model has changed since, the
///   partial file is thrown away.
///
/// On the main actor because its state drives the screen; the network and file work runs off it.
@MainActor
@Observable
final class ModelManager: ModelInstaller {
    private(set) var state: ModelState

    @ObservationIgnored private let backend: BackendClient
    @ObservationIgnored private let modelID: String
    @ObservationIgnored private let files: ModelFiles

    // Shared by everyone who asks while a download runs, so there is only ever one, writing to one file.
    @ObservationIgnored private var running: Task<URL, any Error>?

    init(backend: BackendClient, directory: URL, modelID: String) {
        self.backend = backend
        self.modelID = modelID
        files = ModelFiles(directory: directory, modelID: modelID)
        state = files.size(of: files.installed) != nil ? .ready(files.installed) : .missing
    }

    /// Safe to call again after a failure: it continues the partial download. An installed model is returned
    /// without contacting the server, so the app works offline once it has one.
    func ensureInstalled() async throws(ModelError) -> URL {
        let task = running ?? Task { try await install() }
        running = task
        defer { running = nil }
        do {
            return try await task.value
        } catch {
            throw error as? ModelError ?? ModelError(message: "The model could not be installed")
        }
    }

    private func install() async throws(ModelError) -> URL {
        if files.size(of: files.installed) != nil {
            state = .ready(files.installed)
            return files.installed
        }

        do {
            guard let model = try await backend.modelManifest().first(where: { $0.id == modelID }) else {
                throw ModelError(message: "The server does not offer the model \(modelID)")
            }

            // The download runs off the main actor and reports its progress here, where the state lives.
            let (progress, reporter) = AsyncStream.makeStream(of: Int64.self, bufferingPolicy: .bufferingNewest(1))
            async let downloaded: Void = Self.download(model, to: files, using: backend, progress: reporter)
            for await bytes in progress {
                state = .downloading(bytes: bytes, total: model.size)
            }
            try await downloaded

            state = .verifying
            try await Self.verify(model, files)
            try files.install()
            state = .ready(files.installed)
            return files.installed
        } catch let error as ModelError {
            throw fail(error.message)
        } catch let error as BackendError {
            throw fail(error.message)
        } catch {
            // A dropped connection mid-download lands here. The partial file stays, so the next call resumes.
            throw fail("The download was interrupted")
        }
    }

    private func fail(_ reason: String) -> ModelError {
        state = .failed(reason)
        return ModelError(message: reason)
    }

    @concurrent
    private static func download(
        _ model: RemoteModel,
        to files: ModelFiles,
        using backend: BackendClient,
        progress: AsyncStream<Int64>.Continuation
    ) async throws {
        defer { progress.finish() }
        try files.prepareDirectory()

        // A partial file is only worth keeping if it is the start of this exact model.
        let recorded = try? String(contentsOf: files.partialSHA, encoding: .utf8)
        if recorded != model.sha256 || (files.size(of: files.partial) ?? 0) > model.size {
            try? FileManager.default.removeItem(at: files.partial)
        }
        try model.sha256.write(to: files.partialSHA, atomically: true, encoding: .utf8)

        let have = files.size(of: files.partial) ?? 0
        if have == model.size {
            return // only verification was left
        }

        let download = try await backend.openModel(model, fromByte: have)
        // The server may ignore the offset and send everything (a plain 200). Appending that to what is here would
        // corrupt the file, so in that case start again from an empty one.
        if !download.partial || files.size(of: files.partial) == nil {
            FileManager.default.createFile(atPath: files.partial.path(percentEncoded: false), contents: nil)
        }
        var written = download.partial ? have : 0
        progress.yield(written)

        let handle = try FileHandle(forWritingTo: files.partial)
        defer { try? handle.close() }
        try handle.seekToEnd()

        var chunk: [UInt8] = []
        chunk.reserveCapacity(chunkBytes)
        var lastReported = written
        for try await byte in download.bytes {
            chunk.append(byte)
            guard chunk.count == chunkBytes else { continue }
            try handle.write(contentsOf: chunk)
            written += Int64(chunk.count)
            chunk.removeAll(keepingCapacity: true)
            // Often enough for a smooth progress bar, not on every chunk.
            if written - lastReported >= progressStepBytes {
                progress.yield(written)
                lastReported = written
            }
        }
        try handle.write(contentsOf: chunk)
        progress.yield(written + Int64(chunk.count))
    }

    @concurrent
    private static func verify(_ model: RemoteModel, _ files: ModelFiles) async throws {
        guard try Checksum.sha256(of: files.partial) == model.sha256 else {
            // Nothing in a corrupt file can be trusted, so none of it is kept for a resume.
            try? FileManager.default.removeItem(at: files.partial)
            try? FileManager.default.removeItem(at: files.partialSHA)
            throw ModelError(message: "The downloaded model is corrupt (checksum mismatch)")
        }
    }
}

private let chunkBytes = 64 * 1024
private let progressStepBytes: Int64 = 256 * 1024

/// Where the manager keeps its files.
private struct ModelFiles: Sendable {
    let directory: URL
    let installed: URL
    let partial: URL
    let partialSHA: URL

    init(directory: URL, modelID: String) {
        self.directory = directory
        installed = directory.appending(path: "\(modelID).bin")
        partial = directory.appending(path: "\(modelID).part")
        partialSHA = directory.appending(path: "\(modelID).part.sha256")
    }

    /// The file's size, or nil if there is no such file. Read from the file system each time: a URL's own
    /// resource values are cached, and these files change under it.
    func size(of file: URL) -> Int64? {
        let attributes = try? FileManager.default.attributesOfItem(atPath: file.path(percentEncoded: false))
        return (attributes?[.size] as? NSNumber)?.int64Value
    }

    func prepareDirectory() throws {
        try FileManager.default.createDirectory(at: directory, withIntermediateDirectories: true)
        // The model can always be downloaded again, so it has no place in the user's iCloud backup.
        var excluded = URLResourceValues()
        excluded.isExcludedFromBackup = true
        var directory = directory
        try directory.setResourceValues(excluded)
    }

    /// A rename within one directory is atomic: other code sees either no model or the complete one, never a
    /// half-written file.
    func install() throws(ModelError) {
        do {
            try FileManager.default.moveItem(at: partial, to: installed)
        } catch {
            throw ModelError(message: "Could not save the model")
        }
        try? FileManager.default.removeItem(at: partialSHA)
    }
}
