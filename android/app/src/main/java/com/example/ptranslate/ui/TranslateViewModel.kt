package com.example.ptranslate.ui

import androidx.lifecycle.ViewModel
import androidx.lifecycle.ViewModelProvider
import androidx.lifecycle.ViewModelProvider.AndroidViewModelFactory.Companion.APPLICATION_KEY
import androidx.lifecycle.viewModelScope
import androidx.lifecycle.viewmodel.initializer
import androidx.lifecycle.viewmodel.viewModelFactory
import com.example.ptranslate.PtranslateApp
import com.example.ptranslate.core.Language
import com.example.ptranslate.core.audio.AudioRecorder
import com.example.ptranslate.core.audio.Recording
import com.example.ptranslate.core.model.ModelException
import com.example.ptranslate.core.model.ModelInstaller
import com.example.ptranslate.core.model.ModelState
import com.example.ptranslate.core.pipeline.PipelineEvent
import com.example.ptranslate.core.pipeline.SpeechTranslationPipeline
import com.example.ptranslate.core.stt.DeviceSpeed
import com.example.ptranslate.core.stt.FallbackThresholds
import com.example.ptranslate.core.stt.RoutingNote
import com.example.ptranslate.core.stt.Transcript
import com.example.ptranslate.core.stt.TranscriptionRoute
import com.example.ptranslate.core.stt.TranscriptionStage
import kotlinx.coroutines.Job
import kotlinx.coroutines.delay
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.asStateFlow
import kotlinx.coroutines.flow.catch
import kotlinx.coroutines.flow.update
import kotlinx.coroutines.launch
import java.util.Locale

enum class Phase { IDLE, RECORDING, WORKING }

data class TranslateUiState(
    val phase: Phase = Phase.IDLE,
    val status: String = "Tap Record and speak",
    /** What was said, in the source language. */
    val text: String = "",
    /** The same, in the target language. Empty until translated. */
    val translation: String = "",
    /** Where the last transcript came from and how long it took. */
    val details: String? = null,
    /** Why the result came from where it did, when that needs saying. */
    val routing: String? = null,
    val error: String? = null,
    val route: TranscriptionRoute = TranscriptionRoute.AUTO,
    val source: Language = Language.ENGLISH,
    val target: Language = Language.SPANISH,
    val sourceLanguages: List<Language> = emptyList(),
    val targetLanguages: List<Language> = emptyList(),
    /** The on-device speech model: missing, downloading, ready or failed. */
    val model: ModelState = ModelState.Missing,
    /** This device's measured speed as a real-time factor, or null if unmeasured. */
    val deviceSpeed: Float? = null,
    val measuringSpeed: Boolean = false,
    /** True when [deviceSpeed] is over the limit the automatic route allows. */
    val deviceTooSlow: Boolean = false,
)

