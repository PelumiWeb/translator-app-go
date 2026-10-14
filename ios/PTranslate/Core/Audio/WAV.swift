import Foundation

private let headerBytes = 44
private let bytesPerSample = 2

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
        wav.appendLittleEndian(UInt16(1)) // format: PCM
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
