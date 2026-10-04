package com.example.ptranslate.core.stt

import com.example.ptranslate.core.Language
import com.example.ptranslate.core.audio.PcmAudio
import kotlinx.coroutines.flow.Flow

/**
 * Turns speech into text. The app depends on this interface only; whether the
 * work happens on the device or on the backend is an implementation detail.
 */
interface Transcriber {
    /**
     * A cold flow: nothing happens until it is collected. It ends after one
     * [TranscriptEvent.Final], or fails with [TranscriptionException].
     */
    fun transcribe(audio: PcmAudio, language: Language): Flow<TranscriptEvent>
}

sealed interface TranscriptEvent {
    data class StageChanged(val stage: TranscriptionStage) : TranscriptEvent

    /** The whole text recognised so far, not just the new words. */
    data class Partial(val text: String) : TranscriptEvent

    data class Final(val transcript: Transcript) : TranscriptEvent
}

enum class TranscriptionStage { UPLOADING, QUEUED, PROCESSING }

data class Transcript(
    val text: String,
    /** 0 to 1, or null when the source cannot provide one. */
    val confidence: Float?,
    val source: Source,
) {
    enum class Source { ON_DEVICE, CLOUD }
}

class TranscriptionException(message: String, cause: Throwable? = null) : Exception(message, cause)
