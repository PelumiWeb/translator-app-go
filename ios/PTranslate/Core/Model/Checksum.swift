import CryptoKit
import Foundation

enum Checksum {
    /// The SHA-256 of a file as lowercase hex. The file is read in chunks, so its size does not matter for memory.
    /// Blocks; call it off the main actor.
    static func sha256(of file: URL) throws -> String {
        let handle = try FileHandle(forReadingFrom: file)
        defer { try? handle.close() }
        var hasher = SHA256()
        while let chunk = try handle.read(upToCount: 64 * 1024), !chunk.isEmpty {
            hasher.update(data: chunk)
        }
        return hasher.finalize().map { String(format: "%02x", $0) }.joined()
    }
}
