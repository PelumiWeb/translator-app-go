package com.example.ptranslate.core.audio

import android.media.AudioAttributes
import android.media.AudioFormat
import android.media.AudioTrack
import kotlinx.coroutines.CoroutineDispatcher
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.delay
import kotlinx.coroutines.withContext

/** Plays PCM audio through the phone's media output with [AudioTrack]. */
class AndroidAudioPlayer(
    private val ioDispatcher: CoroutineDispatcher = Dispatchers.IO,
) : AudioPlayer {

    @Volatile
    private var playing: AudioTrack? = null

    override suspend fun play(audio: PcmAudio) {
        if (audio.samples.isEmpty()) return

        // MODE_STATIC: the whole clip is handed over before playback starts.
        // Right for short clips that are already in memory, and it plays
        // without the gaps a slow writer can cause in streaming mode.
        val track = AudioTrack.Builder()
            .setAudioAttributes(
                AudioAttributes.Builder()
                    .setUsage(AudioAttributes.USAGE_MEDIA)
                    .setContentType(AudioAttributes.CONTENT_TYPE_SPEECH)
                    .build(),
            )
            .setAudioFormat(
                AudioFormat.Builder()
                    .setEncoding(AudioFormat.ENCODING_PCM_16BIT)
                    .setSampleRate(audio.sampleRate)
                    .setChannelMask(AudioFormat.CHANNEL_OUT_MONO)
                    .build(),
            )
            .setTransferMode(AudioTrack.MODE_STATIC)
            .setBufferSizeInBytes(audio.samples.size * 2)
            .build()

        playing = track
        try {
            withContext(ioDispatcher) { track.write(audio.samples, 0, audio.samples.size) }
            track.play()
            // AudioTrack has no "finished" callback worth the ceremony for a
            // clip this short, so the position is polled. stop() ends the
            // loop by changing the play state.
            while (track.playState == AudioTrack.PLAYSTATE_PLAYING &&
                track.playbackHeadPosition < audio.samples.size
            ) {
                delay(30)
            }
        } finally {
            // Also reached when the caller is cancelled, so audio never
            // outlives the screen that started it.
            playing = null
            runCatching { track.stop() }
            track.release()
        }
    }

    override fun stop() {
        runCatching { playing?.stop() }
    }
}
