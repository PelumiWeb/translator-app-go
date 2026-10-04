package com.example.ptranslate.core.audio

import org.junit.Assert.assertEquals
import org.junit.Test
import java.nio.ByteBuffer
import java.nio.ByteOrder

class WavTest {

    @Test
    fun `header describes 16 kHz mono 16-bit PCM`() {
        val wav = PcmAudio(shortArrayOf(1, -2, 300)).toWav()
        val bytes = ByteBuffer.wrap(wav).order(ByteOrder.LITTLE_ENDIAN)

        assertEquals(44 + 6, wav.size)
        assertEquals("RIFF", wav.ascii(0))
        assertEquals(36 + 6, bytes.getInt(4))
        assertEquals("WAVE", wav.ascii(8))
        assertEquals("fmt ", wav.ascii(12))
        assertEquals(16, bytes.getInt(16))
        assertEquals("format: PCM", 1, bytes.getShort(20).toInt())
        assertEquals("channels", 1, bytes.getShort(22).toInt())
        assertEquals("sample rate", 16_000, bytes.getInt(24))
        assertEquals("bytes per second", 32_000, bytes.getInt(28))
        assertEquals("bytes per frame", 2, bytes.getShort(32).toInt())
        assertEquals("bits per sample", 16, bytes.getShort(34).toInt())
        assertEquals("data", wav.ascii(36))
        assertEquals(6, bytes.getInt(40))
    }

    @Test
    fun `samples follow the header in little-endian order`() {
        val wav = PcmAudio(shortArrayOf(1, -2, 300)).toWav()
        val bytes = ByteBuffer.wrap(wav).order(ByteOrder.LITTLE_ENDIAN)

        assertEquals(1, bytes.getShort(44).toInt())
        assertEquals(-2, bytes.getShort(46).toInt())
        assertEquals(300, bytes.getShort(48).toInt())
        // 300 is 0x012C: low byte first.
        assertEquals(0x2C, wav[48].toInt())
        assertEquals(0x01, wav[49].toInt())
    }

    @Test
    fun `empty audio is a header with no data`() {
        val wav = PcmAudio(ShortArray(0)).toWav()

        assertEquals(44, wav.size)
        assertEquals(0, ByteBuffer.wrap(wav).order(ByteOrder.LITTLE_ENDIAN).getInt(40))
    }

    @Test
    fun `duration is derived from the sample count`() {
        assertEquals(1_500L, PcmAudio(ShortArray(24_000)).durationMs)
    }

    private fun ByteArray.ascii(offset: Int) = String(this, offset, 4, Charsets.US_ASCII)
}
