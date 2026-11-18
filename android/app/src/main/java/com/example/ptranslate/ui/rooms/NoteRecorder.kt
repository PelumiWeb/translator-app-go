package com.example.ptranslate.ui.rooms

import com.example.ptranslate.core.audio.AudioRecorder
import com.example.ptranslate.core.audio.PcmAudio
import com.example.ptranslate.core.audio.Recording
import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.Deferred
import kotlinx.coroutines.Job
import kotlinx.coroutines.async
import kotlinx.coroutines.delay
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.asStateFlow
import kotlinx.coroutines.flow.update
import kotlinx.coroutines.launch

/** Where the voice note in the composer is. */
enum class NotePhase {
    IDLE,
    RECORDING,

    /** The microphone is off; recording can go on. */
    PAUSED,

    /** Finished, to listen to, then send or throw away. */
    STOPPED,
}

data class NoteState(val phase: NotePhase = NotePhase.IDLE, val elapsedMs: Long = 0)

/**
 * The voice note in a room's composer: recording, paused, or stopped to be listened to before it is sent. Only the
 * time spent recording counts toward the limit; at the limit the note stops and waits to be sent, rather than being
 * sent for the person.
 */
class NoteRecorder(
    private val recorder: AudioRecorder,
    private val scope: CoroutineScope,
    private val maxMs: Long,
    private val now: () -> Long,
) {
    private val _state = MutableStateFlow(NoteState())
    val state: StateFlow<NoteState> = _state.asStateFlow()

    /** The finished note, while [NotePhase.STOPPED]. */
    var audio: PcmAudio? = null
        private set

    private var recording: Recording? = null
    private var ending: Deferred<PcmAudio>? = null
    private var clock: Job? = null

    /** Time recorded before the current stretch, and when that stretch began. */
    private var bankedMs = 0L
    private var stretchFrom = 0L

    /** Starts a new note. Throws what the recorder throws when the microphone cannot be opened. */
    fun start() {
        if (_state.value.phase != NotePhase.IDLE) return
        recording = recorder.start()
        bankedMs = 0
        _state.value = NoteState(NotePhase.RECORDING)
        runClock()
    }

    fun pause() {
        val live = recording ?: return
        if (_state.value.phase != NotePhase.RECORDING) return
        clock?.cancel()
        bankedMs = elapsed()
        live.pause()
        _state.value = NoteState(NotePhase.PAUSED, bankedMs)
    }

    fun resume() {
        val live = recording ?: return
        if (_state.value.phase != NotePhase.PAUSED) return
        live.resume()
        _state.value = NoteState(NotePhase.RECORDING, bankedMs)
        runClock()
    }

    /** Ends the recording, to be listened to before it is sent. */
    suspend fun stop() {
        val ending = end() ?: return
        val audio = ending.await()
        if (this.ending !== ending) return // cancelled meanwhile, or another caller took it
        this.ending = null
        this.audio = audio
        _state.value = NoteState(NotePhase.STOPPED, audio.durationMs)
    }

    /** Ends the note, in whatever phase, and hands it over; the composer is empty again. */
    suspend fun finish(): PcmAudio? {
        stop()
        return audio.also { clear() }
    }

    /** Throws the note away. */
    fun cancel() {
        clock?.cancel()
        recording?.cancel()
        recording = null
        ending = null
        clear()
    }

    /**
     * Ends the capture once, however many ask. It runs in the recorder's own scope, so it finishes even when the
     * caller, such as the clock reaching the limit, is cancelled along the way.
     */
    private fun end(): Deferred<PcmAudio>? {
        ending?.let { return it }
        val live = recording ?: return null
        recording = null
        clock?.cancel()
        return scope.async { live.stop() }.also { ending = it }
    }

    private fun clear() {
        audio = null
        bankedMs = 0
        _state.value = NoteState()
    }

    private fun runClock() {
        stretchFrom = now()
        clock?.cancel()
        clock = scope.launch {
            while (true) {
                val elapsed = elapsed()
                _state.update { it.copy(elapsedMs = elapsed) }
                if (elapsed >= maxMs) {
                    scope.launch { stop() }
                    return@launch
                }
                delay(100)
            }
        }
    }

    private fun elapsed() = bankedMs + (now() - stretchFrom)
}
