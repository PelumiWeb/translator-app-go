package com.example.ptranslate.core

import com.example.ptranslate.core.speech.SpeechException
import com.example.ptranslate.core.speech.SpeechSynthesizer
import kotlinx.coroutines.CompletableDeferred

/** Records what it was asked to say. Speaking ends at once unless [hold] is set. */
class FakeSynthesizer : SpeechSynthesizer {
    val spoken = mutableListOf<String>()
    var failure: SpeechException? = null
    var stops = 0

    /** When set, speak() waits here until [stop] or the test completes it. */
    var hold: CompletableDeferred<Unit>? = null

    override suspend fun speak(text: String, language: Language) {
        failure?.let { throw it }
        spoken += "${language.tag}: $text"
        hold?.await()
    }

    override fun stop() {
        stops++
        hold?.complete(Unit)
    }
}
