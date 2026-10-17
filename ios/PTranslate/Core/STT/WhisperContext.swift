import Foundation
import whisper

/// A loaded Whisper model. This is the only type that calls whisper.cpp's C functions.
///
/// A context holds the state of one transcription, so calls must not overlap; as an actor it runs them one at a
/// time. The model takes tens of megabytes, freed when the context goes away.
actor WhisperContext {
    struct Result: Equatable, Sendable {
        let text: String

        /// 0 to 1, or nil when the model produced no words.
        let meanTokenProbability: Float?

        /// The model's estimate, 0 to 1, that the audio held no speech at all.
        let noSpeechProbability: Float?
    }

    private let context: OpaquePointer

    // whisper_full blocks its thread for seconds. Running the actor on a queue of its own keeps that off Swift's
    // shared thread pool, which has one thread per core and is meant for work that does not block.
    private let queue = DispatchSerialQueue(label: "whisper")

    nonisolated var unownedExecutor: UnownedSerialExecutor {
        queue.asUnownedSerialExecutor()
    }

    private init(context: OpaquePointer) {
        self.context = context
    }

    // Isolated, so the model is freed on the actor's own queue, never while a call is still using it.
    isolated deinit {
        whisper_free(context)
    }

    /// Loads a model file. It takes a moment, so it runs off the caller's actor.
    ///
    /// - Throws: `TranscriptionError` if the file is missing or is not a Whisper model.
    @concurrent
    static func load(_ model: URL) async throws(TranscriptionError) -> WhisperContext {
        let path = model.path(percentEncoded: false)
        guard FileManager.default.isReadableFile(atPath: path) else {
            throw .failed("No model file at \(path)")
        }
        var params = whisper_context_default_params()
        params.use_gpu = false // built without Metal: see the addendum to ADR 0004
        guard let context = whisper_init_from_file_with_params(path, params) else {
            throw .failed("\(model.lastPathComponent) is not a usable Whisper model")
        }
        return WhisperContext(context: context)
    }

    /// Transcribes 16 kHz mono samples in the range -1 to 1.
    func transcribe(_ samples: [Float], language: Language, threads: Int) throws(TranscriptionError) -> Result {
        var params = whisper_full_default_params(WHISPER_SAMPLING_GREEDY)
        params.n_threads = Int32(threads)
        params.translate = false // keep the spoken language; translating is a later stage
        params.no_context = true // each recording stands alone
        params.no_timestamps = true // not shown anywhere, and skipping them is faster
        // The same choices as Android's whisper_jni.cpp, which explains them: labels such as "[Music]" are allowed,
        // because they are how the model says "this is not speech", and a degenerate result is decoded again at
        // most twice.
        params.temperature_inc = 0.4
        params.print_realtime = false
        params.print_progress = false
        params.print_timestamps = false
        params.print_special = false

        let status = language.tag.withCString { tag in
            params.language = tag
            return samples.withUnsafeBufferPointer { whisper_full(context, params, $0.baseAddress, Int32($0.count)) }
        }
        guard status == 0 else {
            throw .failed("The on-device model failed to run")
        }

        let segments = whisper_full_n_segments(context)
        let text = (0..<segments).map { String(cString: whisper_full_get_segment_text(context, $0)) }.joined()
        return Result(
            text: text.trimmingCharacters(in: .whitespacesAndNewlines),
            meanTokenProbability: meanTokenProbability(),
            noSpeechProbability: segments == 0 ? nil : whisper_full_get_segment_no_speech_prob(context, 0)
        )
    }

    /// The CPU features the library was built to use, such as "NEON = 1".
    static var systemInfo: String {
        String(cString: whisper_print_system_info())
    }

    /// The mean probability the model gave to the words of the last transcript: the confidence score.
    private func meanTokenProbability() -> Float? {
        // Ids from "end of text" upwards are control tokens, not words.
        let firstSpecial = whisper_token_eot(context)
        var sum = 0.0
        var count = 0
        for segment in 0..<whisper_full_n_segments(context) {
            for token in 0..<whisper_full_n_tokens(context, segment) {
                guard whisper_full_get_token_id(context, segment, token) < firstSpecial else { continue }
                sum += Double(whisper_full_get_token_p(context, segment, token))
                count += 1
            }
        }
        return count == 0 ? nil : Float(sum / Double(count))
    }
}
