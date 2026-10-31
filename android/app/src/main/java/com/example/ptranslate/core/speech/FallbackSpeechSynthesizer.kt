package com.example.ptranslate.core.speech

import com.example.ptranslate.core.Language

/**
 * Speaks with [preferred] when it can, and with [fallback] when it cannot, so
 * the translation is always heard. In the app: the user's own voice through
 * the server, or the phone's voice when no voice is recorded, there is no
 * network, or the server fails.
 */
class FallbackSpeechSynthesizer(
    private val preferred: SpeechSynthesizer,
    private val fallback: SpeechSynthesizer,
) : SpeechSynthesizer {

    override suspend fun speak(text: String, language: Language) {
        try {
            preferred.speak(text, language)
        } catch (_: SpeechException) {
            fallback.speak(text, language)
        }
    }

    override fun stop() {
        preferred.stop()
        fallback.stop()
    }
}
