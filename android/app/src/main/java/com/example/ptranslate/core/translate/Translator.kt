package com.example.ptranslate.core.translate

import com.example.ptranslate.core.Language

/**
 * Translates text between two languages. The app depends on this interface
 * only; ML Kit is one implementation of it.
 */
interface Translator {
    /** Every language this translator can translate from and to. */
    val supportedLanguages: List<Language>

    /** Whether translating between these two needs no download first. */
    suspend fun isReady(source: Language, target: Language): Boolean

    /**
     * Downloads whatever [translate] needs for this pair. Does nothing if it
     * is already there.
     *
     * @throws TranslationException if the download fails, for example offline.
     */
    suspend fun prepare(source: Language, target: Language)

    /** @throws TranslationException if the pair is unsupported or not prepared. */
    suspend fun translate(text: String, source: Language, target: Language): String
}

class TranslationException(message: String, cause: Throwable? = null) : Exception(message, cause)
