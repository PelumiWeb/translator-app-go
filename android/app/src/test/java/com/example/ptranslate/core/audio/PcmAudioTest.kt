package com.example.ptranslate.core.audio

import org.junit.Assert.assertEquals
import org.junit.Test

class PcmAudioTest {

    @Test
    fun `levels are the loudness of equal slices, relative to the loudest`() {
        // Four slices of 100 samples: silent, quiet, loud, half as loud.
        val samples = ShortArray(400) { i ->
            when (i / 100) {
                0 -> 0
                1 -> if (i % 2 == 0) 1000 else -1000
                2 -> if (i % 2 == 0) 4000 else -4000
                else -> if (i % 2 == 0) 2000 else -2000
            }.toShort()
        }

        assertEquals(listOf(0f, 0.25f, 1f, 0.5f), PcmAudio(samples).levels(4))
    }

    @Test
    fun `silence and empty audio have no level`() {
        assertEquals(listOf(0f, 0f, 0f), PcmAudio(ShortArray(300)).levels(3))
        assertEquals(listOf(0f, 0f), PcmAudio(ShortArray(0)).levels(2))
    }
}
