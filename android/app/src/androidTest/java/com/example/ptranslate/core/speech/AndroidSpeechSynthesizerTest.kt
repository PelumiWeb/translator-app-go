package com.example.ptranslate.core.speech

import androidx.test.ext.junit.runners.AndroidJUnit4
import androidx.test.platform.app.InstrumentationRegistry
import com.example.ptranslate.core.Language
import kotlinx.coroutines.async
import kotlinx.coroutines.delay
import kotlinx.coroutines.runBlocking
import kotlinx.coroutines.withTimeout
import org.junit.Assert.assertThrows
import org.junit.Assert.assertTrue
import org.junit.Test
import org.junit.runner.RunWith

/**
 * Runs the phone's real text-to-speech engine. The device must have one, with
 * an English voice, which emulator images with Google services do.
 */
@RunWith(AndroidJUnit4::class)
class AndroidSpeechSynthesizerTest {

    private val synthesizer =
        AndroidSpeechSynthesizer(InstrumentationRegistry.getInstrumentation().targetContext)

    @Test
    fun speaksAndReturnsWhenFinished() = runBlocking {
        // Returning at all, without an exception, is the result: the engine
        // was found, accepted the language, and reported the utterance done.
        withTimeout(30_000) { synthesizer.speak("Good morning.", Language.ENGLISH) }
        synthesizer.close()
    }

    @Test
    fun stopEndsALongUtteranceEarly() = runBlocking {
        val long = "This sentence is repeated many times. ".repeat(40) // about a minute of speech
        val started = System.nanoTime()

        val speaking = async { synthesizer.speak(long, Language.ENGLISH) }
        delay(1_500)
        synthesizer.stop()
        withTimeout(10_000) { speaking.await() }

        val seconds = (System.nanoTime() - started) / 1_000_000_000.0
        assertTrue("speaking took $seconds s; stop did not cut it short", seconds < 15)
        synthesizer.close()
    }

    @Test
    fun aLanguageWithNoVoiceIsReported() {
        val error = assertThrows(SpeechException::class.java) {
            // Klingon: a valid language tag that no phone ships a voice for.
            runBlocking { synthesizer.speak("nuqneH", Language("tlh")) }
        }

        assertTrue(error.message.orEmpty().startsWith("This phone has no voice for"))
    }
}
