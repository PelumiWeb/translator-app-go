package com.example.ptranslate.core.stt

import com.example.ptranslate.core.Language
import com.example.ptranslate.core.audio.PcmAudio
import com.example.ptranslate.core.audio.toWav
import com.example.ptranslate.core.net.BackendClient
import com.example.ptranslate.core.net.BackendException
import com.example.ptranslate.core.net.JobEvent
import com.example.ptranslate.core.net.followJob
import com.example.ptranslate.core.net.retrying
import kotlinx.coroutines.flow.Flow
import kotlinx.coroutines.flow.flow
import java.util.UUID

/**
 * Transcribes on the backend: uploads the audio, then follows the job over
 * SSE. Both steps survive a connection that drops, which on a phone is
 * ordinary.
 *
 * @param maxAttempts how many times in a row the upload, or the stream, may
 *   fail before giving up.
 * @param retryDelayMs how long to wait before try number `attempt + 1`.
 */
class RemoteTranscriber(
    private val backend: BackendClient,
    private val maxAttempts: Int = 5,
    private val retryDelayMs: (attempt: Int) -> Long = { attempt -> attempt * 1000L },
) : Transcriber {

    override fun transcribe(audio: PcmAudio, language: Language): Flow<TranscriptEvent> = flow {
        emit(TranscriptEvent.StageChanged(TranscriptionStage.UPLOADING))
        val wav = audio.toWav()
        // Every attempt carries the same key, so if an earlier one did arrive
        // and only its response was lost, the server hands back that job
        // instead of creating a second.
        val idempotencyKey = UUID.randomUUID().toString()
        try {
            val jobId = retrying(maxAttempts, retryDelayMs) { backend.createJob(wav, language, idempotencyKey) }
            backend.followJob(
                jobId,
                maxAttempts,
                retryDelayMs,
                onReconnecting = { emit(TranscriptEvent.StageChanged(TranscriptionStage.RECONNECTING)) },
            ) { event ->
                when (event) {
                    JobEvent.Queued -> emit(TranscriptEvent.StageChanged(TranscriptionStage.QUEUED))
                    JobEvent.Processing -> emit(TranscriptEvent.StageChanged(TranscriptionStage.PROCESSING))
                    is JobEvent.Partial -> emit(TranscriptEvent.Partial(event.text))
                    is JobEvent.Done ->
                        emit(TranscriptEvent.Final(Transcript(event.text, confidence = null, Transcript.Source.CLOUD)))
                    is JobEvent.Failed, JobEvent.Unknown -> Unit
                }
            }
        } catch (e: BackendException) {
            throw TranscriptionException(e.message ?: "The server could not be reached", e)
        }
    }
}
