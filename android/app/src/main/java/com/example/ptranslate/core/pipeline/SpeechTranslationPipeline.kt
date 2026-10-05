package com.example.ptranslate.core.pipeline

import com.example.ptranslate.core.Language
import com.example.ptranslate.core.audio.PcmAudio
import com.example.ptranslate.core.stt.Transcriber
import com.example.ptranslate.core.stt.Transcript
import com.example.ptranslate.core.stt.TranscriptEvent
import com.example.ptranslate.core.stt.TranscriptionStage
import com.example.ptranslate.core.translate.Translator
import kotlinx.coroutines.flow.Flow
import kotlinx.coroutines.flow.flow

/**
 * Everything that happens to a recording after it is made: transcribe it,
 * then translate the transcript. This is the entry point any front end uses:
 * the app's screen today, a keyboard later.
 */
class SpeechTranslationPipeline(
    private val transcriber: Transcriber,
    private val translator: Translator,
) {

    /**
     * A cold flow that ends after [PipelineEvent.Translated]. It fails with
     * TranscriptionException or TranslationException; when translation fails
     * the transcript has already been emitted, so it is not lost.
     */
    fun process(audio: PcmAudio, source: Language, target: Language): Flow<PipelineEvent> = flow {
        var transcript: Transcript? = null
        transcriber.transcribe(audio, source).collect { event ->
            when (event) {
                is TranscriptEvent.StageChanged -> emit(PipelineEvent.Transcribing(event.stage))
                is TranscriptEvent.Partial -> emit(PipelineEvent.PartialTranscript(event.text))
                is TranscriptEvent.Final -> {
                    transcript = event.transcript
                    emit(PipelineEvent.Transcribed(event.transcript))
                }
            }
        }
        // A transcriber always ends with Final or throws, so this is set.
        val text = checkNotNull(transcript) { "the transcriber finished without a transcript" }.text

        if (source == target) {
            emit(PipelineEvent.Translated(text))
            return@flow
        }

        if (!translator.isReady(source, target)) {
            emit(PipelineEvent.DownloadingLanguages)
            translator.prepare(source, target)
        }
        emit(PipelineEvent.Translating)
        emit(PipelineEvent.Translated(translator.translate(text, source, target)))
    }
}

sealed interface PipelineEvent {
    data class Transcribing(val stage: TranscriptionStage) : PipelineEvent
    data class PartialTranscript(val text: String) : PipelineEvent
    data class Transcribed(val transcript: Transcript) : PipelineEvent

    /** A language pack is being fetched; this can take a while the first time. */
    data object DownloadingLanguages : PipelineEvent
    data object Translating : PipelineEvent
    data class Translated(val text: String) : PipelineEvent
}
