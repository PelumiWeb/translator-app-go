package com.example.ptranslate.core.stt

import com.example.ptranslate.core.Language
import org.junit.Assert.assertFalse
import org.junit.Assert.assertTrue
import org.junit.Test

class WhisperLanguagesTest {

    @Test
    fun `knows what Whisper can and cannot transcribe`() {
        assertTrue(WhisperLanguages.supports(Language("en")))
        assertTrue(WhisperLanguages.supports(Language("yo")))
        assertTrue(WhisperLanguages.supports(Language("yue")))
        // ML Kit translates these two, but Whisper cannot transcribe them.
        assertFalse(WhisperLanguages.supports(Language("eo")))
        assertFalse(WhisperLanguages.supports(Language("ga")))
        assertFalse(WhisperLanguages.supports(Language("")))
    }
}
