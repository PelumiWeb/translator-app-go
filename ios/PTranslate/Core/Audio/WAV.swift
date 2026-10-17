import Foundation

private let headerBytes = 44
private let bytesPerSample = 2
private let formatPCM: UInt16 = 1

struct WAVError: LocalizedError, Equatable {
    let message: String

    var errorDescription: String? { message }
}

extension PCMAudio {
    /// Wraps the samples in a WAV container: a 44-byte header followed by the raw samples. WAV needs no encoder,
    /// and a 30 second clip is under 1 MB.
    func wavData() -> Data {
        let dataBytes = samples.count * bytesPerSample
        var wav = Data(capacity: headerBytes + dataBytes)

        wav.appendASCII("RIFF")
        wav.appendLittleEndian(UInt32(headerBytes - 8 + dataBytes)) // size of everything after this field
        wav.appendASCII("WAVE")

        wav.appendASCII("fmt ")
        wav.appendLittleEndian(UInt32(16)) // size of the fmt chunk
        wav.appendLittleEndian(formatPCM)
        wav.appendLittleEndian(UInt16(1)) // channels
        wav.appendLittleEndian(UInt32(sampleRate))
        wav.appendLittleEndian(UInt32(sampleRate * bytesPerSample)) // bytes per second
        wav.appendLittleEndian(UInt16(bytesPerSample)) // bytes per frame
        wav.appendLittleEndian(UInt16(bytesPerSample * 8)) // bits per sample

        wav.appendASCII("data")
        wav.appendLittleEndian(UInt32(dataBytes))
        samples.map(\.littleEndian).withUnsafeBytes { wav.append(contentsOf: $0) }

        return wav
    }

    /// Reads a WAV file that holds 16-bit mono PCM, the only kind this app writes or bundles.
    ///
    /// A WAV file is a list of chunks, each with a four-letter name and a length. The samples are in the "data"
    /// chunk, which need not come first: files often carry a "LIST" chunk of metadata before it. So the chunks
    /// are walked, not assumed to sit at fixed offsets.
    init(wav: Data) throws(WAVError) {
        let bytes = [UInt8](wav)
        guard bytes.count >= 12, bytes.ascii(at: 0) == "RIFF", bytes.ascii(at: 8) == "WAVE" else {
            throw WAVError(message: "Not a WAV file")
        }

        var sampleRate: Int?
        var offset = 12
        while offset + 8 <= bytes.count {
            let name = bytes.ascii(at: offset)
            let size = Int(bytes.uint32(at: offset + 4))
            let body = offset + 8
            guard body + size <= bytes.count else { throw WAVError(message: "The WAV file is cut short") }

            switch name {
            case "fmt ":
                guard size >= 16 else { throw WAVError(message: "The WAV format chunk is too short") }
                let format = bytes.uint16(at: body)
                let channels = bytes.uint16(at: body + 2)
                let bits = bytes.uint16(at: body + 14)
                guard format == formatPCM, channels == 1, bits == 16 else {
                    throw WAVError(
                        message: "Only 16-bit mono PCM is supported (got format \(format), \(channels) channels, \(bits) bits)"
                    )
                }
                sampleRate = Int(bytes.uint32(at: body + 4))

            case "data":
                guard let sampleRate else {
                    throw WAVError(message: "The WAV file has no format chunk before its data")
                }
                let samples = (0..<size / bytesPerSample).map { Int16(bitPattern: bytes.uint16(at: body + $0 * 2)) }
                self.init(samples: samples, sampleRate: sampleRate)
                return

            default:
                break
            }
            offset = body + size + (size & 1) // chunks are padded to an even length
        }
        throw WAVError(message: "The WAV file has no data chunk")
    }
}

// Every number in a WAV file is little-endian.
private extension Data {
    mutating func appendASCII(_ text: String) {
        append(contentsOf: Array(text.utf8))
    }

    mutating func appendLittleEndian<T: FixedWidthInteger>(_ value: T) {
        Swift.withUnsafeBytes(of: value.littleEndian) { append(contentsOf: $0) }
    }
}

private extension [UInt8] {
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
