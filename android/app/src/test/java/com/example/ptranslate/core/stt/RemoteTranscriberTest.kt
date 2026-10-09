package com.example.ptranslate.core.stt

import com.example.ptranslate.core.Language
import com.example.ptranslate.core.audio.PcmAudio
import com.example.ptranslate.core.net.BackendClient
import kotlinx.coroutines.flow.toList
import kotlinx.coroutines.runBlocking
import okhttp3.OkHttpClient
import okhttp3.mockwebserver.MockResponse
import okhttp3.mockwebserver.MockWebServer
import okhttp3.mockwebserver.SocketPolicy
import org.junit.After
import org.junit.Assert.assertEquals
import org.junit.Assert.assertNull
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
        transcriber = RemoteTranscriber(
            BackendClient(server.url("/").toString(), OkHttpClient()),
            maxAttempts = 3,
            retryDelayMs = { 0 }, // no waiting in tests
        )
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

    /** A stream that sends [body] and then closes, as the real server does. */
    private fun enqueueEventStream(body: String) {
        server.enqueue(
            MockResponse()
                .setHeader("Content-Type", "text/event-stream")
                .setBody(body),
        )
    }

    private fun sse(seq: Int, type: String, data: String = "{}") = "id: $seq\nevent: $type\ndata: $data\n\n"

    private fun transcribe(language: Language = Language.ENGLISH): List<TranscriptEvent> =
        runBlocking { transcriber.transcribe(audio, language).toList() }

    private fun transcribeFails(): TranscriptionException =
        assertThrows(TranscriptionException::class.java) { transcribe() }

    private val final = TranscriptEvent.Final(Transcript("hello world", confidence = null, Transcript.Source.CLOUD))

    @Test
    fun `maps the job's events to transcript events`() {
        enqueueJobAccepted()
        enqueueEventStream(
            sse(1, "queued") +
                sse(2, "processing", """{"attempt": 1}""") +
                ": keep-alive\n\n" +
                sse(3, "partial", """{"text": "hello"}""") +
                sse(4, "partial", """{"text": "hello world"}""") +
                sse(5, "done", """{"text": "hello world", "language": "en"}"""),
        )

        assertEquals(
            listOf(
                TranscriptEvent.StageChanged(TranscriptionStage.UPLOADING),
                TranscriptEvent.StageChanged(TranscriptionStage.QUEUED),
                TranscriptEvent.StageChanged(TranscriptionStage.PROCESSING),
                TranscriptEvent.Partial("hello"),
                TranscriptEvent.Partial("hello world"),
                final,
            ),
            transcribe(),
        )
    }

    @Test
    fun `uploads the audio as a WAV file with the source language`() {
        enqueueJobAccepted()
        enqueueEventStream(sse(1, "done", """{"text": "ok"}"""))

        transcribe(Language("yo"))

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
        assertNull("a first connection has nothing to resume from", stream.getHeader("Last-Event-ID"))
    }

    @Test
    fun `skips event types it does not know`() {
        enqueueJobAccepted()
        enqueueEventStream(sse(1, "something-new", """{"x": 1}""") + sse(2, "done", """{"text": "ok"}"""))

        val events = transcribe()

        assertEquals(2, events.size) // UPLOADING and the final transcript
        assertTrue(events.last() is TranscriptEvent.Final)
    }

    // --- reconnecting ---

    @Test
    fun `reconnects from the last event it received and carries on`() {
        enqueueJobAccepted()
        // The connection drops after three events...
        enqueueEventStream(sse(1, "queued") + sse(2, "processing") + sse(3, "partial", """{"text": "hello"}"""))
        // ...and the second connection continues from the fourth.
        enqueueEventStream(
            sse(4, "partial", """{"text": "hello world"}""") +
                sse(5, "done", """{"text": "hello world", "language": "en"}"""),
        )

        val events = transcribe()

        assertEquals(
            listOf(
                TranscriptEvent.StageChanged(TranscriptionStage.UPLOADING),
                TranscriptEvent.StageChanged(TranscriptionStage.QUEUED),
                TranscriptEvent.StageChanged(TranscriptionStage.PROCESSING),
                TranscriptEvent.Partial("hello"),
                TranscriptEvent.StageChanged(TranscriptionStage.RECONNECTING),
                TranscriptEvent.Partial("hello world"),
                final,
            ),
            events,
        )
        server.takeRequest() // the upload
        server.takeRequest() // the first stream
        assertEquals("3", server.takeRequest().getHeader("Last-Event-ID"))
    }

    @Test
    fun `shows nothing twice if the server replays events it already sent`() {
        enqueueJobAccepted()
        enqueueEventStream(sse(1, "queued") + sse(2, "partial", """{"text": "hello"}"""))
        // A server that ignores Last-Event-ID and starts from the beginning.
        enqueueEventStream(
            sse(1, "queued") + sse(2, "partial", """{"text": "hello"}""") +
                sse(3, "done", """{"text": "hello world", "language": "en"}"""),
        )

        val events = transcribe()

        assertEquals(
            listOf(
                TranscriptEvent.StageChanged(TranscriptionStage.UPLOADING),
                TranscriptEvent.StageChanged(TranscriptionStage.QUEUED),
                TranscriptEvent.Partial("hello"),
                TranscriptEvent.StageChanged(TranscriptionStage.RECONNECTING),
                final,
            ),
            events,
        )
    }

    @Test
    fun `reconnects when the connection is cut in the middle of the stream`() {
        enqueueJobAccepted()
        server.enqueue(
            MockResponse()
                .setHeader("Content-Type", "text/event-stream")
                .setBody(sse(1, "queued") + sse(2, "processing"))
                .setSocketPolicy(SocketPolicy.DISCONNECT_DURING_RESPONSE_BODY),
        )
        enqueueEventStream(sse(1, "queued") + sse(2, "processing") + sse(3, "done", """{"text": "hello world"}"""))

        val events = transcribe()

        assertEquals(final, events.last())
        assertEquals(1, events.count { it is TranscriptEvent.Final })
    }

    @Test
    fun `gives up after reconnecting several times without any progress`() {
        enqueueJobAccepted()
        enqueueEventStream(sse(1, "queued"))
        enqueueEventStream("") // maxAttempts is 3: two more tries, both empty
        enqueueEventStream("")

        val error = transcribeFails()

        assertEquals("The connection to the server was lost", error.message)
        assertEquals(4, server.requestCount) // one upload and three streams
    }

    @Test
    fun `keeps going as long as each reconnect brings something new`() {
        enqueueJobAccepted()
        // Five connections of one event each: more than maxAttempts, but none
        // of them a failure without progress.
        enqueueEventStream(sse(1, "queued"))
        enqueueEventStream(sse(2, "processing"))
        enqueueEventStream(sse(3, "partial", """{"text": "hello"}"""))
        enqueueEventStream(sse(4, "partial", """{"text": "hello world"}"""))
        enqueueEventStream(sse(5, "done", """{"text": "hello world"}"""))

        assertEquals(final, transcribe().last())
    }

    @Test
    fun `does not reconnect when the job itself failed`() {
        enqueueJobAccepted()
        enqueueEventStream(sse(1, "queued") + sse(2, "error", """{"message": "provider exploded"}"""))

        val error = transcribeFails()

        assertEquals("provider exploded", error.message)
        assertEquals(2, server.requestCount)
    }

    @Test
    fun `does not reconnect when the server says the job does not exist`() {
        enqueueJobAccepted()
        server.enqueue(MockResponse().setResponseCode(404).setBody("""{"error":{"code":"job_not_found","message":"no such job"}}"""))

        val error = transcribeFails()

        assertEquals("The server answered 404", error.message)
        assertEquals(2, server.requestCount)
    }

    // --- uploading ---

    @Test
    fun `retries an upload whose response was lost, with the same idempotency key`() {
        // The server receives the request, then the connection dies before
        // any response: the client cannot know whether a job was created.
        server.enqueue(MockResponse().setSocketPolicy(SocketPolicy.DISCONNECT_AFTER_REQUEST))
        enqueueJobAccepted()
        enqueueEventStream(sse(1, "done", """{"text": "hello world"}"""))

        assertEquals(final, transcribe().last())

        val firstKey = server.takeRequest().getHeader("Idempotency-Key")
        val secondKey = server.takeRequest().getHeader("Idempotency-Key")
        assertTrue("a key is sent", !firstKey.isNullOrBlank())
        assertEquals("both attempts carry the same key", firstKey, secondKey)
    }

    @Test
    fun `uses a new idempotency key for each recording`() {
        repeat(2) {
            enqueueJobAccepted()
            enqueueEventStream(sse(1, "done", """{"text": "ok"}"""))
            transcribe()
        }

        val firstKey = server.takeRequest().getHeader("Idempotency-Key")
        server.takeRequest() // first stream
        val secondKey = server.takeRequest().getHeader("Idempotency-Key")
        assertTrue(firstKey != secondKey)
    }

    @Test
    fun `retries an upload after a server fault`() {
        server.enqueue(MockResponse().setResponseCode(503))
        enqueueJobAccepted()
        enqueueEventStream(sse(1, "done", """{"text": "hello world"}"""))

        assertEquals(final, transcribe().last())
    }

    @Test
    fun `does not retry an upload the server refused`() {
        server.enqueue(
            MockResponse()
                .setResponseCode(400)
                .setBody("""{"error":{"code":"unsupported_audio","message":"audio must be a WAV file"}}"""),
        )

        val error = transcribeFails()

        assertEquals("audio must be a WAV file", error.message)
        assertEquals(1, server.requestCount)
    }

    @Test
    fun `fails when the server cannot be reached`() {
        server.shutdown()

        val error = transcribeFails()

        assertTrue(error.message.orEmpty().startsWith("Could not reach the server"))
    }
}
