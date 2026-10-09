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

/**
 * Reads a WAV file that holds 16-bit mono PCM, the only kind this app writes
 * or bundles.
 *
 * A WAV file is a list of chunks, each with a four-letter name and a length.
 * The samples are in the "data" chunk, which need not come first: files often
 * carry a "LIST" chunk of metadata before it. So the chunks are walked, not
 * assumed to sit at fixed offsets.
 *
 * @throws IllegalArgumentException if this is not such a file.
 */
fun wavToPcm(bytes: ByteArray): PcmAudio {
    require(bytes.size >= 12 && bytes.ascii(0) == "RIFF" && bytes.ascii(8) == "WAVE") { "Not a WAV file" }
    val buffer = ByteBuffer.wrap(bytes).order(ByteOrder.LITTLE_ENDIAN)

    var sampleRate: Int? = null
    var offset = 12
    while (offset + 8 <= bytes.size) {
        val name = bytes.ascii(offset)
        val size = buffer.getInt(offset + 4)
        val body = offset + 8
        require(size >= 0 && body + size <= bytes.size) { "The WAV file is cut short" }

        when (name) {
            "fmt " -> {
                require(size >= 16) { "The WAV format chunk is too short" }
                val format = buffer.getShort(body).toInt()
                val channels = buffer.getShort(body + 2).toInt()
                val bits = buffer.getShort(body + 14).toInt()
                require(format == FORMAT_PCM.toInt() && channels == 1 && bits == 16) {
                    "Only 16-bit mono PCM is supported (got format $format, $channels channels, $bits bits)"
                }
                sampleRate = buffer.getInt(body + 4)
            }

            "data" -> {
                val rate = requireNotNull(sampleRate) { "The WAV file has no format chunk before its data" }
                return PcmAudio(ShortArray(size / BYTES_PER_SAMPLE) { buffer.getShort(body + it * 2) }, rate)
            }
        }
        offset = body + size + (size and 1) // chunks are padded to an even length
    }
    throw IllegalArgumentException("The WAV file has no data chunk")
}

private fun ByteArray.ascii(offset: Int) = String(this, offset, 4, Charsets.US_ASCII)
