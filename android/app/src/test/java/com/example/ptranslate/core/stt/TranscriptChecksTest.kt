package com.example.ptranslate.core.stt

import org.junit.Assert.assertFalse
import org.junit.Assert.assertTrue
import org.junit.Test

class TranscriptChecksTest {

    // The two outputs that reached the screen, and were read aloud, before
    // this check existed.
    @Test
    fun `catches the repetition loops Whisper actually produced`() {
        val english = List(110) { "I," }.joinToString(" ")
        val spanish = "Yo, " + List(140) { "yo," }.joinToString(" ")

        assertTrue("ratio ${compressionRatio(english)}", looksLikeRepetitionLoop(english))
        assertTrue("ratio ${compressionRatio(spanish)}", looksLikeRepetitionLoop(spanish))
    }

    @Test
    fun `catches a repeated phrase as well as a repeated word`() {
        val loop = List(25) { "Thank you for watching." }.joinToString(" ")

        assertTrue("ratio ${compressionRatio(loop)}", looksLikeRepetitionLoop(loop))
    }

    @Test
    fun `leaves ordinary speech alone`() {
        val sentences = listOf(
            "Good morning, how are you?",
            "And so my fellow Americans, ask not what your country can do for you, ask what you can do for your country.",
            "Can you tell me where the nearest pharmacy is? I need something for a headache and I do not speak the language.",
            "Buenos días, ¿cómo estás? Me gustaría reservar una mesa para dos personas esta noche a las ocho.",
            // Long, and naturally repetitive in its wording.
            "I went to the market and I bought some rice, and then I went to the bank and I paid the bill, " +
                "and then I went home and I cooked the rice, and then I sat down and I ate it.",
        )
        for (sentence in sentences) {
            assertFalse("ratio ${compressionRatio(sentence)} for: $sentence", looksLikeRepetitionLoop(sentence))
        }
    }

    // People do repeat themselves. A handful of repeats must not be thrown away.
    @Test
    fun `leaves short, deliberate repetition alone`() {
        for (phrase in listOf("No, no, no, no!", "Yes yes yes", "Wait, wait, wait, wait, wait.", "Go go go go go go")) {
            assertFalse("ratio ${compressionRatio(phrase)} for: $phrase", looksLikeRepetitionLoop(phrase))
        }
    }

    // "[Music]" reached the screen as a transcript before this check existed.
    @Test
    fun `text that is only labels for sounds has no words in it`() {
        for (text in listOf("[Music]", "[BLANK_AUDIO]", "(applause)", " [Music] ", "[Music] [Applause]", "(wind blowing) [Music]", "[ Silence ]")) {
            assertTrue("labels only: $text", isOnlySoundLabels(text))
        }
    }

    @Test
    fun `speech is kept even when it has a label or brackets in it`() {
        for (text in listOf(
            "Hello [laughs] there",
            "Good morning.",
            "[Music] Good morning everyone",
            "It costs five dollars (about four euros).",
            "",
        )) {
            assertFalse("has words: $text", isOnlySoundLabels(text))
        }
    }

    @Test
    fun `an empty transcript is not a loop`() {
        assertFalse(looksLikeRepetitionLoop(""))
    }
}
