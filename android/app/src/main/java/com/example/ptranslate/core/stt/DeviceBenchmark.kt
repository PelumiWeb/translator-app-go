package com.example.ptranslate.core.stt

import android.content.SharedPreferences
import com.example.ptranslate.core.audio.PcmAudio
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.asStateFlow
import kotlinx.coroutines.sync.Mutex
import kotlinx.coroutines.sync.withLock

/** How fast this device runs the speech model, for the screen and for routing. */
interface DeviceSpeed {
    /**
     * Time taken divided by audio length for the benchmark clip: 0.5 means
     * twice as fast as the speech itself. Null until it has been measured.
     */
    val realTimeFactor: StateFlow<Float?>
    val measuring: StateFlow<Boolean>

    /** Measures again, replacing the stored figure. */
    suspend fun measureAgain()
}

/** Where a measurement is kept between runs of the app. */
interface SpeedStore {
    fun load(modelVersion: String): Float?
    fun save(modelVersion: String, realTimeFactor: Float)
}

class SharedPreferencesSpeedStore(private val prefs: SharedPreferences) : SpeedStore {
    override fun load(modelVersion: String): Float? =
        prefs.getFloat(key(modelVersion), -1f).takeIf { it > 0f }

    override fun save(modelVersion: String, realTimeFactor: Float) {
        prefs.edit().putFloat(key(modelVersion), realTimeFactor).apply()
    }

    private fun key(modelVersion: String) = "real_time_factor:$modelVersion"
}

/**
 * Times the on-device model on a fixed clip of real speech.
 *
 * The figure is stored per model version, because a different model has a
 * different speed, and measured once: it costs several seconds of full CPU.
 *
 * @param modelVersion identifies the installed model, or null if there is none.
 * @param sample the benchmark clip.
 * @param prepare loads the model, so loading is not counted in the timing.
 * @param transcribe runs the model on the clip; its result is ignored.
 */
class DeviceBenchmark(
    private val modelVersion: () -> String?,
    private val sample: suspend () -> PcmAudio,
    private val prepare: suspend () -> Unit,
    private val transcribe: suspend (PcmAudio) -> Unit,
    private val store: SpeedStore,
    private val now: () -> Long = { System.nanoTime() / 1_000_000 },
) : DeviceSpeed {

    private val _realTimeFactor = MutableStateFlow<Float?>(null)
    override val realTimeFactor: StateFlow<Float?> = _realTimeFactor.asStateFlow()

    private val _measuring = MutableStateFlow(false)
    override val measuring: StateFlow<Boolean> = _measuring.asStateFlow()

    // One measurement at a time: two at once would halve each other's speed.
    private val mutex = Mutex()

    /** Uses the stored figure for the installed model, or measures if there is none. */
    suspend fun ensureMeasured() = mutex.withLock {
        val version = modelVersion() ?: return@withLock
        val stored = store.load(version)
        if (stored != null) _realTimeFactor.value = stored else measure(version)
    }

    override suspend fun measureAgain() = mutex.withLock {
        modelVersion()?.let { measure(it) }
        Unit
    }

    private suspend fun measure(version: String) {
        _measuring.value = true
        try {
            val audio = sample()
            prepare()
            val started = now()
            transcribe(audio)
            val factor = (now() - started).toFloat() / audio.durationMs

            store.save(version, factor)
            _realTimeFactor.value = factor
        } catch (e: TranscriptionException) {
            // The model would not run. Leave the speed unknown: routing then
            // tries the device anyway and falls back if that fails too.
            _realTimeFactor.value = null
        } finally {
            _measuring.value = false
        }
    }
}
