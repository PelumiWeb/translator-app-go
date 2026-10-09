package com.example.ptranslate.core.stt

import androidx.test.platform.app.InstrumentationRegistry
import com.example.ptranslate.core.audio.PcmAudio
import com.example.ptranslate.core.audio.wavToPcm

/** Where `make android-push-model` puts the model for device tests. */
const val TEST_MODEL_PATH = "/data/local/tmp/ptranslate/ggml-base-q5_1.bin"

/** Reads a 16 kHz, 16-bit mono WAV from the test assets. */
fun readSampleWav(name: String): PcmAudio =
    wavToPcm(InstrumentationRegistry.getInstrumentation().context.assets.open(name).use { it.readBytes() })
