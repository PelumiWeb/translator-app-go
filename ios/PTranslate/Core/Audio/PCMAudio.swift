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
