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
    /** Stops capturing and returns what was recorded. */
    suspend fun stop(): PcmAudio

    /** Stops capturing and throws the audio away. */
    fun cancel()
}
