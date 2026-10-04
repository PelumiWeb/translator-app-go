package com.example.ptranslate.core

/** A language as a BCP 47 tag such as "en" or "yo". */
@JvmInline
value class Language(val tag: String) {
    companion object {
        val ENGLISH = Language("en")
    }
}
