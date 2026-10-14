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

enum TranscriptionError: LocalizedError, Equatable {
    /// The transcription could not be done; the message says why, in words for the user.
    case failed(String)

    var errorDescription: String? {
        switch self {
        case .failed(let message): message
        }
    }
}
