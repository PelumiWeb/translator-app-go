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
     * [onStart] is called once, when the speech becomes audible, with the
     * voice it is in. That can be seconds after the call: a voice made on the
     * server takes that long. It may be called on another thread.
     *
     * @throws SpeechException if it cannot be spoken, for example because the
     *   phone has no voice for that language.
     */
    suspend fun speak(text: String, language: Language, onStart: (SpeechVoice) -> Unit = {})

    /** Stops whatever is being spoken. Does nothing if nothing is. */
    fun stop()
}

/** Whose voice a translation is spoken in. */
enum class SpeechVoice {
    /** The user's own, made on the server from their recording. */
    OWN,

    /** One of the phone's built-in voices. */
    PHONE,
}

class SpeechException(message: String, cause: Throwable? = null) : Exception(message, cause)
