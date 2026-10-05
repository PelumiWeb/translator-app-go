package com.example.ptranslate.core.translate

import com.example.ptranslate.core.Language
import com.google.android.gms.tasks.Task
import com.google.mlkit.common.model.DownloadConditions
import com.google.mlkit.common.model.RemoteModelManager
import com.google.mlkit.nl.translate.TranslateLanguage
import com.google.mlkit.nl.translate.TranslateRemoteModel
import com.google.mlkit.nl.translate.Translation
import com.google.mlkit.nl.translate.TranslatorOptions
import kotlinx.coroutines.suspendCancellableCoroutine
import kotlin.coroutines.resume
import kotlin.coroutines.resumeWithException
import com.google.mlkit.nl.translate.Translator as MlKitClient

/**
 * On-device translation with ML Kit.
 *
 * ML Kit keeps one language pack (about 30 MB) per language and downloads and
 * stores them itself, so they do not go through this app's model manager. It
 * translates through English: Yoruba to French is Yoruba to English, then
 * English to French, and needs both packs.
 */
class MlKitTranslator : Translator {

    private val models = RemoteModelManager.getInstance()

    // A client holds a loaded model pair, so each pair is created once and
    // reused. Guarded by synchronized: callers may be on different threads.
    private val clients = mutableMapOf<Pair<String, String>, MlKitClient>()

    override val supportedLanguages: List<Language> =
        TranslateLanguage.getAllLanguages().sorted().map(::Language)

    override suspend fun isReady(source: Language, target: Language): Boolean =
        isDownloaded(source) && isDownloaded(target)

    override suspend fun prepare(source: Language, target: Language) {
        // Outside the try: an unsupported language has its own, clearer error.
        val client = client(source, target)
        try {
            // No Wi-Fi requirement: the user is waiting for this translation.
            client.downloadModelIfNeeded(DownloadConditions.Builder().build()).await()
        } catch (e: Exception) {
            throw TranslationException("Could not download the language pack. Check the connection", e)
        }
    }

    override suspend fun translate(text: String, source: Language, target: Language): String {
        val client = client(source, target)
        return try {
            client.translate(text).await()
        } catch (e: Exception) {
            throw TranslationException("Translation failed", e)
        }
    }

    /** Releases the loaded models. */
    fun close() = synchronized(clients) {
        clients.values.forEach { it.close() }
        clients.clear()
    }

    private suspend fun isDownloaded(language: Language): Boolean {
        val model = TranslateRemoteModel.Builder(mlKitCode(language)).build()
        return try {
            models.isModelDownloaded(model).await()
        } catch (e: Exception) {
            false
        }
    }

    private fun client(source: Language, target: Language): MlKitClient {
        val pair = mlKitCode(source) to mlKitCode(target)
        return synchronized(clients) {
            clients.getOrPut(pair) {
                Translation.getClient(
                    TranslatorOptions.Builder()
                        .setSourceLanguage(pair.first)
                        .setTargetLanguage(pair.second)
                        .build(),
                )
            }
        }
    }

    private fun mlKitCode(language: Language): String =
        TranslateLanguage.fromLanguageTag(language.tag)
            ?: throw TranslationException("Translation does not support ${language.tag}")
}

/**
 * Turns a Google Play services Task into a suspending call. The library
 * kotlinx-coroutines-play-services provides the same thing; these few lines
 * save a dependency.
 */
private suspend fun <T> Task<T>.await(): T = suspendCancellableCoroutine { continuation ->
    addOnSuccessListener { continuation.resume(it) }
    addOnFailureListener { continuation.resumeWithException(it) }
    addOnCanceledListener { continuation.cancel() }
}
