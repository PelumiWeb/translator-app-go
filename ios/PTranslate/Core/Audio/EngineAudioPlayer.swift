import AVFoundation

/// Plays PCM audio through the phone's speaker with `AVAudioEngine`.
final class EngineAudioPlayer: AudioPlayer {
    private var current: AVAudioPlayerNode?

    func play(_ audio: PCMAudio) async throws {
        stop()
        guard !audio.samples.isEmpty else { return }

        guard
            let format = AVAudioFormat(standardFormatWithSampleRate: Double(audio.sampleRate), channels: 1),
            let buffer = AVAudioPCMBuffer(pcmFormat: format, frameCapacity: AVAudioFrameCount(audio.samples.count)),
            let channel = buffer.floatChannelData?[0]
        else {
            throw PlayerError(message: "The recording could not be prepared for playback")
        }
        buffer.frameLength = buffer.frameCapacity
        for (index, sample) in audio.samples.enumerated() {
            channel[index] = Float(sample) / 32768
        }

        let session = AVAudioSession.sharedInstance()
        let engine = AVAudioEngine()
        let node = AVAudioPlayerNode()
        engine.attach(node)
        // The mixer converts the clip's 16 kHz to whatever rate the output runs at.
        engine.connect(node, to: engine.mainMixerNode, format: format)
        do {
            // The recorder leaves the session set up for recording, which plays nothing.
            try session.setCategory(.playback, mode: .spokenAudio)
            try session.setActive(true)
            try engine.start()
        } catch {
            try? session.setActive(false)
            throw PlayerError(message: "Could not play the recording: \(error.localizedDescription)")
        }

        current = node
        await withCheckedContinuation { continuation in
            // Called once the clip has been heard to the end, or as soon as stop() stops the node.
            node.scheduleBuffer(buffer, completionCallbackType: .dataPlayedBack) { _ in
                continuation.resume()
            }
            node.play()
        }

        node.stop()
        engine.stop()
        if current === node {
            current = nil
        }
        try? session.setActive(false, options: .notifyOthersOnDeactivation)
    }

    func stop() {
        current?.stop()
    }
}

struct PlayerError: LocalizedError {
    let message: String

    var errorDescription: String? { message }
}
