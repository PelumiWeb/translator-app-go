package com.example.ptranslate

import android.app.Application
import android.content.Context
import com.example.ptranslate.core.audio.AndroidAudioRecorder
import com.example.ptranslate.core.audio.AudioRecorder
import com.example.ptranslate.core.net.BackendClient
import com.example.ptranslate.core.pipeline.SpeechTranslationPipeline
import com.example.ptranslate.core.stt.RemoteTranscriber
import com.example.ptranslate.core.stt.RoutingTranscriber
import com.example.ptranslate.core.stt.TranscriptionRoute
import com.example.ptranslate.core.stt.WhisperTranscriber
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

    private val whisper = WhisperTranscriber(
        // Where the model manager will put models from milestone 4. Until
        // then `make android-push-model` copies the file here.
        modelFile = File(context.filesDir, "models/ggml-base-q5_1.bin"),
        dispatcher = whisperDispatcher,
        // More threads than fast cores makes Whisper slower, not faster:
        // the slow cores hold the others back.
        threads = Runtime.getRuntime().availableProcessors().coerceIn(1, 4),
    )

    /** Which transcriber the next recording uses. The screen's switch sets it. */
    val route = MutableStateFlow(TranscriptionRoute.ON_DEVICE)

    val recorder: AudioRecorder = AndroidAudioRecorder()
    val pipeline = SpeechTranslationPipeline(
        RoutingTranscriber(onDevice = whisper, cloud = RemoteTranscriber(backend), route = route),
    )
}

class PtranslateApp : Application() {
    val container: AppContainer by lazy { AppContainer(this) }
}