class TranslateViewModel(
    private val recorder: AudioRecorder,
    private val pipeline: SpeechTranslationPipeline,
    private val route: MutableStateFlow<TranscriptionRoute>,
    private val models: ModelInstaller,
    private val speed: DeviceSpeed,
    sourceLanguages: List<Language>,
    targetLanguages: List<Language>,
    private val thresholds: FallbackThresholds = FallbackThresholds(),
    private val maxRecordingMs: Long = 30_000,
    /** A clock that only moves forward, in milliseconds. Replaced in tests. */
    private val now: () -> Long = { System.nanoTime() / 1_000_000 },
) : ViewModel() {

    private val _state = MutableStateFlow(
        TranslateUiState(
            route = route.value,
            sourceLanguages = sourceLanguages,
            targetLanguages = targetLanguages,
            model = models.state.value,
        ),
    )
    val state: StateFlow<TranslateUiState> = _state.asStateFlow()

    private var recording: Recording? = null
    private var autoStop: Job? = null

    init {
        // Mirror the model's state onto the screen for as long as it exists.
        viewModelScope.launch {
            models.state.collect { model -> _state.update { it.copy(model = model) } }
        }
        viewModelScope.launch {
            speed.realTimeFactor.collect { factor ->
                _state.update {
                    it.copy(
                        deviceSpeed = factor,
                        deviceTooSlow = factor != null && factor > thresholds.maxRealTimeFactor,
                    )
                }
            }
        }
        viewModelScope.launch {
            speed.measuring.collect { measuring -> _state.update { it.copy(measuringSpeed = measuring) } }
        }
    }

    fun onMeasureSpeedClicked() {
        if (_state.value.measuringSpeed) return
        viewModelScope.launch { speed.measureAgain() }
    }

    /** Starts, or after a failure continues, the download of the speech model. */
    fun onDownloadModelClicked() {
        if (_state.value.model is ModelState.Downloading || _state.value.model is ModelState.Verifying) return
        viewModelScope.launch {
            try {
                models.ensureInstalled()
            } catch (_: ModelException) {
                // Nothing to do here: the reason is already on screen, because
                // the manager's state is ModelState.Failed.
            }
        }
    }

    /** The one button: starts a recording, or stops it and sends it off. */
    fun onRecordClicked() {
        when (_state.value.phase) {
            Phase.IDLE -> startRecording()
            Phase.RECORDING -> stopAndProcess()
            Phase.WORKING -> Unit
        }
    }

    /** The choice of route: automatic, this device, or the server. */
    fun onRouteSelected(selected: TranscriptionRoute) {
        if (_state.value.phase != Phase.IDLE) return
        route.value = selected
        _state.update { it.copy(route = selected) }
    }

    fun onSourceSelected(language: Language) {
        if (_state.value.phase != Phase.IDLE) return
        _state.update { it.copy(source = language) }
    }

    fun onTargetSelected(language: Language) {
        if (_state.value.phase != Phase.IDLE) return
        _state.update { it.copy(target = language) }
    }

    fun onPermissionDenied() {
        _state.update { it.copy(error = "Microphone permission is needed to record") }
    }

    private fun startRecording() {
        recording = try {
            recorder.start()
        } catch (e: Exception) {
            _state.update { it.copy(error = e.message ?: "Could not start recording") }
            return
        }
        // A new recording clears the previous result but keeps the settings.
        _state.update {
            it.copy(
                phase = Phase.RECORDING,
                status = "Recording. Tap Stop when done",
                text = "",
                translation = "",
                details = null,
                routing = null,
                error = null,
            )
        }

        autoStop = viewModelScope.launch {
            delay(maxRecordingMs)
            stopAndProcess()
        }
    }

    private fun stopAndProcess() {
        val finished = recording ?: return
        recording = null
        _state.update { it.copy(phase = Phase.WORKING, status = "Preparing audio") }
        // Read once, so the whole run uses the languages chosen at this moment.
        val (source, target) = _state.value.let { it.source to it.target }

        viewModelScope.launch {
            val audio = finished.stop()
            if (audio.durationMs < MIN_RECORDING_MS) {
                finish(error = "That was too short. Hold on a little longer")
                return@launch
            }

            val startedAt = now()
            var failed = false
            pipeline.process(audio, source, target)
                // catch sees failures from the pipeline but lets cancellation
                // through, so leaving the screen still stops the work.
                .catch { e ->
                    failed = true
                    finish(error = e.message ?: "Something went wrong")
                }
                .collect { event -> show(event, audio.durationMs, startedAt, translating = source != target) }
            if (!failed) finish(error = null)
        }
        // Cancelled last: when the 30 second limit triggered this call, the
        // caller is the autoStop coroutine itself.
        autoStop?.cancel()
    }

    private fun show(event: PipelineEvent, audioMs: Long, startedAt: Long, translating: Boolean) {
        _state.update {
            when (event) {
                is PipelineEvent.Transcribing -> it.copy(status = event.stage.label())
                is PipelineEvent.PartialTranscript -> it.copy(text = event.text)
                is PipelineEvent.Transcribed -> it.copy(
                    status = "Transcribed",
                    text = event.transcript.text,
                    details = describe(event.transcript, audioMs, elapsedMs = now() - startedAt),
                    routing = event.transcript.note?.explanation(),
                )
                PipelineEvent.DownloadingLanguages -> it.copy(status = "Downloading language pack")
                PipelineEvent.Translating -> it.copy(status = "Translating")
                // With the same language on both sides there is nothing to
                // show twice.
                is PipelineEvent.Translated -> it.copy(
                    status = "Done",
                    translation = if (translating) event.text else "",
                )
                PipelineEvent.Speaking -> it.copy(status = "Speaking")
                PipelineEvent.Spoken -> it.copy(status = "Done")
                // The translation is on screen; only the voice is missing.
                is PipelineEvent.SpeechFailed -> it.copy(status = "Done", error = event.reason)
            }
        }
    }

    /**
     * For example "On device, 1.2 s for 4.0 s of audio (0.30x real time),
     * confidence 0.87". The real-time factor is time taken divided by audio
     * length: below 1 means faster than the speech itself.
     */
    private fun describe(transcript: Transcript, audioMs: Long, elapsedMs: Long): String {
        val source = when (transcript.source) {
            Transcript.Source.ON_DEVICE -> "On device"
            Transcript.Source.CLOUD -> "Cloud"
        }
        val timing = String.format(
            Locale.US,
            "%.1f s for %.1f s of audio (%.2fx real time)",
            elapsedMs / 1000.0,
            audioMs / 1000.0,
            elapsedMs.toDouble() / audioMs,
        )
        val confidence = transcript.confidence?.let { String.format(Locale.US, ", confidence %.2f", it) }.orEmpty()
        return "$source, $timing$confidence"
    }

    private fun finish(error: String?) {
        _state.update {
            // With no new error, one already on screen stays: a result can be
            // complete and still carry a note, such as "could not be spoken".
            it.copy(phase = Phase.IDLE, status = if (error == null) it.status else "Failed", error = error ?: it.error)
        }
    }

    override fun onCleared() {
        recording?.cancel() // let go of the microphone if the screen goes away
    }

    companion object {
        private const val MIN_RECORDING_MS = 300L

        val Factory: ViewModelProvider.Factory = viewModelFactory {
            initializer {
                val container = (this[APPLICATION_KEY] as PtranslateApp).container
                TranslateViewModel(
                    recorder = container.recorder,
                    pipeline = container.pipeline,
                    route = container.route,
                    models = container.models,
                    speed = container.speed,
                    sourceLanguages = container.sourceLanguages,
                    targetLanguages = container.targetLanguages,
                    thresholds = container.thresholds,
                )
            }
        }
    }
}

private fun RoutingNote.explanation(): String = when (this) {
    RoutingNote.NO_MODEL -> "Sent to the server: there is no speech model on this device"
    RoutingNote.SLOW_DEVICE -> "Sent to the server: this device is too slow for the model"
    RoutingNote.LOW_CONFIDENCE -> "Sent to the server: this device was not confident in its own result"
    RoutingNote.NO_SPEECH_ON_DEVICE -> "Sent to the server: this device heard sound but found no words"
    RoutingNote.GARBLED_ON_DEVICE -> "Sent to the server: this device could not make out the speech"
    RoutingNote.ON_DEVICE_FAILED -> "Sent to the server: the model on this device failed"
    RoutingNote.CLOUD_UNAVAILABLE -> "This device's result was kept: the server could not be reached"
}

private fun TranscriptionStage.label(): String = when (this) {
    TranscriptionStage.UPLOADING -> "Uploading"
    TranscriptionStage.QUEUED -> "Queued"
    TranscriptionStage.PROCESSING -> "Transcribing"
    TranscriptionStage.RECONNECTING -> "Connection lost. Reconnecting"
}
