package com.example.ptranslate.core.stt

import com.example.ptranslate.core.Language
import com.example.ptranslate.core.audio.PcmAudio
import kotlinx.coroutines.flow.Flow
import kotlinx.coroutines.flow.FlowCollector
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.flow

enum class TranscriptionRoute {
    /** Use the device when it can do the job well, the cloud when it cannot. */
    AUTO,
    ON_DEVICE,
    CLOUD,
}

/**
 * The numbers the automatic route decides by. They are starting points chosen
 * without measurements from real phones; see ADR 0007 before trusting them.
 */
data class FallbackThresholds(
    /** A device result with confidence below this is sent to the cloud as well. */
    val minConfidence: Float = 0.6f,
    /**
     * A device slower than this is not used. The real-time factor is time
     * taken divided by audio length: at 1.0, ten seconds of speech take ten
     * seconds to transcribe.
     */
    val maxRealTimeFactor: Float = 1.0f,
)

/**
 * Sends each recording to the device, the cloud, or both in turn.
 *
 * [TranscriptionRoute.ON_DEVICE] and [TranscriptionRoute.CLOUD] do exactly
 * that. [TranscriptionRoute.AUTO] prefers the device, because it is private,
 * free and works offline, and falls back to the cloud in three cases:
 *
 * 1. There is no model on the device.
 * 2. The device is too slow, by [realTimeFactor].
 * 3. The device tried and the result is poor: low confidence, no words found,
 *    or the model failed.
 *
 * The cloud is a fallback, not a requirement. Whenever it cannot be reached
 * and the device has, or can produce, a result, that result is used.
 *
 * @param modelReady whether the on-device model is installed.
 * @param realTimeFactor this device's measured speed, or null if unmeasured.
 */
class RoutingTranscriber(
    private val onDevice: Transcriber,
    private val cloud: Transcriber,
    private val route: StateFlow<TranscriptionRoute>,
    private val modelReady: () -> Boolean,
    private val realTimeFactor: () -> Float?,
    private val thresholds: FallbackThresholds = FallbackThresholds(),
) : Transcriber {

    override fun transcribe(audio: PcmAudio, language: Language): Flow<TranscriptEvent> =
        when (route.value) {
            TranscriptionRoute.ON_DEVICE -> onDevice.transcribe(audio, language)
            TranscriptionRoute.CLOUD -> cloud.transcribe(audio, language)
            TranscriptionRoute.AUTO -> automatic(audio, language)
        }

    private fun automatic(audio: PcmAudio, language: Language): Flow<TranscriptEvent> = flow {
        if (!modelReady()) {
            // Nothing to fall back on: if the cloud fails, so does this.
            forward(cloud.transcribe(audio, language), RoutingNote.NO_MODEL)
            return@flow
        }

        val speed = realTimeFactor()
        if (speed != null && speed > thresholds.maxRealTimeFactor) {
            try {
                forward(cloud.transcribe(audio, language), RoutingNote.SLOW_DEVICE)
                return@flow
            } catch (e: TranscriptionException) {
                // Offline, most likely. Slow is better than nothing.
                forward(onDevice.transcribe(audio, language), RoutingNote.CLOUD_UNAVAILABLE)
                return@flow
            }
        }

        // The usual case: try the device, and judge what it produced.
        var local: Transcript? = null
        var localFailure: TranscriptionException? = null
        try {
            onDevice.transcribe(audio, language).collect { event ->
                // The final result is held back until it has been judged.
                if (event is TranscriptEvent.Final) local = event.transcript else emit(event)
            }
        } catch (e: SilentAudioException) {
            throw e // nothing was recorded; uploading silence helps nobody
        } catch (e: TranscriptionException) {
            localFailure = e
        }

        val result = local
        val reason = when {
            localFailure is NoSpeechException -> RoutingNote.NO_SPEECH_ON_DEVICE
            localFailure != null || result == null -> RoutingNote.ON_DEVICE_FAILED
            // No confidence figure counts as confident: there is nothing to doubt it with.
            (result.confidence ?: 1f) >= thresholds.minConfidence -> {
                emit(TranscriptEvent.Final(result))
                return@flow
            }
            else -> RoutingNote.LOW_CONFIDENCE
        }

        // Show the doubtful text while the cloud works, rather than nothing.
        if (result != null) emit(TranscriptEvent.Partial(result.text))

        try {
            forward(cloud.transcribe(audio, language), reason)
        } catch (e: TranscriptionException) {
            // The cloud could not help. A doubtful result still beats an
            // error; with no result at all, report what the device said.
            if (result == null) throw localFailure ?: e
            emit(TranscriptEvent.Final(result.copy(note = RoutingNote.CLOUD_UNAVAILABLE)))
        }
    }

    /** Passes a transcriber's events on, marking its final result with [note]. */
    private suspend fun FlowCollector<TranscriptEvent>.forward(events: Flow<TranscriptEvent>, note: RoutingNote) {
        events.collect { event ->
            emit(if (event is TranscriptEvent.Final) TranscriptEvent.Final(event.transcript.copy(note = note)) else event)
        }
    }
}
