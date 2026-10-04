package com.example.ptranslate.core.stt

import com.example.ptranslate.core.Language
import com.example.ptranslate.core.audio.PcmAudio
import com.example.ptranslate.core.net.BackendClient
import kotlinx.coroutines.flow.toList
import kotlinx.coroutines.runBlocking
import okhttp3.OkHttpClient
import okhttp3.mockwebserver.MockResponse
import okhttp3.mockwebserver.MockWebServer
import org.junit.After
import org.junit.Assert.assertEquals
import org.junit.Assert.assertThrows
import org.junit.Assert.assertTrue
import org.junit.Before
import org.junit.Test

/**
 * Runs the real [BackendClient] against a local fake server, so the HTTP
 * requests and the SSE parsing are tested as they go over the wire.
 */
class RemoteTranscriberTest {

    private lateinit var server: MockWebServer
    private lateinit var transcriber: RemoteTranscriber

    private val audio = PcmAudio(ShortArray(1600))

    @Before
    fun setUp() {
        server = MockWebServer()
        server.start()
        transcriber = RemoteTranscriber(BackendClient(server.url("/").toString(), OkHttpClient()))
    }

    @After
    fun tearDown() {
        server.shutdown()
    }

    private fun enqueueJobAccepted() {
        server.enqueue(
            MockResponse()
                .setResponseCode(202)
                .setHeader("Content-Type", "application/json")
                .setBody("""{"id":"job-1","status":"queued","events_url":"/v1/jobs/job-1/events"}"""),
        )
    }

    private fun enqueueEventStream(body: String) {
        server.enqueue(
            MockResponse()
                .setHeader("Content-Type", "text/event-stream")
                .setBody(body),
        )
    }

    @Test
    fun `maps the job's events to transcript events`() = runBlocking {
        enqueueJobAccepted()
        enqueueEventStream(
            """
            |id: 1
            |event: queued
            |data: {}
            |
            |id: 2
            |event: processing
            |data: {"attempt": 1}
            |
            |: keep-alive
            |
            |id: 3
            |event: partial
            |data: {"text": "hello"}
            |
            |id: 4
            |event: partial
            |data: {"text": "hello world"}
            |
            |id: 5
            |event: done
            |data: {"text": "hello world", "language": "en"}
            |
            |
            """.trimMargin(),
        )

        val events = transcriber.transcribe(audio, Language.ENGLISH).toList()

        assertEquals(
            listOf(
                TranscriptEvent.StageChanged(TranscriptionStage.UPLOADING),
                TranscriptEvent.StageChanged(TranscriptionStage.QUEUED),
                TranscriptEvent.StageChanged(TranscriptionStage.PROCESSING),
                TranscriptEvent.Partial("hello"),
                TranscriptEvent.Partial("hello world"),
                TranscriptEvent.Final(Transcript("hello world", confidence = null, Transcript.Source.CLOUD)),
            ),
            events,
        )
    }

    @Test
    fun `uploads the audio as a WAV file with the source language`() = runBlocking {
        enqueueJobAccepted()
        enqueueEventStream("id: 1\nevent: done\ndata: {\"text\": \"ok\"}\n\n")

        transcriber.transcribe(audio, Language("yo")).toList()

        val upload = server.takeRequest()
        assertEquals("POST", upload.method)
        assertEquals("/v1/jobs", upload.path)
        assertTrue(upload.getHeader("Content-Type").orEmpty().startsWith("multipart/form-data"))
        val body = upload.body.readByteString().utf8()
        assertTrue("source_lang field", body.contains("name=\"source_lang\"") && body.contains("\r\n\r\nyo\r\n"))
        assertTrue("audio part", body.contains("name=\"audio\""))
        assertTrue("WAV signature", body.contains("RIFF") && body.contains("WAVE"))

        val stream = server.takeRequest()
        assertEquals("GET", stream.method)
        assertEquals("/v1/jobs/job-1/events", stream.path)
    }

    @Test
    fun `skips event types it does not know`() = runBlocking {
        enqueueJobAccepted()
        enqueueEventStream(
            "id: 1\nevent: something-new\ndata: {\"x\": 1}\n\n" +
                "id: 2\nevent: done\ndata: {\"text\": \"ok\"}\n\n",
        )

        val events = transcriber.transcribe(audio, Language.ENGLISH).toList()

        assertEquals(2, events.size) // UPLOADING and the final transcript
        assertTrue(events.last() is TranscriptEvent.Final)
    }

    @Test
    fun `fails with the server's message when the job fails`() {
        enqueueJobAccepted()
        enqueueEventStream(
            "id: 1\nevent: queued\ndata: {}\n\n" +
                "id: 2\nevent: error\ndata: {\"message\": \"provider exploded\"}\n\n",
        )

        val error = assertThrows(TranscriptionException::class.java) {
            runBlocking { transcriber.transcribe(audio, Language.ENGLISH).toList() }
        }

        assertEquals("provider exploded", error.message)
    }

    @Test
    fun `fails with the server's message when the upload is rejected`() {
        server.enqueue(
            MockResponse()
                .setResponseCode(400)
                .setBody("""{"error":{"code":"unsupported_audio","message":"audio must be a WAV file"}}"""),
        )

        val error = assertThrows(TranscriptionException::class.java) {
            runBlocking { transcriber.transcribe(audio, Language.ENGLISH).toList() }
        }

        assertEquals("audio must be a WAV file", error.message)
    }

    @Test
    fun `fails when the stream ends before the transcript arrives`() {
        enqueueJobAccepted()
        enqueueEventStream("id: 1\nevent: queued\ndata: {}\n\n")

        val error = assertThrows(TranscriptionException::class.java) {
            runBlocking { transcriber.transcribe(audio, Language.ENGLISH).toList() }
        }

        assertEquals("The connection closed before the transcript arrived", error.message)
    }

    @Test
    fun `fails when the server cannot be reached`() {
        server.shutdown()

        val error = assertThrows(TranscriptionException::class.java) {
            runBlocking { transcriber.transcribe(audio, Language.ENGLISH).toList() }
        }

        assertTrue(error.message.orEmpty().startsWith("Could not reach the server"))
    }
}
