package com.example.ptranslate

import android.app.Application
import android.content.Context
import com.example.ptranslate.core.Language
import com.example.ptranslate.core.audio.AndroidAudioRecorder
import com.example.ptranslate.core.audio.AudioRecorder
import com.example.ptranslate.core.model.ModelInstaller
import com.example.ptranslate.core.model.ModelManager
import com.example.ptranslate.core.model.ModelState
import com.example.ptranslate.core.net.BackendClient
import com.example.ptranslate.core.pipeline.SpeechTranslationPipeline
import com.example.ptranslate.core.stt.RemoteTranscriber
import com.example.ptranslate.core.stt.RoutingTranscriber
import com.example.ptranslate.core.stt.TranscriptionRoute
import com.example.ptranslate.core.stt.WhisperLanguages
import com.example.ptranslate.core.stt.WhisperTranscriber
import com.example.ptranslate.core.translate.MlKitTranslator
import com.example.ptranslate.core.translate.Translator
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.asCoroutineDispatcher
import kotlinx.coroutines.flow.MutableStateFlow
import okhttp3.OkHttpClient
import java.io.File
import java.util.concurrent.Executors

/**
 * Builds the app's object graph by hand: every class receives what it needs
 * through its constructor, and this is the one place that decides which
 * implementation that is. There is no DI framework.
 */
class AppContainer(context: Context) {
    private val backend = BackendClient(BuildConfig.BACKEND_URL, OkHttpClient())

    // One thread, because a Whisper context must not be used concurrently.
    // Whisper spreads the work over several cores itself (see threads below).
    private val whisperDispatcher =
        Executors.newSingleThreadExecutor { Thread(it, "whisper") }.asCoroutineDispatcher()

    /** Downloads the Whisper model from the backend and keeps it in the app's files. */
    val models: ModelInstaller = ModelManager(
        backend = backend,
        directory = File(context.filesDir, "models"),
        modelId = WHISPER_MODEL_ID,
        ioDispatcher = Dispatchers.IO,
    )

    private val whisper = WhisperTranscriber(
        modelFile = { (models.state.value as? ModelState.Ready)?.file },
        dispatcher = whisperDispatcher,
        // More threads than fast cores makes Whisper slower, not faster:
        // the slow cores hold the others back.
        threads = Runtime.getRuntime().availableProcessors().coerceIn(1, 4),
    )

    /** Which transcriber the next recording uses. The screen's switch sets it. */
    val route = MutableStateFlow(TranscriptionRoute.ON_DEVICE)

    val recorder: AudioRecorder = AndroidAudioRecorder()
    private val translator: Translator = MlKitTranslator()

    /** A language can be spoken only if Whisper transcribes it and ML Kit translates from it. */
    val sourceLanguages: List<Language> = translator.supportedLanguages.filter(WhisperLanguages::supports)
    val targetLanguages: List<Language> = translator.supportedLanguages

    val pipeline = SpeechTranslationPipeline(
        transcriber = RoutingTranscriber(
            onDevice = whisper,
            cloud = RemoteTranscriber(backend),
            route = route,
            modelReady = { models.state.value is ModelState.Ready },
            // Not measured yet; the benchmark arrives in checkpoint 6.2.
            realTimeFactor = { null },
        ),
        translator = translator,
    )
}

/** Multilingual "base", quantised: 57 MB. The reasons are in ADR 0004. */
private const val WHISPER_MODEL_ID = "ggml-base-q5_1"

class PtranslateApp : Application() {
    val container: AppContainer by lazy { AppContainer(this) }
}
