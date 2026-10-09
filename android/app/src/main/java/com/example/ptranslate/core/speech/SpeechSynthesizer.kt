package com.example.ptranslate.core.speech

import com.example.ptranslate.core.Language

/**
 * Says text aloud. The app depends on this interface only: today the phone's
 * built-in voices are behind it, later a voice cloned from the speaker.
 */
interface SpeechSynthesizer {
    /**
     * Speaks [text] in [language] and returns when it has finished, or when
     * [stop] cut it short. Cancelling the caller stops the speech too.
     *
     * @throws SpeechException if it cannot be spoken, for example because the
     *   phone has no voice for that language.
     */
    suspend fun speak(text: String, language: Language)

    /** Stops whatever is being spoken. Does nothing if nothing is. */
    fun stop()
}

class SpeechException(message: String, cause: Throwable? = null) : Exception(message, cause)
