import Foundation

/// Turns speech into text. The app depends on this protocol only; whether the work happens on the phone or on the
/// backend is an implementation detail.
protocol Transcriber: Sendable {
    /// Starts at once. The stream ends after one `.final`, or fails with `TranscriptionError`. Stopping reading it
    /// cancels the work.
    func transcribe(_ audio: PCMAudio, language: Language) -> AsyncThrowingStream<TranscriptEvent, any Error>
}

enum TranscriptEvent: Equatable, Sendable {
    case stageChanged(TranscriptionStage)

    /// The whole text recognised so far, not just the new words.
    case partial(String)

    case final(Transcript)
}

enum TranscriptionStage: Sendable {
    case uploading
    case queued
    case processing
    case reconnecting
}

struct Transcript: Equatable, Sendable {
    enum Source: Sendable {
        case onDevice
        case cloud
    }

    var text: String

    /// 0 to 1, or nil when the source cannot provide one.
    var confidence: Float?

    var source: Source
}

/// Why a transcription did not produce a transcript. Each case carries the reason in words for the user.
enum TranscriptionError: LocalizedError, Equatable {
    case failed(String)

    /// Nothing was recorded: a muted or missing microphone. No transcriber can do better.
    case silentAudio(String)

    /// There was sound, but this transcriber found no words in it. Another one might.
    case noSpeech(String)

    /// The transcriber produced text, but it is not a transcript: the same phrase over and over. Showing,
    /// translating or speaking it would be worse than saying nothing. Another transcriber might do better.
    case garbled(String)

    var errorDescription: String? {
        switch self {
        case .failed(let message), .silentAudio(let message), .noSpeech(let message), .garbled(let message): message
        }
    }
}
