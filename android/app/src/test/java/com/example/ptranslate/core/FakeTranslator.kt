package com.example.ptranslate.core

import com.example.ptranslate.core.translate.TranslationException
import com.example.ptranslate.core.translate.Translator

/** Translates "hello" from en to es as "[en>es] hello", and records what it was asked. */
class FakeTranslator(private var ready: Boolean = true) : Translator {
    var prepareError: TranslationException? = null
    var translateError: TranslationException? = null
    val calls = mutableListOf<String>()

    override val supportedLanguages = listOf(Language("en"), Language("es"), Language("yo"))

    override suspend fun isReady(source: Language, target: Language): Boolean = ready

    override suspend fun prepare(source: Language, target: Language) {
        calls += "prepare ${source.tag}>${target.tag}"
        prepareError?.let { throw it }
        ready = true
    }

    override suspend fun translate(text: String, source: Language, target: Language): String {
        calls += "translate ${source.tag}>${target.tag}"
        translateError?.let { throw it }
        return "[${source.tag}>${target.tag}] $text"
    }
}
