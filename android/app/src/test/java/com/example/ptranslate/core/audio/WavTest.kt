package com.example.ptranslate.core.audio

import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertTrue
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

    @Test
    fun `float samples are scaled to the range minus one to one`() {
        val floats = PcmAudio(shortArrayOf(0, 16_384, Short.MAX_VALUE, Short.MIN_VALUE)).toFloatSamples()

        assertEquals(0f, floats[0], 0f)
        assertEquals(0.5f, floats[1], 0f)
        assertEquals(0.99997f, floats[2], 0.00001f)
        assertEquals(-1f, floats[3], 0f)
    }

    @Test
    fun `quiet recordings count as silent and speech-level ones do not`() {
        assertTrue("empty", PcmAudio(ShortArray(0)).isSilent())
        assertTrue("digital silence", PcmAudio(ShortArray(16_000)).isSilent())
        assertTrue("faint hiss", PcmAudio(ShortArray(16_000) { if (it % 2 == 0) 40 else -40 }).isSilent())
        assertFalse("quiet speech level", PcmAudio(ShortArray(16_000) { if (it % 2 == 0) 600 else -600 }).isSilent())
        // One loud click in a second of silence is not speech either.
        assertTrue("single click", PcmAudio(ShortArray(16_000).also { it[100] = 8_000 }).isSilent())
    }

    @Test
    fun `level is in decibels below full scale`() {
        assertEquals("digital silence", -100.0, PcmAudio(ShortArray(16_000)).levelDb(), 0.0)
        assertEquals("empty", -100.0, PcmAudio(ShortArray(0)).levelDb(), 0.0)
        // A constant 3277 is a tenth of full scale, which is -20 dB.
        assertEquals(-20.0, PcmAudio(ShortArray(16_000) { 3277 }).levelDb(), 0.01)
        assertEquals(-40.0, PcmAudio(ShortArray(16_000) { 328 }).levelDb(), 0.02)
    }

    private fun ByteArray.ascii(offset: Int) = String(this, offset, 4, Charsets.US_ASCII)
}
