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

enum class TranscriptionStage { UPLOADING, QUEUED, PROCESSING, RECONNECTING }

data class Transcript(
    val text: String,
    /** 0 to 1, or null when the source cannot provide one. */
    val confidence: Float?,
    val source: Source,
    /** Why this came from where it did, when that was not the first choice. */
    val note: RoutingNote? = null,
) {
    enum class Source { ON_DEVICE, CLOUD }
}

/** Why automatic routing did not simply use the device. */
enum class RoutingNote {
    /** Sent to the cloud: there is no model on this device. */
    NO_MODEL,

    /** Sent to the cloud: this device runs the model too slowly. */
    SLOW_DEVICE,

    /** Sent to the cloud: the device's own result was not confident enough. */
    LOW_CONFIDENCE,

    /** Sent to the cloud: the device heard sound but no words. */
    NO_SPEECH_ON_DEVICE,

    /** Sent to the cloud: the on-device model failed to run. */
    ON_DEVICE_FAILED,

    /** Kept the device's result although the cloud was wanted: it could not be reached. */
    CLOUD_UNAVAILABLE,
}

open class TranscriptionException(message: String, cause: Throwable? = null) : Exception(message, cause)

/** Nothing was recorded: a muted or missing microphone. No transcriber can do better. */
class SilentAudioException(message: String) : TranscriptionException(message)

/** There was sound, but this transcriber found no words in it. Another one might. */
class NoSpeechException(message: String) : TranscriptionException(message)
