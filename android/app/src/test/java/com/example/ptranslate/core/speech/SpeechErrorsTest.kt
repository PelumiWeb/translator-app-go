package com.example.ptranslate.core.speech

import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertTrue
import org.junit.Test

class SpeechErrorsTest {

    @Test
    fun `a missing or unreachable voice is worth asking for again`() {
        assertTrue("not installed yet", isVoiceNotReady(-9))
        assertTrue("network", isVoiceNotReady(-6))
        assertTrue("network timeout", isVoiceNotReady(-7)) // what the emulator reported on first use
    }

    @Test
    fun `other engine failures are not retried`() {
        for (code in listOf(-1, -3, -4, -5, -8)) { // generic, synthesis, service, output, invalid request
            assertFalse("code $code", isVoiceNotReady(code))
        }
    }

    @Test
    fun `tells the user in words when the voice is not there yet`() {
        assertEquals(
            "The Spanish voice is not on this phone yet. It may still be downloading: try again in a moment",
            speechErrorMessage(-7, "Spanish"),
        )
    }

    @Test
    fun `keeps the code for failures it cannot explain`() {
        assertEquals("The phone could not speak the French translation (error -3)", speechErrorMessage(-3, "French"))
    }
}
