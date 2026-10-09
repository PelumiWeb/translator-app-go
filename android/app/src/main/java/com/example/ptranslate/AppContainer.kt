package com.example.ptranslate

import android.app.Application
import android.content.Context
import com.example.ptranslate.core.Language
import com.example.ptranslate.core.audio.AndroidAudioRecorder
import com.example.ptranslate.core.audio.AudioRecorder
import com.example.ptranslate.core.audio.wavToPcm
import com.example.ptranslate.core.model.ModelInstaller
import com.example.ptranslate.core.model.ModelManager
import com.example.ptranslate.core.model.ModelState
import com.example.ptranslate.core.net.BackendClient
import com.example.ptranslate.core.pipeline.SpeechTranslationPipeline
import com.example.ptranslate.core.stt.DeviceBenchmark
import com.example.ptranslate.core.stt.DeviceSpeed
import com.example.ptranslate.core.stt.FallbackThresholds
import com.example.ptranslate.core.stt.RemoteTranscriber
import com.example.ptranslate.core.stt.SharedPreferencesSpeedStore
import com.example.ptranslate.core.stt.RoutingTranscriber
import com.example.ptranslate.core.stt.TranscriptionRoute
import com.example.ptranslate.core.stt.WhisperLanguages
import com.example.ptranslate.core.stt.WhisperTranscriber
import com.example.ptranslate.core.translate.MlKitTranslator
import com.example.ptranslate.core.translate.Translator
import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.SupervisorJob
import kotlinx.coroutines.asCoroutineDispatcher
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.collect
import kotlinx.coroutines.launch
import kotlinx.coroutines.withContext
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

    /** Which transcriber the next recording uses. The choice on screen sets it. */
    val route = MutableStateFlow(TranscriptionRoute.AUTO)

    val thresholds = FallbackThresholds()

    private val benchmark = DeviceBenchmark(
        // The file's name and size stand in for a version: a different model
        // has at least a different size.
        modelVersion = { (models.state.value as? ModelState.Ready)?.file?.let { "${it.name}:${it.length()}" } },
        // Eleven seconds of clear English speech, from whisper.cpp's samples.
        sample = {
            withContext(Dispatchers.IO) { wavToPcm(context.assets.open("benchmark.wav").use { it.readBytes() }) }
        },
        prepare = { whisper.load() },
        transcribe = { audio -> whisper.transcribe(audio, Language.ENGLISH).collect() },
        store = SharedPreferencesSpeedStore(context.getSharedPreferences("device", Context.MODE_PRIVATE)),
    )
    val speed: DeviceSpeed = benchmark

    // Lives as long as the app process, for work that belongs to no screen.
    private val appScope = CoroutineScope(SupervisorJob() + Dispatchers.Default)

    init {
        // Whenever a model becomes available, make sure its speed is known.
        appScope.launch {
            models.state.collect { state -> if (state is ModelState.Ready) benchmark.ensureMeasured() }
        }
    }

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
            realTimeFactor = { benchmark.realTimeFactor.value },
            thresholds = thresholds,
        ),
        translator = translator,
    )
}

/** Multilingual "base", quantised: 57 MB. The reasons are in ADR 0004. */
private const val WHISPER_MODEL_ID = "ggml-base-q5_1"

class PtranslateApp : Application() {
    val container: AppContainer by lazy { AppContainer(this) }
}
