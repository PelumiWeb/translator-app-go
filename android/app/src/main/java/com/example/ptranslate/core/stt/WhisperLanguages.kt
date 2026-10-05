package com.example.ptranslate.core.stt

import com.example.ptranslate.core.Language

/**
 * The languages the multilingual Whisper models can transcribe, as listed in
 * whisper.cpp (`g_lang` in src/whisper.cpp). The cloud path is held to the
 * same list so that a recording can always fall back from one to the other.
 */
object WhisperLanguages {
    private val tags: Set<String> = (
        "en zh de es ru ko fr ja pt tr pl ca nl ar sv it id hi fi vi he uk el ms cs ro da hu ta no " +
            "th ur hr bg lt la mi ml cy sk te fa lv bn sr az sl kn et mk br eu is hy ne mn bs kk sq sw " +
            "gl mr pa si km sn yo so af oc ka be tg sd gu am yi lo uz fo ht ps tk nn mt sa lb my bo tl " +
            "mg as tt haw ln ha ba jw su yue"
        ).split(" ").toSet()

    fun supports(language: Language): Boolean = language.tag in tags
}
