package com.example.ptranslate.core.speech

import com.example.ptranslate.core.FakeSynthesizer
import com.example.ptranslate.core.Language
import kotlinx.coroutines.runBlocking
import org.junit.Assert.assertEquals
import org.junit.Assert.assertThrows
import org.junit.Test

class FallbackSpeechSynthesizerTest {

    private val ownVoice = FakeSynthesizer()
    private val phoneVoice = FakeSynthesizer()
    private val synthesizer = FallbackSpeechSynthesizer(preferred = ownVoice, fallback = phoneVoice)

    private fun speak() = runBlocking { synthesizer.speak("buenos días", Language("es")) }

    @Test
    fun `speaks in the preferred voice when it can`() {
        speak()

        assertEquals(listOf("es: buenos días"), ownVoice.spoken)
        assertEquals(emptyList<String>(), phoneVoice.spoken)
    }

    @Test
    fun `falls back when the preferred voice cannot speak`() {
        ownVoice.failure = SpeechException("Could not reach the server")

        speak()

        assertEquals(listOf("es: buenos días"), phoneVoice.spoken)
    }

    @Test
    fun `reports the fallback's failure when neither can speak`() {
        ownVoice.failure = SpeechException("Could not reach the server")
        phoneVoice.failure = SpeechException("This phone has no voice for Spanish")

        val error = assertThrows(SpeechException::class.java) { speak() }

        // The one that says why nothing at all was heard.
        assertEquals("This phone has no voice for Spanish", error.message)
    }

    @Test
    fun `stop stops both`() {
        synthesizer.stop()

        assertEquals(1, ownVoice.stops)
        assertEquals(1, phoneVoice.stops)
    }
}
