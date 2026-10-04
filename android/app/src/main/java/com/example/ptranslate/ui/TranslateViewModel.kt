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
import com.example.ptranslate.core.stt.TranscriptionStage
import kotlinx.coroutines.Job
import kotlinx.coroutines.delay
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.asStateFlow
import kotlinx.coroutines.flow.catch
import kotlinx.coroutines.flow.update
import kotlinx.coroutines.launch

enum class Phase { IDLE, RECORDING, WORKING }

data class TranslateUiState(
    val phase: Phase = Phase.IDLE,
    val status: String = "Tap Record and speak",
    val text: String = "",
    val error: String? = null,
)

class TranslateViewModel(
    private val recorder: AudioRecorder,
    private val pipeline: SpeechTranslationPipeline,
    private val maxRecordingMs: Long = 30_000,
) : ViewModel() {

    private val _state = MutableStateFlow(TranslateUiState())
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
        _state.value = TranslateUiState(phase = Phase.RECORDING, status = "Recording. Tap Stop when done")

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

            var failed = false
            pipeline.process(audio, Language.ENGLISH)
                // catch sees failures from the pipeline but lets cancellation
                // through, so leaving the screen still stops the work.
                .catch { e ->
                    failed = true
                    finish(error = e.message ?: "Something went wrong")
                }
                .collect(::show)
            if (!failed) finish(error = null)
        }
        // Cancelled last: when the 30 second limit triggered this call, the
        // caller is the autoStop coroutine itself.
        autoStop?.cancel()
    }

    private fun show(event: PipelineEvent) {
        _state.update {
            when (event) {
                is PipelineEvent.Transcribing -> it.copy(status = event.stage.label())
                is PipelineEvent.PartialTranscript -> it.copy(text = event.text)
                is PipelineEvent.Transcribed -> it.copy(status = "Done", text = event.transcript.text)
            }
        }
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
                TranslateViewModel(container.recorder, container.pipeline)
            }
        }
    }
}

private fun TranscriptionStage.label(): String = when (this) {
    TranscriptionStage.UPLOADING -> "Uploading"
    TranscriptionStage.QUEUED -> "Queued"
    TranscriptionStage.PROCESSING -> "Transcribing"
}
