package com.example.ptranslate.core.pipeline

import com.example.ptranslate.core.Language
import com.example.ptranslate.core.audio.PcmAudio
import com.example.ptranslate.core.stt.Transcriber
import com.example.ptranslate.core.stt.Transcript
import com.example.ptranslate.core.stt.TranscriptEvent
import com.example.ptranslate.core.stt.TranscriptionStage
import kotlinx.coroutines.flow.Flow
import kotlinx.coroutines.flow.map

/**
 * Everything that happens to a recording after it is made. This is the entry
 * point any front end uses: the app's screen today, a keyboard later.
 *
 * It only transcribes for now. Translation joins in milestone 3 and will add
 * events here without changing how callers collect them.
 */
class SpeechTranslationPipeline(private val transcriber: Transcriber) {

    fun process(audio: PcmAudio, source: Language): Flow<PipelineEvent> =
        transcriber.transcribe(audio, source).map { event ->
            when (event) {
                is TranscriptEvent.StageChanged -> PipelineEvent.Transcribing(event.stage)
                is TranscriptEvent.Partial -> PipelineEvent.PartialTranscript(event.text)
                is TranscriptEvent.Final -> PipelineEvent.Transcribed(event.transcript)
            }
        }
}

sealed interface PipelineEvent {
    data class Transcribing(val stage: TranscriptionStage) : PipelineEvent
    data class PartialTranscript(val text: String) : PipelineEvent
    data class Transcribed(val transcript: Transcript) : PipelineEvent
}
