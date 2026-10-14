import Foundation
import Testing
@testable import PTranslate

struct WAVTests {
    @Test func headerDescribes16kHzMono16BitPCM() {
        let wav = PCMAudio(samples: [1, -2, 300]).wavData()

        #expect(wav.count == 44 + 6)
        #expect(wav.ascii(at: 0) == "RIFF")
        #expect(wav.uint32(at: 4) == 36 + 6)
        #expect(wav.ascii(at: 8) == "WAVE")
        #expect(wav.ascii(at: 12) == "fmt ")
        #expect(wav.uint32(at: 16) == 16)
        #expect(wav.uint16(at: 20) == 1, "format: PCM")
        #expect(wav.uint16(at: 22) == 1, "channels")
        #expect(wav.uint32(at: 24) == 16_000, "sample rate")
        #expect(wav.uint32(at: 28) == 32_000, "bytes per second")
        #expect(wav.uint16(at: 32) == 2, "bytes per frame")
        #expect(wav.uint16(at: 34) == 16, "bits per sample")
        #expect(wav.ascii(at: 36) == "data")
        #expect(wav.uint32(at: 40) == 6)
    }

    @Test func samplesFollowTheHeaderInLittleEndianOrder() {
        let wav = PCMAudio(samples: [1, -2, 300]).wavData()

        #expect(Int16(bitPattern: wav.uint16(at: 44)) == 1)
        #expect(Int16(bitPattern: wav.uint16(at: 46)) == -2)
        #expect(Int16(bitPattern: wav.uint16(at: 48)) == 300)
        // 300 is 0x012C: low byte first.
        #expect(wav[48] == 0x2C)
        #expect(wav[49] == 0x01)
    }

    @Test func emptyAudioIsAHeaderWithNoData() {
        let wav = PCMAudio(samples: []).wavData()

        #expect(wav.count == 44)
        #expect(wav.uint32(at: 40) == 0)
    }

    @Test func durationIsDerivedFromTheSampleCount() {
        #expect(PCMAudio(samples: Array(repeating: 0, count: 24_000)).duration == .milliseconds(1_500))
    }
}

private extension Data {
    func ascii(at offset: Int) -> String {
        String(decoding: self[offset..<offset + 4], as: UTF8.self)
    }

    func uint16(at offset: Int) -> UInt16 {
        UInt16(self[offset]) | UInt16(self[offset + 1]) << 8
    }

    func uint32(at offset: Int) -> UInt32 {
        (0..<4).reduce(0) { $0 | UInt32(self[offset + $1]) << (8 * $1) }
    }
}
