package com.example.ptranslate.core.audio

interface AudioRecorder {
    /**
     * Starts capturing from the microphone. The caller must hold the
     * RECORD_AUDIO permission.
     *
     * @throws IllegalStateException if the microphone cannot be opened.
     */
    fun start(): Recording
}

/** A capture in progress. Finish it with exactly one of [stop] or [cancel]. */
interface Recording {
    /** Stops listening for now, keeping what was recorded. The microphone is off until [resume]. */
    fun pause()

    /**
     * Listens again after [pause], adding to what was recorded. If the microphone cannot be had again, as during a
     * phone call, the recording keeps what it has.
     */
    fun resume()

    /** Stops capturing and returns what was recorded. */
    suspend fun stop(): PcmAudio

    /** Stops capturing and throws the audio away. */
    fun cancel()
}
