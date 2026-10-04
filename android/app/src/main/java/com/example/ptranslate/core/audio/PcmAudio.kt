package com.example.ptranslate.core.audio

/** Mono 16-bit PCM audio, the format Whisper and the backend both expect. */
class PcmAudio(val samples: ShortArray, val sampleRate: Int = SAMPLE_RATE_HZ) {

    val durationMs: Long
        get() = samples.size * 1000L / sampleRate

    companion object {
        const val SAMPLE_RATE_HZ = 16_000
    }
}
