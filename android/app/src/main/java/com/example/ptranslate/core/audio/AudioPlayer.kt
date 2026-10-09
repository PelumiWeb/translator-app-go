package com.example.ptranslate.core.audio

interface AudioPlayer {
    /** Plays the audio and returns when it has finished or [stop] was called. */
    suspend fun play(audio: PcmAudio)

    /** Stops what is playing. Does nothing if nothing is. */
    fun stop()
}
