package com.example.ptranslate.core.stt

import com.example.ptranslate.core.Language
import com.example.ptranslate.core.audio.PcmAudio
import kotlinx.coroutines.CoroutineDispatcher
import kotlinx.coroutines.flow.Flow
import kotlinx.coroutines.flow.flow
import kotlinx.coroutines.withContext
import java.io.File
import kotlin.math.roundToInt

/**
 * Transcribes on the device with whisper.cpp.
 *
 * @param modelFile returns the installed model, or null while there is none.
 *   It is asked on every transcription, so a model that finishes downloading
 *   is picked up without restarting anything.
 * @param dispatcher must run on a single thread: it is what keeps the
 *   underlying [WhisperContext] from being used by two callers at once.
 * @param threads how many CPU threads Whisper itself may use.
 */
class WhisperTranscriber(
    private val modelFile: () -> File?,
    private val dispatcher: CoroutineDispatcher,
    private val threads: Int,
) : Transcriber {

    // Read and written only on [dispatcher]. The model is loaded on first use
    // and kept, since loading it takes about a second.
    private var context: WhisperContext? = null

    override fun transcribe(audio: PcmAudio, language: Language): Flow<TranscriptEvent> = flow {
        // Given silence, Whisper does not return nothing: it invents a
        // plausible sentence. So silence is caught here, before the model
        // runs, which also saves the seconds it would take.
        if (audio.isSilent()) {
            // A different message from "no speech", with the level, so a dead
            // microphone can be told apart from words the model did not catch.
            val level = audio.levelDb().roundToInt()
            throw SilentAudioException("The recording was silent ($level dB). Check the microphone")
        }

        emit(TranscriptEvent.StageChanged(TranscriptionStage.PROCESSING))

        // Whisper does not stream and cannot be interrupted: if the collector
        // is cancelled, this call still runs to its end before the
        // cancellation takes effect.
        val result = withContext(dispatcher) {
            loadedContext().transcribe(audio.toFloatSamples(), language.tag, threads)
        }

        // The second guard, for audio that is loud enough but is not speech
        // (noise, music): the model's own no-speech estimate.
        val noSpeech = (result.noSpeechProbability ?: 0f) > NO_SPEECH_THRESHOLD
        if (noSpeech || result.text.isEmpty() || result.text in SILENCE_MARKERS) {
            throw NoSpeechException(NO_SPEECH)
        }
        emit(
            TranscriptEvent.Final(
                Transcript(result.text, confidence = result.meanTokenProbability, Transcript.Source.ON_DEVICE),
            ),
        )
    }

    /**
     * Loads the model now instead of on the first transcription, so that a
     * timing taken afterwards measures transcribing and not loading.
     */
    suspend fun load() {
        withContext(dispatcher) { loadedContext() }
    }

    /** Frees the model's memory. The next transcription loads it again. */
    suspend fun close() = withContext(dispatcher) {
        context?.close()
        context = null
    }

    private fun loadedContext(): WhisperContext =
        context ?: run {
            val file = modelFile()?.takeIf { it.isFile }
                ?: throw TranscriptionException("The on-device model is not installed")
            WhisperContext.load(file).also { context = it }
        }

    private companion object {
        const val NO_SPEECH = "No speech was recognised"

        // The threshold OpenAI's reference implementation uses.
        const val NO_SPEECH_THRESHOLD = 0.6f

        // What Whisper writes instead of words when it hears none.
        val SILENCE_MARKERS = setOf("[BLANK_AUDIO]", "[ Silence ]", "(silence)")
    }
}
