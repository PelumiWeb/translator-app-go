package com.example.ptranslate.core.speech

import android.content.Context
import android.speech.tts.TextToSpeech
import android.speech.tts.UtteranceProgressListener
import com.example.ptranslate.core.Language
import kotlinx.coroutines.CancellableContinuation
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.delay
import kotlinx.coroutines.suspendCancellableCoroutine
import kotlinx.coroutines.sync.Mutex
import kotlinx.coroutines.sync.withLock
import kotlinx.coroutines.withContext
import java.util.Locale
import java.util.UUID
import java.util.concurrent.ConcurrentHashMap
import kotlin.coroutines.resume
import kotlin.coroutines.resumeWithException

/**
 * Speaks with Android's built-in text-to-speech: a stock voice, offline once
 * the voice for a language is on the phone, and no extra dependency.
 *
 * Android's API reports progress through callbacks tagged with an utterance
 * id. This class turns that into one suspending call per utterance.
 */
class AndroidSpeechSynthesizer(
    private val context: Context,
    /** How many times to ask for a language whose voice is not on the phone yet. */
    private val voiceAttempts: Int = 3,
    private val voiceRetryDelayMs: Long = 4_000,
) : SpeechSynthesizer {

    private var engine: TextToSpeech? = null

    // One utterance at a time, and the engine is created only once.
    private val mutex = Mutex()

    // Callers waiting for their utterance to end, by utterance id. Written
    // from the caller's thread, completed from the engine's callback thread.
    private val waiting = ConcurrentHashMap<String, CancellableContinuation<Unit>>()

    override suspend fun speak(text: String, language: Language) = mutex.withLock {
        val tts = engine ?: connect().also { engine = it }

        val locale = Locale.forLanguageTag(language.tag)
        val available = tts.setLanguage(locale)
        if (available == TextToSpeech.LANG_MISSING_DATA || available == TextToSpeech.LANG_NOT_SUPPORTED) {
            throw SpeechException("This phone has no voice for ${locale.displayLanguage}")
        }

        // The first request in a new language often fails while the engine
        // fetches the voice for it. Give the download a few chances to finish
        // before reporting that to the user.
        var attempt = 1
        while (true) {
            try {
                speakOnce(tts, text)
                return@withLock
            } catch (e: EngineError) {
                if (!isVoiceNotReady(e.code) || attempt >= voiceAttempts) {
                    throw SpeechException(speechErrorMessage(e.code, locale.displayLanguage), e)
                }
            }
            delay(voiceRetryDelayMs)
            attempt++
        }
    }

    /** Speaks once and waits for the engine to report the end. */
    private suspend fun speakOnce(tts: TextToSpeech, text: String) {
        val id = UUID.randomUUID().toString()
        suspendCancellableCoroutine { continuation ->
            waiting[id] = continuation
            continuation.invokeOnCancellation {
                waiting.remove(id)
                tts.stop()
            }
            // QUEUE_FLUSH: replace anything still queued rather than wait behind it.
            if (tts.speak(text, TextToSpeech.QUEUE_FLUSH, null, id) != TextToSpeech.SUCCESS) {
                waiting.remove(id)
                continuation.resumeWithException(EngineError(TextToSpeech.ERROR))
            }
        }
    }

    override fun stop() {
        // The engine then reports the utterance as stopped, which ends speak().
        engine?.stop()
    }

    /** Releases the engine. The next speak() connects again. */
    fun close() {
        engine?.shutdown()
        engine = null
    }

    /** Binds to the phone's speech engine, which answers asynchronously. */
    private suspend fun connect(): TextToSpeech = withContext(Dispatchers.Main) {
        suspendCancellableCoroutine { continuation ->
            var tts: TextToSpeech? = null
            tts = TextToSpeech(context.applicationContext) { status ->
                val created = tts
                if (status == TextToSpeech.SUCCESS && created != null) {
                    created.setOnUtteranceProgressListener(listener)
                    continuation.resume(created)
                } else {
                    created?.shutdown()
                    continuation.resumeWithException(SpeechException("This phone has no text-to-speech engine"))
                }
            }
        }
    }

    private val listener = object : UtteranceProgressListener() {
        override fun onStart(utteranceId: String) = Unit

        override fun onDone(utteranceId: String) {
            waiting.remove(utteranceId)?.resume(Unit)
        }

        // Stopped on request: for the caller that is a normal end.
        override fun onStop(utteranceId: String, interrupted: Boolean) {
            waiting.remove(utteranceId)?.resume(Unit)
        }

        @Deprecated("Android calls the two-argument version on current releases")
        override fun onError(utteranceId: String) {
            waiting.remove(utteranceId)?.resumeWithException(EngineError(TextToSpeech.ERROR))
        }

        override fun onError(utteranceId: String, errorCode: Int) {
            waiting.remove(utteranceId)?.resumeWithException(EngineError(errorCode))
        }
    }

    /** A failure reported by the engine, kept as its raw code until speak() decides what it means. */
    private class EngineError(val code: Int) : Exception("text-to-speech error $code")
}
