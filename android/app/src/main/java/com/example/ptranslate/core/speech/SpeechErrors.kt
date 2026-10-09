package com.example.ptranslate.core.speech

// Android's TextToSpeech error codes, copied here as plain numbers so that the
// decisions made from them can be tested without an Android device.
private const val ERROR_NETWORK = -6
private const val ERROR_NETWORK_TIMEOUT = -7
private const val ERROR_NOT_INSTALLED_YET = -9

/**
 * Whether an engine error means "the voice for this language is not on the
 * phone yet". The first time a language is spoken, the engine has no voice
 * for it; it starts downloading one and fails that request, either saying so
 * directly or after trying to synthesise over the network instead. Asking
 * again a few seconds later usually works.
 */
internal fun isVoiceNotReady(errorCode: Int): Boolean =
    errorCode == ERROR_NOT_INSTALLED_YET || errorCode == ERROR_NETWORK || errorCode == ERROR_NETWORK_TIMEOUT

/** What to tell the user when speaking failed. [languageName] is e.g. "Spanish". */
internal fun speechErrorMessage(errorCode: Int, languageName: String): String =
    if (isVoiceNotReady(errorCode)) {
        "The $languageName voice is not on this phone yet. It may still be downloading: try again in a moment"
    } else {
        "The phone could not speak the $languageName translation (error $errorCode)"
    }
