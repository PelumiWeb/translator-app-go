package com.example.ptranslate.core.audio

import java.nio.ByteBuffer
import java.nio.ByteOrder

private const val HEADER_BYTES = 44
private const val BYTES_PER_SAMPLE = 2
private const val FORMAT_PCM: Short = 1
private const val CHANNELS: Short = 1

/**
 * Wraps the samples in a WAV container: a 44-byte header followed by the raw
 * samples. WAV needs no encoder, and a 30 second clip is under 1 MB.
 */
fun PcmAudio.toWav(): ByteArray {
    val dataBytes = samples.size * BYTES_PER_SAMPLE
    // Every number in a WAV file is little-endian.
    val out = ByteBuffer.allocate(HEADER_BYTES + dataBytes).order(ByteOrder.LITTLE_ENDIAN)

    out.put("RIFF".toByteArray(Charsets.US_ASCII))
    out.putInt(HEADER_BYTES - 8 + dataBytes) // size of everything after this field
    out.put("WAVE".toByteArray(Charsets.US_ASCII))

    out.put("fmt ".toByteArray(Charsets.US_ASCII))
    out.putInt(16) // size of the fmt chunk
    out.putShort(FORMAT_PCM)
    out.putShort(CHANNELS)
    out.putInt(sampleRate)
    out.putInt(sampleRate * CHANNELS * BYTES_PER_SAMPLE) // bytes per second
    out.putShort((CHANNELS * BYTES_PER_SAMPLE).toShort()) // bytes per frame
    out.putShort((BYTES_PER_SAMPLE * 8).toShort()) // bits per sample

    out.put("data".toByteArray(Charsets.US_ASCII))
    out.putInt(dataBytes)
    samples.forEach(out::putShort)

    return out.array()
}
