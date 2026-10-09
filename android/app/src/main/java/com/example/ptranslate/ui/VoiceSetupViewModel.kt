package com.example.ptranslate.ui

import androidx.lifecycle.ViewModel
import androidx.lifecycle.ViewModelProvider
import androidx.lifecycle.ViewModelProvider.AndroidViewModelFactory.Companion.APPLICATION_KEY
import androidx.lifecycle.viewModelScope
import androidx.lifecycle.viewmodel.initializer
import androidx.lifecycle.viewmodel.viewModelFactory
import com.example.ptranslate.PtranslateApp
import com.example.ptranslate.core.audio.AudioPlayer
import com.example.ptranslate.core.audio.AudioRecorder
import com.example.ptranslate.core.audio.PcmAudio
import com.example.ptranslate.core.audio.Recording
import com.example.ptranslate.core.voice.VoiceSampleRules
import com.example.ptranslate.core.voice.VoiceSampleState
import com.example.ptranslate.core.voice.VoiceSampleStore
import com.example.ptranslate.core.voice.VoiceSetupPreference
import kotlinx.coroutines.Job
import kotlinx.coroutines.delay
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.asStateFlow
import kotlinx.coroutines.flow.update
import kotlinx.coroutines.launch

enum class VoiceSetupStage {
    /** Explaining, and waiting for the user to start. */
    INTRO,
    RECORDING,

    /** A recording has been made and is waiting to be kept or redone. */
    REVIEW,
}

data class VoiceSetupUiState(
    val stage: VoiceSetupStage = VoiceSetupStage.INTRO,
    /** How long the current recording has been running. */
    val elapsedMs: Long = 0,
    /** Length of the recording under review. */
    val recordedMs: Long = 0,
    /** What is wrong with the recording under review; null means it will do. */
    val problem: String? = null,
    val playing: Boolean = false,
    /** Length of the recording already saved on this phone, if there is one. */
    val savedMs: Long? = null,
    val error: String? = null,
    /** Set when the user is finished here, one way or another. */
    val finished: Boolean = false,
)

/** Records, reviews and saves the sample of the user's voice. */
class VoiceSetupViewModel(
    private val recorder: AudioRecorder,
    private val player: AudioPlayer,
    private val samples: VoiceSampleStore,
    private val preference: VoiceSetupPreference,
    private val maxRecordingMs: Long = 30_000,
    private val now: () -> Long = { System.nanoTime() / 1_000_000 },
) : ViewModel() {

    private val _state = MutableStateFlow(VoiceSetupUiState())
    val state: StateFlow<VoiceSetupUiState> = _state.asStateFlow()

    private var recording: Recording? = null
    private var timer: Job? = null
    private var playback: Job? = null

    /** The recording under review. Held here, not in the state: it is half a megabyte. */
    private var candidate: PcmAudio? = null

    init {
        viewModelScope.launch {
            samples.state.collect { saved ->
                _state.update { it.copy(savedMs = (saved as? VoiceSampleState.Present)?.durationMs) }
            }
        }
    }

    /** Starts recording, or stops it and moves on to the review. */
    fun onRecordClicked() {
        when (_state.value.stage) {
            VoiceSetupStage.INTRO, VoiceSetupStage.REVIEW -> startRecording()
            VoiceSetupStage.RECORDING -> stopRecording()
        }
    }

    fun onPermissionDenied() {
        _state.update { it.copy(error = "Microphone permission is needed to record your voice") }
    }

    private fun startRecording() {
        stopPlayback()
        recording = try {
            recorder.start()
        } catch (e: Exception) {
            _state.update { it.copy(error = e.message ?: "Could not start recording") }
            return
        }
        candidate = null
        _state.update {
            it.copy(stage = VoiceSetupStage.RECORDING, elapsedMs = 0, problem = null, error = null)
        }

        val startedAt = now()
        timer = viewModelScope.launch {
            while (true) {
                val elapsed = now() - startedAt
                _state.update { it.copy(elapsedMs = elapsed) }
                if (elapsed >= maxRecordingMs) {
                    stopRecording()
                    return@launch
                }
                delay(100)
            }
        }
    }

    private fun stopRecording() {
        val finished = recording ?: return
        recording = null

        viewModelScope.launch {
            val audio = finished.stop()
            candidate = audio
            _state.update {
                it.copy(
                    stage = VoiceSetupStage.REVIEW,
                    recordedMs = audio.durationMs,
                    problem = VoiceSampleRules.problemWith(audio),
                )
            }
        }
        // Cancelled last: when the time limit triggered this call, the caller
        // is the timer coroutine itself.
        timer?.cancel()
    }

    /** Plays the recording under review, or stops it if it is playing. */
    fun onListenClicked() {
        if (_state.value.playing) {
            stopPlayback()
            return
        }
        val audio = candidate ?: return
        playback = viewModelScope.launch {
            _state.update { it.copy(playing = true) }
            try {
                player.play(audio)
            } finally {
                _state.update { it.copy(playing = false) }
            }
        }
    }

    /** Keeps the recording under review as the user's voice. */
    fun onUseClicked() {
        val audio = candidate ?: return
        if (_state.value.problem != null) return
        stopPlayback()
        viewModelScope.launch {
            try {
                samples.save(audio)
            } catch (e: Exception) {
                _state.update { it.copy(error = "The recording could not be saved") }
                return@launch
            }
            preference.skipped = false
            candidate = null
            _state.update { it.copy(finished = true) }
        }
    }

    /** Leaves without recording. The phone's own voice is used, and the app does not ask again. */
    fun onSkipClicked() {
        discardRecording()
        // Only "do not ask again" if there is nothing saved; with a saved
        // voice this button just means "keep it as it is".
        if (_state.value.savedMs == null) preference.skipped = true
        _state.update { it.copy(finished = true) }
    }

    /** Deletes the saved recording from the phone. */
    fun onDeleteClicked() {
        discardRecording()
        viewModelScope.launch {
            samples.delete()
            preference.skipped = true
            _state.update { it.copy(finished = true) }
        }
    }

    /** Called when the screen has been left, so it starts fresh next time. */
    fun onLeft() {
        _state.update { VoiceSetupUiState(savedMs = it.savedMs) }
    }

    private fun discardRecording() {
        stopPlayback()
        timer?.cancel()
        recording?.cancel()
        recording = null
        candidate = null
    }

    private fun stopPlayback() {
        player.stop()
        playback?.cancel()
    }

    override fun onCleared() {
        recording?.cancel()
        player.stop()
    }

    companion object {
        val Factory: ViewModelProvider.Factory = viewModelFactory {
            initializer {
                val container = (this[APPLICATION_KEY] as PtranslateApp).container
                VoiceSetupViewModel(
                    recorder = container.recorder,
                    player = container.player,
                    samples = container.voice,
                    preference = container.voiceSetup,
                )
            }
        }
    }
}
