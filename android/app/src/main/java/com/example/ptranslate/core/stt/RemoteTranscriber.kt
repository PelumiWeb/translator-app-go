package com.example.ptranslate.core.stt

import com.example.ptranslate.core.Language
import com.example.ptranslate.core.audio.PcmAudio
import com.example.ptranslate.core.audio.toWav
import com.example.ptranslate.core.net.BackendClient
import com.example.ptranslate.core.net.BackendException
import com.example.ptranslate.core.net.JobEvent
import kotlinx.coroutines.flow.Flow
import kotlinx.coroutines.flow.flow

/** Transcribes on the backend: uploads the audio, then follows the job over SSE. */
class RemoteTranscriber(private val backend: BackendClient) : Transcriber {

    override fun transcribe(audio: PcmAudio, language: Language): Flow<TranscriptEvent> = flow {
        emit(TranscriptEvent.StageChanged(TranscriptionStage.UPLOADING))

        try {
            val jobId = backend.createJob(audio.toWav(), language)

            var finished = false
            backend.jobEvents(jobId).collect { event ->
                when (event) {
                    JobEvent.Queued ->
                        emit(TranscriptEvent.StageChanged(TranscriptionStage.QUEUED))

                    JobEvent.Processing ->
                        emit(TranscriptEvent.StageChanged(TranscriptionStage.PROCESSING))

                    is JobEvent.Partial ->
                        emit(TranscriptEvent.Partial(event.text))

                    is JobEvent.Done -> {
                        finished = true
                        emit(TranscriptEvent.Final(Transcript(event.text, confidence = null, Transcript.Source.CLOUD)))
                    }

                    is JobEvent.Failed ->
                        throw TranscriptionException(event.message)
                }
            }
            // The server closes the stream after "done" or "error". Closing
            // without either means the connection was cut short.
            if (!finished) throw TranscriptionException("The connection closed before the transcript arrived")
        } catch (e: BackendException) {
            throw TranscriptionException(e.message ?: "The server could not be reached", e)
        }
    }
}
