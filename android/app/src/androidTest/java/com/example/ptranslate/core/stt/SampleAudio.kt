package com.example.ptranslate.core.stt

import androidx.test.platform.app.InstrumentationRegistry
import com.example.ptranslate.core.audio.PcmAudio
import java.nio.ByteBuffer
import java.nio.ByteOrder

/** Where `make android-push-model` puts the model for device tests. */
const val TEST_MODEL_PATH = "/data/local/tmp/ptranslate/ggml-base-q5_1.bin"

/** Reads a 16 kHz, 16-bit mono WAV from the test assets. */
fun readSampleWav(name: String): PcmAudio {
    val bytes = InstrumentationRegistry.getInstrumentation().context.assets.open(name).use { it.readBytes() }
    val buffer = ByteBuffer.wrap(bytes).order(ByteOrder.LITTLE_ENDIAN)

    // Walk the chunks after the 12-byte RIFF header until "data". The sample
    // file has a LIST chunk first, so its samples do not start at byte 44.
    var offset = 12
    while (offset + 8 <= bytes.size) {
        val id = String(bytes, offset, 4, Charsets.US_ASCII)
        val size = buffer.getInt(offset + 4)
        if (id == "data") {
            val start = offset + 8
            return PcmAudio(ShortArray(size / 2) { i -> buffer.getShort(start + i * 2) })
        }
        offset += 8 + size + (size and 1) // chunks are padded to an even size
    }
    error("$name has no data chunk")
}
