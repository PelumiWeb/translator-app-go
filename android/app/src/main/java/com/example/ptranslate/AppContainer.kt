package com.example.ptranslate

import android.app.Application
import android.content.Context
import com.example.ptranslate.core.Language
import com.example.ptranslate.core.account.Account
import com.example.ptranslate.core.account.KeystoreSessionStore
import com.example.ptranslate.core.account.SharedPreferencesOnboardingProgress
import com.example.ptranslate.core.audio.AndroidAudioPlayer
import com.example.ptranslate.core.audio.AndroidAudioRecorder
import com.example.ptranslate.core.audio.AudioPlayer
import com.example.ptranslate.core.audio.AudioRecorder
import com.example.ptranslate.core.audio.wavToPcm
import com.example.ptranslate.core.model.ModelInstaller
import com.example.ptranslate.core.model.ModelManager
import com.example.ptranslate.core.model.ModelState
import com.example.ptranslate.core.net.AccountClient
import com.example.ptranslate.core.net.BackendClient
import com.example.ptranslate.core.net.Rest
import com.example.ptranslate.core.net.RoomsClient
import com.example.ptranslate.core.rooms.MessagePlayer
import com.example.ptranslate.core.rooms.RoomFeed
import com.example.ptranslate.core.pipeline.SpeechTranslationPipeline
import com.example.ptranslate.core.speech.AndroidSpeechSynthesizer
import com.example.ptranslate.core.speech.FallbackSpeechSynthesizer
import com.example.ptranslate.core.speech.RemoteSpeechSynthesizer
import com.example.ptranslate.core.speech.SpeechSynthesizer
import com.example.ptranslate.core.stt.DeviceBenchmark
import com.example.ptranslate.core.stt.DeviceSpeed
import com.example.ptranslate.core.stt.FallbackThresholds
import com.example.ptranslate.core.stt.RemoteTranscriber
import com.example.ptranslate.core.stt.SharedPreferencesSpeedStore
import com.example.ptranslate.core.stt.RoutingTranscriber
import com.example.ptranslate.core.stt.Transcriber
import com.example.ptranslate.core.stt.TranscriptionRoute
import com.example.ptranslate.core.stt.WhisperLanguages
import com.example.ptranslate.core.stt.WhisperTranscriber
import com.example.ptranslate.core.translate.MlKitTranslator
import com.example.ptranslate.core.voice.FileVoiceSampleStore
import com.example.ptranslate.core.voice.SharedPreferencesVoiceSetup
import com.example.ptranslate.core.voice.VoiceSampleStore
import com.example.ptranslate.core.voice.VoiceSetupPreference
import com.example.ptranslate.core.translate.Translator
import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.SupervisorJob
import kotlinx.coroutines.asCoroutineDispatcher
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.collect
import kotlinx.coroutines.flow.drop
import kotlinx.coroutines.flow.filter
import kotlinx.coroutines.launch
import kotlinx.coroutines.withContext
import okhttp3.OkHttpClient
import java.io.File
import java.util.Locale
import java.util.concurrent.Executors

/**
 * Builds the app's object graph by hand: every class receives what it needs
 * through its constructor, and this is the one place that decides which
 * implementation that is. There is no DI framework.
 */
class AppContainer(context: Context) {
    private val http = OkHttpClient()
    private val backend = BackendClient(BuildConfig.BACKEND_URL, http)

    // Requests on behalf of the signed-in user. Its token comes from the
    // account, and a session the server says has ended is forgotten there;
    // the lambdas reach the account only when a request runs, after it exists.
    private val rest = Rest(BuildConfig.BACKEND_URL, http, token = { account.token }, onSessionEnded = { account.forget() })

    // Small settings that must outlive the app process. Declared first:
    // properties are initialised top to bottom, and several below use it.
    private val preferences = context.getSharedPreferences("device", Context.MODE_PRIVATE)

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
        store = SharedPreferencesSpeedStore(preferences),
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
    val player: AudioPlayer = AndroidAudioPlayer()

    /** The signed-in user, kept across launches, the token sealed by the Android Keystore (ADR 0018). */
    val account: Account by lazy {
        Account(
            client = AccountClient(rest),
            store = KeystoreSessionStore(context.getSharedPreferences("session", Context.MODE_PRIVATE)),
            progress = SharedPreferencesOnboardingProgress(preferences),
        )
    }

    val rooms = RoomsClient(rest)

    /** One stream of room changes for every screen that shows rooms. */
    val roomFeed = RoomFeed(rooms, appScope)

    /** The language the phone is set to, offered first when signing up, if Whisper knows it. */
    val phoneLanguage: Language? = Language(Locale.getDefault().language).takeIf(WhisperLanguages::supports)

    /** The recording of the user's voice, in the app's private files. */
    val voice: VoiceSampleStore = FileVoiceSampleStore(File(context.filesDir, "voice"), Dispatchers.IO)
    val voiceSetup: VoiceSetupPreference = SharedPreferencesVoiceSetup(preferences)
    private val translator: Translator = MlKitTranslator()

    /**
     * Says translations aloud in the user's own voice, through the server, or
     * in one of the phone's voices when that cannot be done.
     */
    private val phoneVoice = AndroidSpeechSynthesizer(context)

    val synthesizer: SpeechSynthesizer = FallbackSpeechSynthesizer(
        preferred = RemoteSpeechSynthesizer(backend, voice, player),
        fallback = phoneVoice,
    )

    /** Plays room notes: the sender's voice from the server, or this phone's voice reading the words. */
    val messagePlayer = MessagePlayer(rooms, player, phoneVoice, Dispatchers.Default)

    /** A language can be spoken only if Whisper transcribes it and ML Kit translates from it. */
    val sourceLanguages: List<Language> = translator.supportedLanguages.filter(WhisperLanguages::supports)
    val targetLanguages: List<Language> = translator.supportedLanguages

    /** Writes down what was said: on this phone or on the server, as the route and the phone allow. */
    val transcriber: Transcriber = RoutingTranscriber(
        onDevice = whisper,
        cloud = RemoteTranscriber(backend),
        route = route,
        modelReady = { models.state.value is ModelState.Ready },
        realTimeFactor = { benchmark.realTimeFactor.value },
        thresholds = thresholds,
    )

    val pipeline = SpeechTranslationPipeline(
        transcriber = transcriber,
        translator = translator,
        synthesizer = synthesizer,
    )

    init {
        // Signing out, or deleting the account, leaves nothing of the person
        // on this phone: the next person to sign in must not speak in their
        // voice. Only a change counts; the first value is how the app opened.
        appScope.launch {
            account.session.drop(1).filter { it == null }.collect {
                voice.delete()
                voiceSetup.skipped = false
            }
        }
    }
}

/** Multilingual "base", quantised: 57 MB. The reasons are in ADR 0004. */
private const val WHISPER_MODEL_ID = "ggml-base-q5_1"

class PtranslateApp : Application() {
    val container: AppContainer by lazy { AppContainer(this) }
}
