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

    /// Where the last transcript came from and how long it took.
    private(set) var details: String?

    private(set) var error: String?
    private(set) var route: TranscriptionRoute

    /// The on-device speech model: missing, downloading, ready or failed.
    var model: ModelState { models.state }

    /// The work the last tap started, and the model download. Tests wait on them.
    @ObservationIgnored private(set) var work: Task<Void, Never>?
    @ObservationIgnored private(set) var download: Task<Void, Never>?

    @ObservationIgnored private let recorder: any AudioRecorder
    @ObservationIgnored private let pipeline: SpeechTranslationPipeline
    @ObservationIgnored private let models: any ModelInstaller
    @ObservationIgnored private let routeSetting: RouteSetting
    @ObservationIgnored private let requestMicrophone: () async -> Bool
    @ObservationIgnored private let maxRecording: Duration
    @ObservationIgnored private let now: () -> Duration

    @ObservationIgnored private var recording: (any Recording)?
    @ObservationIgnored private var autoStop: Task<Void, Never>?

    /// - Parameter now: a clock for timing transcriptions, as time since any fixed point.
    init(
        recorder: any AudioRecorder,
        pipeline: SpeechTranslationPipeline,
        models: any ModelInstaller,
        route: RouteSetting,
        requestMicrophone: @escaping () async -> Bool,
        maxRecording: Duration = .seconds(30),
        now: @escaping () -> Duration = { clockOrigin.duration(to: .now) }
    ) {
        self.recorder = recorder
        self.pipeline = pipeline
        self.models = models
        routeSetting = route
        self.route = route.current
        self.requestMicrophone = requestMicrophone
        self.maxRecording = maxRecording
        self.now = now
    }

    isolated deinit {
        recording?.cancel() // let go of the microphone if the screen goes away
        autoStop?.cancel()
        work?.cancel()
    }

    /// The one button: starts a recording, or stops it and sends it off.
    func recordTapped() {
        switch phase {
        case .idle:
            work = Task { await startRecording() }
        case .recording:
            let startedAt = now()
            work = Task { await stopAndTranscribe(startedAt: startedAt) }
        case .working:
            break
        }
    }

    /// The choice between this device and the server. Not during a recording: the next result must come from
    /// the route that was showing when it was made.
    func routeSelected(_ selected: TranscriptionRoute) {
        guard phase == .idle else { return }
        routeSetting.current = selected
        route = selected
    }

    /// Starts, or after a failure continues, the download of the speech model.
    func downloadModelTapped() {
        switch model {
        case .downloading, .verifying: return
        case .missing, .ready, .failed: break
        }
        download = Task {
            // A failure needs no handling here: the manager's state says what went wrong.
            _ = try? await models.ensureInstalled()
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
        details = nil
        error = nil

        let limit = maxRecording
        autoStop = Task { [weak self] in
            try? await Task.sleep(for: limit)
            guard !Task.isCancelled, let self else { return }
            // Cleared first, so that stopping does not cancel this task, which goes on to run the pipeline.
            autoStop = nil
            await stopAndTranscribe(startedAt: now())
        }
    }

    private func stopAndTranscribe(startedAt: Duration) async {
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
                show(event, audio: audio.duration, startedAt: startedAt)
            }
            finish(error: nil)
        } catch {
            finish(error: error.localizedDescription)
        }
    }

    private func show(_ event: PipelineEvent, audio: Duration, startedAt: Duration) {
        switch event {
        case .transcribing(let stage):
            status = stage.label
        case .partialTranscript(let partial):
            text = partial
        case .transcribed(let transcript):
            status = "Done"
            text = transcript.text
            details = Self.describe(transcript, audio: audio, elapsed: now() - startedAt)
        }
    }

    /// For example "On device, 1.2 s for 4.0 s of audio (0.30x real time), confidence 0.87". The real-time factor
    /// is time taken divided by audio length: below 1 means faster than the speech itself.
    private static func describe(_ transcript: Transcript, audio: Duration, elapsed: Duration) -> String {
        let source = switch transcript.source {
        case .onDevice: "On device"
        case .cloud: "Cloud"
        }
        let seconds = elapsed / .seconds(1)
        let audioSeconds = audio / .seconds(1)
        let timing = String(
            format: "%.1f s for %.1f s of audio (%.2fx real time)",
            seconds, audioSeconds, seconds / audioSeconds
        )
        let confidence = transcript.confidence.map { String(format: ", confidence %.2f", $0) } ?? ""
        return "\(source), \(timing)\(confidence)"
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

/// The fixed point the default clock measures from.
private let clockOrigin = ContinuousClock.now

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
