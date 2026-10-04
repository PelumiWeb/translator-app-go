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
import com.example.ptranslate.core.pipeline.PipelineEvent
import com.example.ptranslate.core.pipeline.SpeechTranslationPipeline
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
    val text: String = "",
    /** Where the last transcript came from and how long it took. */
    val details: String? = null,
    val error: String? = null,
    val onDevice: Boolean = true,
)

class TranslateViewModel(
    private val recorder: AudioRecorder,
    private val pipeline: SpeechTranslationPipeline,
    private val route: MutableStateFlow<TranscriptionRoute>,
    private val maxRecordingMs: Long = 30_000,
    /** A clock that only moves forward, in milliseconds. Replaced in tests. */
    private val now: () -> Long = { System.nanoTime() / 1_000_000 },
) : ViewModel() {

    private val _state = MutableStateFlow(TranslateUiState(onDevice = route.value == TranscriptionRoute.ON_DEVICE))
    val state: StateFlow<TranslateUiState> = _state.asStateFlow()

    private var recording: Recording? = null
    private var autoStop: Job? = null

    /** The one button: starts a recording, or stops it and sends it off. */
    fun onRecordClicked() {
        when (_state.value.phase) {
            Phase.IDLE -> startRecording()
            Phase.RECORDING -> stopAndTranscribe()
            Phase.WORKING -> Unit
        }
    }

    /** The switch: transcribe on this device, or on the server. */
    fun onRouteChanged(onDevice: Boolean) {
        if (_state.value.phase != Phase.IDLE) return
        route.value = if (onDevice) TranscriptionRoute.ON_DEVICE else TranscriptionRoute.CLOUD
        _state.update { it.copy(onDevice = onDevice) }
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
        _state.update {
            TranslateUiState(phase = Phase.RECORDING, status = "Recording. Tap Stop when done", onDevice = it.onDevice)
        }

        autoStop = viewModelScope.launch {
            delay(maxRecordingMs)
            stopAndTranscribe()
        }
    }

    private fun stopAndTranscribe() {
        val finished = recording ?: return
        recording = null
        _state.update { it.copy(phase = Phase.WORKING, status = "Preparing audio") }

        viewModelScope.launch {
            val audio = finished.stop()
            if (audio.durationMs < MIN_RECORDING_MS) {
                finish(error = "That was too short. Hold on a little longer")
                return@launch
            }

            val startedAt = now()
            var failed = false
            pipeline.process(audio, Language.ENGLISH)
                // catch sees failures from the pipeline but lets cancellation
                // through, so leaving the screen still stops the work.
                .catch { e ->
                    failed = true
                    finish(error = e.message ?: "Something went wrong")
                }
                .collect { event -> show(event, audio.durationMs, startedAt) }
            if (!failed) finish(error = null)
        }
        // Cancelled last: when the 30 second limit triggered this call, the
        // caller is the autoStop coroutine itself.
        autoStop?.cancel()
    }

    private fun show(event: PipelineEvent, audioMs: Long, startedAt: Long) {
        _state.update {
            when (event) {
                is PipelineEvent.Transcribing -> it.copy(status = event.stage.label())
                is PipelineEvent.PartialTranscript -> it.copy(text = event.text)
                is PipelineEvent.Transcribed -> it.copy(
                    status = "Done",
                    text = event.transcript.text,
                    details = describe(event.transcript, audioMs, elapsedMs = now() - startedAt),
                )
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
            it.copy(phase = Phase.IDLE, status = if (error == null) it.status else "Failed", error = error)
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
                TranslateViewModel(container.recorder, container.pipeline, container.route)
            }
        }
    }
}

private fun TranscriptionStage.label(): String = when (this) {
    TranscriptionStage.UPLOADING -> "Uploading"
    TranscriptionStage.QUEUED -> "Queued"
    TranscriptionStage.PROCESSING -> "Transcribing"
}
