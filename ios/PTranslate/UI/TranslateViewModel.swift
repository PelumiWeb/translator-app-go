import Foundation
import Observation

enum RecordingPhase {
    case idle
    case recording
    case working
}

@MainActor
@Observable
final class TranslateViewModel {
    private(set) var phase = RecordingPhase.idle
    private(set) var status = "Tap Record and speak"
    private(set) var text = ""
    private(set) var error: String?

    /// The work the last tap started. Tests wait on it.
    @ObservationIgnored private(set) var work: Task<Void, Never>?

    @ObservationIgnored private let recorder: any AudioRecorder
    @ObservationIgnored private let pipeline: SpeechTranslationPipeline
    @ObservationIgnored private let requestMicrophone: () async -> Bool
    @ObservationIgnored private let maxRecording: Duration

    @ObservationIgnored private var recording: (any Recording)?
    @ObservationIgnored private var autoStop: Task<Void, Never>?

    init(
        recorder: any AudioRecorder,
        pipeline: SpeechTranslationPipeline,
        requestMicrophone: @escaping () async -> Bool,
        maxRecording: Duration = .seconds(30)
    ) {
        self.recorder = recorder
        self.pipeline = pipeline
        self.requestMicrophone = requestMicrophone
        self.maxRecording = maxRecording
    }

    isolated deinit {
        recording?.cancel() // let go of the microphone if the screen goes away
        autoStop?.cancel()
        work?.cancel()
    }

    /// The one button: starts a recording, or stops it and sends it off.
    func recordTapped() {
        switch phase {
        case .idle: work = Task { await startRecording() }
        case .recording: work = Task { await stopAndTranscribe() }
        case .working: break
        }
    }

    private func startRecording() async {
        guard await requestMicrophone() else {
            error = "Microphone permission is needed to record"
            return
        }
        // A second tap while iOS was asking for permission has already started one.
        guard phase == .idle else { return }

        do {
            recording = try recorder.start()
        } catch {
            self.error = error.localizedDescription
            return
        }
        phase = .recording
        status = "Recording. Tap Stop when done"
        text = ""
        error = nil

        let limit = maxRecording
        autoStop = Task { [weak self] in
            try? await Task.sleep(for: limit)
            guard !Task.isCancelled, let self else { return }
            // Cleared first, so that stopping does not cancel this task, which goes on to run the pipeline.
            autoStop = nil
            await stopAndTranscribe()
        }
    }

    private func stopAndTranscribe() async {
        guard let finished = recording else { return }
        recording = nil
        autoStop?.cancel()
        autoStop = nil
        phase = .working
        status = "Preparing audio"

        let audio = finished.stop()
        guard audio.duration >= Self.minRecording else {
            finish(error: "That was too short. Hold on a little longer")
            return
        }

        do {
            for try await event in pipeline.process(audio, source: .english) {
                show(event)
            }
            finish(error: nil)
        } catch {
            finish(error: error.localizedDescription)
        }
    }

    private func show(_ event: PipelineEvent) {
        switch event {
        case .transcribing(let stage): status = stage.label
        case .partialTranscript(let partial): text = partial
        case .transcribed(let transcript):
            status = "Done"
            text = transcript.text
        }
    }

    private func finish(error: String?) {
        phase = .idle
        if let error {
            status = "Failed"
            self.error = error
        }
    }

    private static let minRecording = Duration.milliseconds(300)
}

private extension TranscriptionStage {
    var label: String {
        switch self {
        case .uploading: "Uploading"
        case .queued: "Queued"
        case .processing: "Transcribing"
        case .reconnecting: "Connection lost. Reconnecting"
        }
    }
}
