package com.example.ptranslate.core.stt

import java.util.zip.Deflater

/**
 * Above this, text repeats itself too much to be speech. The value is the one
 * OpenAI's reference Whisper uses for the same purpose.
 */
private const val MAX_COMPRESSION_RATIO = 2.4f

/**
 * How many times smaller the text gets when compressed. Ordinary sentences
 * come out between 1 and 2. Text made of one phrase repeated compresses to
 * almost nothing, so its ratio is very high.
 */
internal fun compressionRatio(text: String): Float {
    val bytes = text.toByteArray(Charsets.UTF_8)
    if (bytes.isEmpty()) return 1f

    val deflater = Deflater()
    try {
        deflater.setInput(bytes)
        deflater.finish()
        // Compressed output is never much larger than its input.
        val compressed = deflater.deflate(ByteArray(bytes.size + 64))
        return bytes.size.toFloat() / compressed
    } finally {
        deflater.end()
    }
}

/**
 * Whether a transcript is a repetition loop: the model stuck on one token or
 * phrase ("I, I, I, I, ..."), a known way for Whisper to fail.
 *
 * The model's confidence cannot be used to catch this. It is typically high
 * for such output, because each repeated token is, given the ones before it,
 * exactly what the model expects next. Compressibility measures the text
 * itself instead. A few repeated words do not trip it: short text does not
 * compress well enough to pass the limit.
 */
internal fun looksLikeRepetitionLoop(text: String): Boolean =
    compressionRatio(text) > MAX_COMPRESSION_RATIO
