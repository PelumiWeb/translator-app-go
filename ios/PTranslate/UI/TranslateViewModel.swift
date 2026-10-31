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

    /// What was said, in the source language.
    private(set) var text = ""

    /// The same, in the target language. Empty until translated.
    private(set) var translation = ""

    /// Where the last transcript came from and how long it took.
    private(set) var details: String?

    /// Why the result came from where it did, when that needs saying.
    private(set) var routing: String?

    private(set) var error: String?
    private(set) var route: TranscriptionRoute

    private(set) var source = Language.english
    private(set) var target = Language.spanish

    /// Empty until `loadLanguages` has run.
    private(set) var sourceLanguages: [Language] = []
    private(set) var targetLanguages: [Language] = []

    /// The on-device speech model: missing, downloading, ready or failed.
    var model: ModelState { models.state }

    /// This phone's measured speed as a real-time factor, or nil if it has not been measured.
    var deviceSpeed: Double? { speed.realTimeFactor }

    var measuringSpeed: Bool { speed.measuring }

    /// True when `deviceSpeed` is over the limit the automatic route allows.
    var deviceTooSlow: Bool {
        deviceSpeed.map { $0 > thresholds.maxRealTimeFactor } ?? false
    }

    /// The work the last tap started, the model download, and the speed measurement. Tests wait on them.
    @ObservationIgnored private(set) var work: Task<Void, Never>?
    @ObservationIgnored private(set) var download: Task<Void, Never>?
    @ObservationIgnored private(set) var measurement: Task<Void, Never>?

    @ObservationIgnored private let recorder: any AudioRecorder
    @ObservationIgnored private let pipeline: SpeechTranslationPipeline
    @ObservationIgnored private let translator: any Translator
    @ObservationIgnored private let models: any ModelInstaller
    @ObservationIgnored private let routeSetting: RouteSetting
    @ObservationIgnored private let speed: any DeviceSpeed
    @ObservationIgnored private let thresholds: FallbackThresholds
    @ObservationIgnored private let requestMicrophone: () async -> Bool
    @ObservationIgnored private let maxRecording: Duration
    @ObservationIgnored private let now: () -> Duration

    @ObservationIgnored private var recording: (any Recording)?
    @ObservationIgnored private var autoStop: Task<Void, Never>?

    /// - Parameter now: a clock for timing transcriptions, as time since any fixed point.
    init(
        recorder: any AudioRecorder,
        pipeline: SpeechTranslationPipeline,
        translator: any Translator,
        models: any ModelInstaller,
        route: RouteSetting,
        speed: any DeviceSpeed,
        thresholds: FallbackThresholds = FallbackThresholds(),
        requestMicrophone: @escaping () async -> Bool,
        maxRecording: Duration = .seconds(30),
        now: @escaping () -> Duration = { clockOrigin.duration(to: .now) }
    ) {
        self.recorder = recorder
        self.pipeline = pipeline
        self.translator = translator
        self.models = models
        routeSetting = route
        self.route = route.current
        self.speed = speed
        self.thresholds = thresholds
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
            work = Task { await stopAndProcess(startedAt: startedAt) }
        case .working:
            break
        }
    }

    /// The choice of route: automatic, this device, or the server. Not during a recording: the next result must
    /// come from the route that was showing when it was made.
    func routeSelected(_ selected: TranscriptionRoute) {
        guard phase == .idle else { return }
        routeSetting.current = selected
        route = selected
    }

    /// The languages can be changed only between recordings: a run uses the ones showing when it started.
    func sourceSelected(_ language: Language) {
        guard phase == .idle else { return }
        source = language
    }

    func targetSelected(_ language: Language) {
        guard phase == .idle else { return }
        target = language
    }

    /// Fills the "From" and "To" lists. A language can be spoken only if Whisper transcribes it and the translator
    /// translates from it.
    func loadLanguages() async {
        let all = await translator.supportedLanguages
        sourceLanguages = all.filter(WhisperLanguages.supports)
        targetLanguages = all
    }

    func measureSpeedTapped() {
        guard !measuringSpeed else { return }
        measurement = Task { await speed.measureAgain() }
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
        // A new recording clears the previous result but keeps the settings.
        text = ""
        translation = ""
        details = nil
        routing = nil
        error = nil

        let limit = maxRecording
        autoStop = Task { [weak self] in
            try? await Task.sleep(for: limit)
            guard !Task.isCancelled, let self else { return }
            // Cleared first, so that stopping does not cancel this task, which goes on to run the pipeline.
            autoStop = nil
            await stopAndProcess(startedAt: now())
        }
    }

    private func stopAndProcess(startedAt: Duration) async {
        guard let finished = recording else { return }
        recording = nil
        autoStop?.cancel()
        autoStop = nil
        phase = .working
        status = "Preparing audio"
        // Read once, so the whole run uses the languages chosen at this moment.
        let (source, target) = (source, target)

        let audio = finished.stop()
        guard audio.duration >= Self.minRecording else {
            finish(error: "That was too short. Hold on a little longer")
            return
        }

        do {
            for try await event in pipeline.process(audio, source: source, target: target) {
                show(event, audio: audio.duration, startedAt: startedAt, translating: source != target)
            }
            finish(error: nil)
        } catch {
            finish(error: error.localizedDescription)
        }
    }

    private func show(_ event: PipelineEvent, audio: Duration, startedAt: Duration, translating: Bool) {
        switch event {
        case .transcribing(let stage):
            status = stage.label
        case .partialTranscript(let partial):
            text = partial
        case .transcribed(let transcript):
            status = "Transcribed"
            text = transcript.text
            details = Self.describe(transcript, audio: audio, elapsed: now() - startedAt)
            routing = transcript.note?.explanation
        case .downloadingLanguages:
            status = "Downloading language pack"
        case .translating:
            status = "Translating"
        case .translated(let translated):
            status = "Done"
            // With the same language on both sides there is nothing to show twice.
            translation = translating ? translated : ""
        case .speaking:
            status = "Speaking"
        case .spoken:
            status = "Done"
        case .speechFailed(let reason):
            // The translation is on screen; only the voice is missing.
            status = "Done"
            error = reason
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
        // With no new error, one already on screen stays: a result can be complete and still carry a note, such
        // as "could not be spoken".
        if let error {
            status = "Failed"
            self.error = error
        }
    }

    private static let minRecording = Duration.milliseconds(300)
}

/// The fixed point the default clock measures from.
private let clockOrigin = ContinuousClock.now

private extension RoutingNote {
    var explanation: String {
        switch self {
        case .noModel: "Sent to the server: there is no speech model on this device"
        case .slowDevice: "Sent to the server: this device is too slow for the model"
        case .lowConfidence: "Sent to the server: this device was not confident in its own result"
        case .noSpeechOnDevice: "Sent to the server: this device heard sound but found no words"
        case .garbledOnDevice: "Sent to the server: this device could not make out the speech"
        case .onDeviceFailed: "Sent to the server: the model on this device failed"
        case .cloudUnavailable: "This device's result was kept: the server could not be reached"
        }
    }
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
