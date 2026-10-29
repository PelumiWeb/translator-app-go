import Foundation
import Testing
@testable import PTranslate

struct ChecksumTests {
    private func file(with content: Data) throws -> URL {
        let file = FileManager.default.temporaryDirectory.appending(path: "checksum-\(UUID().uuidString)")
        try content.write(to: file)
        return file
    }

    // The standard test vectors published with SHA-256.
    @Test func matchesThePublishedTestVectors() throws {
        #expect(try Checksum.sha256(of: file(with: Data())) == "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855")
        #expect(try Checksum.sha256(of: file(with: Data("abc".utf8))) == "ba7816bf8f01cfea414140de5dae2223b00361a396177a9cb410ff61f20015ad")
    }

    @Test func hashesAFileLargerThanItsReadBuffer() throws {
        // One million "a"s, another published vector, read in 64 KB chunks.
        let file = try file(with: Data(repeating: UInt8(ascii: "a"), count: 1_000_000))

        #expect(try Checksum.sha256(of: file) == "cdc76e5c9914fb9281a1c7e284d73e67f1809a48a497200e046d39ccc7112cd0")
    }

    @Test func oneChangedByteChangesTheHash() throws {
        let original = Data((0..<100_000).map { UInt8($0 % 251) })
        var corrupted = original
        corrupted[50_000] &+= 1

        #expect(try Checksum.sha256(of: file(with: original)) != Checksum.sha256(of: file(with: corrupted)))
    }
}
