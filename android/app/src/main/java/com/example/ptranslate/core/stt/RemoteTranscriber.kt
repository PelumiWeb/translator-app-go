package com.example.ptranslate.core.stt

import com.example.ptranslate.core.Language
import com.example.ptranslate.core.audio.PcmAudio
import com.example.ptranslate.core.audio.toWav
import com.example.ptranslate.core.net.BackendClient
import com.example.ptranslate.core.net.BackendException
import com.example.ptranslate.core.net.JobEvent
import kotlinx.coroutines.delay
import kotlinx.coroutines.flow.Flow
import kotlinx.coroutines.flow.FlowCollector
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
        val jobId = upload(audio.toWav(), language)
        follow(jobId)
    }

    /**
     * Uploads until the server answers. Every attempt carries the same key,
     * so if an earlier one did arrive and only its response was lost, the
     * server hands back that job instead of creating a second.
     */
    private suspend fun upload(wav: ByteArray, language: Language): String {
        val idempotencyKey = UUID.randomUUID().toString()
        var attempt = 1
        while (true) {
            try {
                return backend.createJob(wav, language, idempotencyKey)
            } catch (e: BackendException) {
                if (!e.retryable || attempt >= maxAttempts) {
                    throw TranscriptionException(e.message ?: "The server could not be reached", e)
                }
            }
            delay(retryDelayMs(attempt))
            attempt++
        }
    }

    /**
     * Follows the job to its end, reconnecting when the stream breaks. Each
     * reconnect tells the server the last event already received, and the
     * server continues from the next one, so nothing is repeated or skipped.
     */
    private suspend fun FlowCollector<TranscriptEvent>.follow(jobId: String) {
        var lastSeq = 0
        var failures = 0 // in a row, without any new event in between

        while (true) {
            var finished = false
            val seqBefore = lastSeq
            try {
                backend.jobEvents(jobId, afterSeq = lastSeq).collect { update ->
                    // A guard for a server that replays more than it was
                    // asked to: never show the same event twice.
                    if (update.seq <= lastSeq) return@collect
                    lastSeq = update.seq

                    when (val event = update.event) {
                        JobEvent.Queued -> emit(TranscriptEvent.StageChanged(TranscriptionStage.QUEUED))
                        JobEvent.Processing -> emit(TranscriptEvent.StageChanged(TranscriptionStage.PROCESSING))
                        is JobEvent.Partial -> emit(TranscriptEvent.Partial(event.text))
                        is JobEvent.Done -> {
                            finished = true
                            emit(TranscriptEvent.Final(Transcript(event.text, confidence = null, Transcript.Source.CLOUD)))
                        }
                        // The job itself failed. Reconnecting would only
                        // replay the same ending.
                        is JobEvent.Failed -> throw TranscriptionException(event.message)
                        JobEvent.Unknown -> Unit
                    }
                }
            } catch (e: BackendException) {
                if (!e.retryable) throw TranscriptionException(e.message ?: "The server refused the request", e)
            }
            if (finished) return

            // The stream ended, cleanly or not, before the job did. The
            // server closes a stream only after "done" or "error", so this
            // was the connection, a server restart, or a client that fell
            // behind. In every case the answer is the same: ask again.
            if (lastSeq > seqBefore) failures = 0 // it was making progress
            failures++
            if (failures >= maxAttempts) {
                throw TranscriptionException("The connection to the server was lost")
            }
            emit(TranscriptEvent.StageChanged(TranscriptionStage.RECONNECTING))
            delay(retryDelayMs(failures))
        }
    }
}
