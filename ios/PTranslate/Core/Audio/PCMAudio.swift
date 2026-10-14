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
}
