package com.example.ptranslate.core.stt

import androidx.test.ext.junit.runners.AndroidJUnit4
import com.example.ptranslate.core.Language
import com.example.ptranslate.core.audio.PcmAudio
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.ExperimentalCoroutinesApi
import kotlinx.coroutines.flow.toList
import kotlinx.coroutines.runBlocking
import org.junit.Assert.assertEquals
import org.junit.Assert.assertThrows
import org.junit.Assert.assertTrue
import org.junit.Assume.assumeTrue
import org.junit.Test
import org.junit.runner.RunWith
import java.io.File
import kotlin.random.Random

@OptIn(ExperimentalCoroutinesApi::class)
@RunWith(AndroidJUnit4::class)
class WhisperTranscriberTest {

    // limitedParallelism(1) gives the single-threaded access the transcriber requires.
    private fun transcriber(model: File) =
        WhisperTranscriber(model, Dispatchers.Default.limitedParallelism(1), threads = 4)

    @Test
    fun transcribesSpeechAndReportsConfidence() = runBlocking {
        val model = File(TEST_MODEL_PATH)
        assumeTrue("no model at $TEST_MODEL_PATH; run `make model-download android-push-model`", model.canRead())
        val whisper = transcriber(model)

        val events = whisper.transcribe(readSampleWav("jfk.wav"), Language.ENGLISH).toList()
        whisper.close()

        assertEquals(TranscriptEvent.StageChanged(TranscriptionStage.PROCESSING), events.first())
        val transcript = (events.last() as TranscriptEvent.Final).transcript
        assertTrue("transcript: ${transcript.text}", transcript.text.contains("ask not what your country", ignoreCase = true))
        assertEquals(Transcript.Source.ON_DEVICE, transcript.source)
        assertTrue("confidence: ${transcript.confidence}", transcript.confidence!! > 0.5f)
    }

    @Test
    fun silenceIsReportedWithoutRunningTheModel() {
        // No model at all: the silence check must answer before one is needed.
        val whisper = transcriber(File("/does/not/exist.bin"))
        val twoSilentSeconds = PcmAudio(ShortArray(32_000))

        val error = assertThrows(TranscriptionException::class.java) {
            runBlocking { whisper.transcribe(twoSilentSeconds, Language.ENGLISH).toList() }
        }

        assertEquals("The recording was silent (-100 dB). Check the microphone", error.message)
    }

    // Loud enough to get past the silence check, so the model itself has to
    // recognise that there are no words in it.
    @Test
    fun noiseIsReportedAsNoSpeech() {
        val model = File(TEST_MODEL_PATH)
        assumeTrue("no model at $TEST_MODEL_PATH", model.canRead())
        val whisper = transcriber(model)
        val random = Random(seed = 1)
        val threeSecondsOfHiss = PcmAudio(ShortArray(48_000) { random.nextInt(-400, 400).toShort() })
        assertTrue("the hiss must be above the silence threshold", !threeSecondsOfHiss.isSilent())

        val error = assertThrows(TranscriptionException::class.java) {
            runBlocking {
                val events = whisper.transcribe(threeSecondsOfHiss, Language.ENGLISH).toList()
                throw AssertionError("expected no speech, got: ${events.last()}")
            }
        }

        assertEquals("No speech was recognised", error.message)
    }

    @Test
    fun aMissingModelIsReportedPlainly() {
        val whisper = transcriber(File("/does/not/exist.bin"))

        val error = assertThrows(TranscriptionException::class.java) {
            // Real speech: silent audio would be rejected before the model is needed.
            runBlocking { whisper.transcribe(readSampleWav("jfk.wav"), Language.ENGLISH).toList() }
        }

        assertEquals("The on-device model is not installed", error.message)
    }
}
