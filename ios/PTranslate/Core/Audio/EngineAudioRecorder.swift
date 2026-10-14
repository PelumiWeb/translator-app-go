import AVFoundation

/// Records 16 kHz mono PCM with `AVAudioEngine`.
///
/// The hardware records at its own rate, usually 48 kHz, and cannot be asked for 16 kHz, so each buffer is
/// converted as it arrives. Nothing past `maxDuration`, one Whisper window, is kept.
final class EngineAudioRecorder: AudioRecorder {
    private let maxDuration: Duration

    init(maxDuration: Duration = .seconds(30)) {
        self.maxDuration = maxDuration
    }

    func start() throws -> any Recording {
        let session = AVAudioSession.sharedInstance()
        do {
            // .measurement turns off the processing meant for calls, such as automatic gain, which helps a
            // listener but hurts transcription. The same reason Android records with VOICE_RECOGNITION.
            try session.setCategory(.record, mode: .measurement)
            try session.setActive(true)
        } catch {
            throw RecorderError(message: "Could not open the microphone: \(error.localizedDescription)")
        }

        let engine = AVAudioEngine()
        let input = engine.inputNode
        let hardwareFormat = input.outputFormat(forBus: 0)
        guard hardwareFormat.sampleRate > 0, hardwareFormat.channelCount > 0 else {
            try? session.setActive(false)
            throw RecorderError(message: "This device has no microphone to record from")
        }

        let maxSamples = Int(maxDuration / .seconds(1) * Double(PCMAudio.standardSampleRate))
        let capture = try Capture(from: hardwareFormat, maxSamples: maxSamples)
        // Marked @Sendable so the closure has no actor of its own: the engine calls it on its audio thread.
        input.installTap(onBus: 0, bufferSize: 4096, format: hardwareFormat) { @Sendable buffer, _ in
            capture.append(buffer)
        }

        engine.prepare()
        do {
            try engine.start()
        } catch {
            input.removeTap(onBus: 0)
            try? session.setActive(false)
            throw RecorderError(message: "Could not start recording: \(error.localizedDescription)")
        }
        return EngineRecording(engine: engine, capture: capture)
    }
}

private final class EngineRecording: Recording {
    private let engine: AVAudioEngine
    private let capture: Capture

    init(engine: AVAudioEngine, capture: Capture) {
        self.engine = engine
        self.capture = capture
    }

    func stop() -> PCMAudio {
        end()
        return PCMAudio(samples: capture.samples)
    }

    func cancel() {
        end()
    }

    private func end() {
        engine.inputNode.removeTap(onBus: 0)
        engine.stop()
        // Lets audio from other apps that the recording interrupted, such as music, carry on.
        try? AVAudioSession.sharedInstance().setActive(false, options: .notifyOthersOnDeactivation)
    }
}

/// Converts the engine's buffers to 16 kHz mono 16-bit and keeps the result. The engine calls `append` on its
/// audio thread while `samples` is read from another, so all state is behind one lock; that is what makes the
/// unchecked Sendable true.
private final class Capture: @unchecked Sendable {
    private let converter: AVAudioConverter
    private let outputFormat: AVAudioFormat
    private let maxSamples: Int
    private let lock = NSLock()
    private var collected: [Int16] = []

    init(from inputFormat: AVAudioFormat, maxSamples: Int) throws {
        guard
            let outputFormat = AVAudioFormat(
                commonFormat: .pcmFormatInt16,
                sampleRate: Double(PCMAudio.standardSampleRate),
                channels: 1,
                interleaved: true
            ),
            let converter = AVAudioConverter(from: inputFormat, to: outputFormat)
        else {
            throw RecorderError(message: "This device cannot record 16 kHz mono audio")
        }
        converter.downmix = true // a stereo microphone becomes one channel
        self.converter = converter
        self.outputFormat = outputFormat
        self.maxSamples = maxSamples
        collected.reserveCapacity(maxSamples)
    }

    var samples: [Int16] {
        lock.withLock { collected }
    }

    func append(_ buffer: AVAudioPCMBuffer) {
        lock.withLock {
            guard collected.count < maxSamples else { return }

            let ratio = outputFormat.sampleRate / buffer.format.sampleRate
            let capacity = AVAudioFrameCount((Double(buffer.frameLength) * ratio).rounded(.up)) + 32
            guard let converted = AVAudioPCMBuffer(pcmFormat: outputFormat, frameCapacity: capacity) else { return }

            // The converter pulls its input through this block, synchronously, inside convert(): it is handed
            // the one buffer, then told there is no more for now. Nothing here crosses a thread, which is why the
            // unsafe markings are sound.
            nonisolated(unsafe) var delivered = false
            nonisolated(unsafe) let input = buffer
            var error: NSError?
            converter.convert(to: converted, error: &error) { _, status in
                if delivered {
                    status.pointee = .noDataNow
                    return nil
                }
                delivered = true
                status.pointee = .haveData
                return input
            }
            guard error == nil, let channel = converted.int16ChannelData else { return }

            let count = min(Int(converted.frameLength), maxSamples - collected.count)
            collected.append(contentsOf: UnsafeBufferPointer(start: channel[0], count: count))
        }
    }
}
