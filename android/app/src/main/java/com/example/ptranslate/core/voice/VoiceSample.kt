package com.example.ptranslate.core.voice

import com.example.ptranslate.core.audio.PcmAudio
import com.example.ptranslate.core.audio.toWav
import kotlinx.coroutines.CoroutineDispatcher
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.asStateFlow
import kotlinx.coroutines.withContext
import java.io.File

/** Whether this phone holds a recording of its owner's voice. */
sealed interface VoiceSampleState {
    data object None : VoiceSampleState
    data class Present(val durationMs: Long) : VoiceSampleState
}

/**
 * Keeps the one recording of the user's voice that translations are spoken
 * in. It lives on the phone and nowhere else: it is sent to the server with
 * each request to speak, and the server deletes it afterwards.
 */
interface VoiceSampleStore {
    val state: StateFlow<VoiceSampleState>

    /** Replaces any earlier recording. */
    suspend fun save(audio: PcmAudio)

    /** The recording as a WAV file, ready to upload, or null if there is none. */
    suspend fun loadWav(): ByteArray?

    suspend fun delete()
}

/** What a recording must be like to clone a voice from. */
object VoiceSampleRules {
    /** Below this a voice-cloning engine has too little to go on. */
    const val MIN_MS = 10_000L

    /** What the passage on screen is written to take when read aloud. */
    const val TARGET_MS = 20_000L

    /** Returns what is wrong with the recording, in words for the user, or null if it will do. */
    fun problemWith(audio: PcmAudio): String? = when {
        audio.isSilent() -> "Nothing was heard. Check the microphone and try again"
        audio.durationMs < MIN_MS ->
            "That was only ${audio.durationMs / 1000} seconds. About 20 are needed: please read the whole passage"
        else -> null
    }
}

/** Stores the recording as one WAV file in the app's private files. */
class FileVoiceSampleStore(
    private val directory: File,
    private val ioDispatcher: CoroutineDispatcher,
) : VoiceSampleStore {

    private val file = File(directory, "voice-sample.wav")

    private val _state = MutableStateFlow(readState())
    override val state: StateFlow<VoiceSampleState> = _state.asStateFlow()

    override suspend fun save(audio: PcmAudio) = withContext(ioDispatcher) {
        directory.mkdirs()
        // Written beside the real file and renamed over it, so a crash half
        // way leaves the previous recording intact instead of a broken one.
        val incoming = File(directory, "voice-sample.part")
        incoming.writeBytes(audio.toWav())
        check(incoming.renameTo(file)) { "Could not save the voice recording" }
        _state.value = VoiceSampleState.Present(audio.durationMs)
    }

    override suspend fun loadWav(): ByteArray? = withContext(ioDispatcher) {
        if (file.isFile) file.readBytes() else null
    }

    override suspend fun delete() = withContext(ioDispatcher) {
        file.delete()
        _state.value = VoiceSampleState.None
    }

    private fun readState(): VoiceSampleState {
        if (!file.isFile || file.length() <= WAV_HEADER_BYTES) return VoiceSampleState.None
        // 16 kHz, 16-bit mono: 32 bytes per millisecond.
        return VoiceSampleState.Present((file.length() - WAV_HEADER_BYTES) / BYTES_PER_MS)
    }

    private companion object {
        const val WAV_HEADER_BYTES = 44L
        const val BYTES_PER_MS = PcmAudio.SAMPLE_RATE_HZ * 2 / 1000
    }
}

/** Whether the user chose to go without their own voice, so they are not asked on every launch. */
interface VoiceSetupPreference {
    var skipped: Boolean
}
