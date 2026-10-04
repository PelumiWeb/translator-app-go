package com.example.ptranslate.core.stt

import androidx.test.ext.junit.runners.AndroidJUnit4
import androidx.test.platform.app.InstrumentationRegistry
import org.junit.Assert.assertEquals
import org.junit.Assert.assertNotNull
import org.junit.Assert.assertThrows
import org.junit.Assert.assertTrue
import org.junit.Assume.assumeTrue
import org.junit.Test
import org.junit.runner.RunWith
import java.io.File

/**
 * Runs on a device or emulator, because it exercises the native library.
 *
 * The transcription test needs a model, which is too large for the repository.
 * `make model-download android-push-model` puts one where this test looks;
 * without it that test is skipped, not failed.
 */
@RunWith(AndroidJUnit4::class)
class WhisperContextTest {

    @Test
    fun nativeLibraryLoads() {
        val info = WhisperContext.systemInfo()

        assertTrue("system info: $info", info.contains("NEON") || info.contains("AVX"))
    }

    @Test
    fun loadingAMissingModelFails() {
        val error = assertThrows(TranscriptionException::class.java) {
            WhisperContext.load(File("/does/not/exist.bin"))
        }

        assertTrue(error.message.orEmpty().startsWith("No model file"))
    }

    @Test
    fun loadingAFileThatIsNotAModelFails() {
        val context = InstrumentationRegistry.getInstrumentation().targetContext
        val junk = File(context.cacheDir, "junk.bin").apply { writeText("this is not a model") }

        val error = assertThrows(TranscriptionException::class.java) { WhisperContext.load(junk) }

        assertEquals("junk.bin is not a usable Whisper model", error.message)
    }

    @Test
    fun transcribesTheSampleRecording() {
        val model = File(TEST_MODEL_PATH)
        assumeTrue("no model at $TEST_MODEL_PATH; run `make model-download android-push-model`", model.canRead())

        val samples = readSampleWav("jfk.wav").toFloatSamples()

        val result = WhisperContext.load(model).use { whisper ->
            whisper.transcribe(samples, languageTag = "en", threads = 4)
        }

        val normalised = result.text.lowercase().replace(Regex("[^a-z ]"), "")
        assertTrue(
            "transcript: ${result.text}",
            normalised.contains("ask not what your country can do for you"),
        )
        val confidence = result.meanTokenProbability
        assertNotNull(confidence)
        assertTrue("confidence: $confidence", confidence!! > 0.5f && confidence <= 1f)
    }

    @Test
    fun aClosedContextCannotBeUsed() {
        val model = File(TEST_MODEL_PATH)
        assumeTrue("no model at $TEST_MODEL_PATH", model.canRead())

        val whisper = WhisperContext.load(model)
        whisper.close()
        whisper.close() // closing twice is harmless

        assertThrows(IllegalStateException::class.java) {
            whisper.transcribe(FloatArray(16_000), languageTag = "en", threads = 1)
        }
    }
}
