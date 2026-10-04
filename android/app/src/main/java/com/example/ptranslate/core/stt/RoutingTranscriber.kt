package com.example.ptranslate.core.stt

import com.example.ptranslate.core.Language
import com.example.ptranslate.core.audio.PcmAudio
import kotlinx.coroutines.flow.Flow
import kotlinx.coroutines.flow.StateFlow

enum class TranscriptionRoute { ON_DEVICE, CLOUD }

/**
 * Sends each recording to one of two transcribers.
 *
 * For now the route is whatever [route] holds, set by a switch on screen.
 * Milestone 6 adds the automatic policy (model missing, device too slow, low
 * confidence) in this class, without touching its callers.
 */
class RoutingTranscriber(
    private val onDevice: Transcriber,
    private val cloud: Transcriber,
    private val route: StateFlow<TranscriptionRoute>,
) : Transcriber {

    override fun transcribe(audio: PcmAudio, language: Language): Flow<TranscriptEvent> =
        when (route.value) {
            TranscriptionRoute.ON_DEVICE -> onDevice.transcribe(audio, language)
            TranscriptionRoute.CLOUD -> cloud.transcribe(audio, language)
        }
}
