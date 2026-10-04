package com.example.ptranslate

import android.app.Application
import com.example.ptranslate.core.audio.AndroidAudioRecorder
import com.example.ptranslate.core.audio.AudioRecorder
import com.example.ptranslate.core.net.BackendClient
import com.example.ptranslate.core.pipeline.SpeechTranslationPipeline
import com.example.ptranslate.core.stt.RemoteTranscriber
import okhttp3.OkHttpClient

/**
 * Builds the app's object graph by hand: every class receives what it needs
 * through its constructor, and this is the one place that decides which
 * implementation that is. There is no DI framework.
 */
class AppContainer {
    private val backend = BackendClient(BuildConfig.BACKEND_URL, OkHttpClient())

    val recorder: AudioRecorder = AndroidAudioRecorder()
    val pipeline = SpeechTranslationPipeline(RemoteTranscriber(backend))
}

class PtranslateApp : Application() {
    val container: AppContainer by lazy { AppContainer() }
}
