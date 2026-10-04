package com.example.ptranslate.core.audio

import kotlin.math.log10
import kotlin.math.sqrt

/** Mono 16-bit PCM audio, the format Whisper and the backend both expect. */
class PcmAudio(val samples: ShortArray, val sampleRate: Int = SAMPLE_RATE_HZ) {

    val durationMs: Long
        get() = samples.size * 1000L / sampleRate

    /** The samples scaled to the range -1 to 1, the form Whisper takes. */
    fun toFloatSamples(): FloatArray = FloatArray(samples.size) { samples[it] / 32768f }

    /**
     * How loud the recording is, in decibels relative to the loudest value a
     * sample can hold: 0 is full scale, more negative is quieter. Measured as
     * the root mean square of the samples, the usual measure of loudness.
     */
    fun levelDb(): Double {
        if (samples.isEmpty()) return MIN_LEVEL_DB
        val rms = sqrt(samples.sumOf { it.toDouble() * it } / samples.size)
        if (rms == 0.0) return MIN_LEVEL_DB
        return (20 * log10(rms / 32768.0)).coerceAtLeast(MIN_LEVEL_DB)
    }

    /** True when the recording is too quiet to contain speech: a muted or absent microphone, for example. */
    fun isSilent(): Boolean = levelDb() < SILENCE_DB

    companion object {
        const val SAMPLE_RATE_HZ = 16_000

        // Quiet speech is around -35 dB; the noise floor of a phone
        // microphone in a quiet room is near -60.
        private const val SILENCE_DB = -50.0

        /** Reported for digital silence, where the true value is minus infinity. */
        private const val MIN_LEVEL_DB = -100.0
    }
}
