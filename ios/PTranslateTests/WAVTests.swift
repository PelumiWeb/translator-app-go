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

    @Test func quietRecordingsCountAsSilentAndSpeechLevelOnesDoNot() {
        #expect(PCMAudio(samples: []).isSilent, "empty")
        #expect(PCMAudio(samples: Array(repeating: 0, count: 16_000)).isSilent, "digital silence")
        #expect(PCMAudio(samples: alternating(40)).isSilent, "faint hiss")
        #expect(!PCMAudio(samples: alternating(600)).isSilent, "quiet speech level")
        // One loud click in a second of silence is not speech either.
        var click = Array(repeating: Int16(0), count: 16_000)
        click[100] = 8_000
        #expect(PCMAudio(samples: click).isSilent, "single click")
    }

    @Test func levelIsInDecibelsBelowFullScale() {
        #expect(PCMAudio(samples: Array(repeating: 0, count: 16_000)).levelDB == -100, "digital silence")
        #expect(PCMAudio(samples: []).levelDB == -100, "empty")
        // A constant 3277 is a tenth of full scale, which is -20 dB.
        #expect(abs(PCMAudio(samples: Array(repeating: 3277, count: 16_000)).levelDB + 20) < 0.01)
        #expect(abs(PCMAudio(samples: Array(repeating: 328, count: 16_000)).levelDB + 40) < 0.02)
    }

    @Test func aDecodedWAVHasTheSamplesItWasWrittenWith() throws {
        let original = PCMAudio(samples: [0, 1, -1, 300, .max, .min])

        let decoded = try PCMAudio(wav: original.wavData())

        #expect(decoded.samples == original.samples)
        #expect(decoded.sampleRate == 16_000)
    }

    // Real files often carry a metadata chunk before the samples, so the samples do not start at byte 44. The
    // bundled benchmark clip is one.
    @Test func findsTheSamplesAfterAMetadataChunk() throws {
        let plain = PCMAudio(samples: [10, 20, 30]).wavData()
        let list = Data("LIST".utf8) + Data([5, 0, 0, 0]) + Data("INFOx".utf8) + Data([0]) // 5 bytes, padded to 6
        let withMetadata = plain[0..<36] + list + plain[36...]

        #expect(try PCMAudio(wav: withMetadata).samples == [10, 20, 30])
    }

    @Test func rejectsFilesThatAreNot16BitMonoPCM() {
        var stereo = PCMAudio(samples: [1, 2]).wavData()
        stereo[22] = 2
        let truncated = PCMAudio(samples: Array(repeating: 0, count: 100)).wavData().prefix(60)

        #expect(throws: WAVError.self) { try PCMAudio(wav: Data("this is not audio".utf8)) }
        #expect(throws: WAVError.self) { try PCMAudio(wav: stereo) }
        #expect(throws: WAVError.self) { try PCMAudio(wav: Data(truncated)) }
        #expect(throws: WAVError.self) { try PCMAudio(wav: Data()) }
    }

    /// A second of a square wave at the given amplitude.
    private func alternating(_ amplitude: Int16) -> [Int16] {
        (0..<16_000).map { $0 % 2 == 0 ? amplitude : -amplitude }
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
