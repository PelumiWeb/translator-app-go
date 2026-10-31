import Foundation

/// Mono 16-bit PCM audio, the format Whisper and the backend both expect.
struct PCMAudio: Equatable, Sendable {
    /// What Whisper takes, and what the recorder produces.
    static let standardSampleRate = 16_000

    var samples: [Int16]
    var sampleRate: Int

    init(samples: [Int16], sampleRate: Int = PCMAudio.standardSampleRate) {
        self.samples = samples
        self.sampleRate = sampleRate
    }

    var duration: Duration {
        .milliseconds(samples.count * 1000 / sampleRate)
    }

    /// The samples scaled to the range -1 to 1, the form Whisper takes.
    /// How loud each of `count` equal slices of the audio is, from 0 to 1 relative to the loudest slice: the heights of
    /// a waveform drawing.
    func levels(_ count: Int) -> [Float] {
        guard count > 0 else { return [] }
        guard !samples.isEmpty else { return Array(repeating: 0, count: count) }
        let rms: [Double] = (0..<count).map { slice in
            let from = samples.count * slice / count
            let to = min(max(samples.count * (slice + 1) / count, from + 1), samples.count)
            let squares = samples[from..<to].reduce(0.0) { $0 + Double($1) * Double($1) }
            return (squares / Double(to - from)).squareRoot()
        }
        guard let loudest = rms.max(), loudest > 0 else { return Array(repeating: 0, count: count) }
        return rms.map { Float($0 / loudest) }
    }

    var floatSamples: [Float] {
        samples.map { Float($0) / 32768 }
    }

    /// How loud the recording is, in decibels relative to the loudest value a sample can hold: 0 is full scale,
    /// more negative is quieter. Measured as the root mean square of the samples, the usual measure of loudness.
    var levelDB: Double {
        guard !samples.isEmpty else { return Self.minimumLevelDB }
        let sumOfSquares = samples.reduce(0.0) { $0 + Double($1) * Double($1) }
        let rms = (sumOfSquares / Double(samples.count)).squareRoot()
        guard rms > 0 else { return Self.minimumLevelDB }
        return max(20 * log10(rms / 32768), Self.minimumLevelDB)
    }

    /// True when the recording is too quiet to contain speech: a muted or absent microphone, for example.
    var isSilent: Bool {
        levelDB < Self.silenceDB
    }

    // Quiet speech is around -35 dB; the noise floor of a phone microphone in a quiet room is near -60.
    private static let silenceDB = -50.0

    /// Reported for digital silence, where the true value is minus infinity.
    private static let minimumLevelDB = -100.0
}
