package com.example.ptranslate.core.speech

import com.example.ptranslate.core.Language
import com.example.ptranslate.core.audio.AudioPlayer
import com.example.ptranslate.core.audio.PcmAudio
import com.example.ptranslate.core.audio.toWav
import com.example.ptranslate.core.net.BackendClient
import com.example.ptranslate.core.voice.VoiceSampleState
import com.example.ptranslate.core.voice.VoiceSampleStore
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.runBlocking
import okhttp3.OkHttpClient
import okhttp3.mockwebserver.MockResponse
import okhttp3.mockwebserver.MockWebServer
import okhttp3.mockwebserver.SocketPolicy
import okio.Buffer
import org.junit.After
import org.junit.Assert.assertArrayEquals
import org.junit.Assert.assertEquals
import org.junit.Assert.assertThrows
import org.junit.Assert.assertTrue
import org.junit.Before
import org.junit.Test

/**
 * Runs the real [BackendClient] against a local fake server, so the requests,
 * the event stream and the audio download are tested as they go over the wire.
 */
class RemoteSpeechSynthesizerTest {

    private class FakePlayer : AudioPlayer {
        val played = mutableListOf<PcmAudio>()
        var stops = 0

        override suspend fun play(audio: PcmAudio) {
            played += audio
        }

        override fun stop() {
            stops++
        }
    }

    private class FakeStore(private val wav: ByteArray?) : VoiceSampleStore {
        override val state = MutableStateFlow<VoiceSampleState>(VoiceSampleState.None)
        override suspend fun save(audio: PcmAudio) = Unit
        override suspend fun loadWav(): ByteArray? = wav
        override suspend fun delete() = Unit
    }

    private lateinit var server: MockWebServer
    private val player = FakePlayer()

    /** What the user recorded at onboarding. */
    private val sample = PcmAudio(ShortArray(16_000) { (it % 100).toShort() }).toWav()

    /** What the server makes from it. */
    private val spoken = PcmAudio(ShortArray(8_000) { (it % 50).toShort() }, sampleRate = 22_050)

    @Before
    fun setUp() {
        server = MockWebServer()
        server.start()
    }

    @After
    fun tearDown() {
        server.shutdown()
    }

    private fun synthesizer(wav: ByteArray? = sample) = RemoteSpeechSynthesizer(
        BackendClient(server.url("/").toString(), OkHttpClient()),
        FakeStore(wav),
        player,
        maxAttempts = 3,
        retryDelayMs = { 0 }, // no waiting in tests
    )

    private fun enqueueSpeechAccepted() {
        server.enqueue(
            MockResponse()
                .setResponseCode(202)
                .setHeader("Content-Type", "application/json")
                .setBody(
                    """{"id":"job-1","status":"queued","events_url":"/v1/jobs/job-1/events",""" +
                        """"audio_url":"/v1/jobs/job-1/audio"}""",
                ),
        )
    }

    private fun enqueueEventStream(body: String) {
        server.enqueue(MockResponse().setHeader("Content-Type", "text/event-stream").setBody(body))
    }

    private fun enqueueAudio() {
        server.enqueue(MockResponse().setHeader("Content-Type", "audio/wav").setBody(Buffer().write(spoken.toWav())))
    }

    private fun sse(seq: Int, type: String, data: String = "{}") = "id: $seq\nevent: $type\ndata: $data\n\n"

    /** How many sounds the player had already been given each time the speech said it started, and in what voice. */
    private val startedAfter = mutableListOf<Int>()
    private val startedIn = mutableListOf<SpeechVoice>()

    private fun speak(synthesizer: RemoteSpeechSynthesizer = synthesizer()) =
        runBlocking {
            synthesizer.speak("buenos días", Language("es")) { voice ->
                startedAfter += player.played.size
                startedIn += voice
            }
        }

    private fun speakFails(synthesizer: RemoteSpeechSynthesizer = synthesizer()): SpeechException =
        assertThrows(SpeechException::class.java) { speak(synthesizer) }

    @Test
    fun `sends the voice sample and the text, then plays what the server made`() {
        enqueueSpeechAccepted()
        enqueueEventStream(sse(1, "queued") + sse(2, "processing") + sse(3, "done"))
        enqueueAudio()

        speak()

        val upload = server.takeRequest()
        assertEquals("POST /v1/speech HTTP/1.1", upload.requestLine)
        assertTrue("an idempotency key is sent", !upload.getHeader("Idempotency-Key").isNullOrBlank())
        val form = upload.body.readUtf8()
        assertTrue(form.contains("name=\"text\"") && form.contains("buenos días"))
        assertTrue(form.contains("name=\"lang\"") && form.contains("\r\nes\r\n"))
        assertTrue(form.contains("name=\"voice\"; filename=\"voice.wav\""))
        assertEquals("/v1/jobs/job-1/events", server.takeRequest().path)
        assertEquals("/v1/jobs/job-1/audio", server.takeRequest().path)

        val played = player.played.single()
        assertArrayEquals(spoken.samples, played.samples)
        assertEquals(22_050, played.sampleRate)
        // Reported as started once, just before it plays: not while the voice is being made.
        assertEquals(listOf(0), startedAfter)
        assertEquals(listOf(SpeechVoice.OWN), startedIn)
    }

    @Test
    fun `reconnects to the event stream and still plays the result`() {
        enqueueSpeechAccepted()
        enqueueEventStream(sse(1, "queued"))
        server.enqueue(MockResponse().setSocketPolicy(SocketPolicy.DISCONNECT_AT_START))
        enqueueEventStream(sse(2, "processing") + sse(3, "done"))
        enqueueAudio()

        speak()

        assertEquals(1, player.played.size)
    }

    @Test
    fun `retries an upload after a server fault, with the same idempotency key`() {
        server.enqueue(MockResponse().setResponseCode(503))
        enqueueSpeechAccepted()
        enqueueEventStream(sse(1, "done"))
        enqueueAudio()

        speak()

        val firstKey = server.takeRequest().getHeader("Idempotency-Key")
        val secondKey = server.takeRequest().getHeader("Idempotency-Key")
        assertEquals(firstKey, secondKey)
        assertEquals(1, player.played.size)
    }

    @Test
    fun `a job that failed is reported in words and nothing is played`() {
        enqueueSpeechAccepted()
        enqueueEventStream(sse(1, "queued") + sse(2, "error", """{"message": "the voice engine is down"}"""))

        val error = speakFails()

        assertEquals("the voice engine is down", error.message)
        assertEquals(0, player.played.size)
        assertEquals(emptyList<Int>(), startedAfter)
    }

    @Test
    fun `a request the server refused is reported with its reason`() {
        server.enqueue(
            MockResponse()
                .setResponseCode(400)
                .setBody("""{"error":{"code":"text_too_long","message":"the text is too long to speak in one request"}}"""),
        )

        val error = speakFails()

        assertEquals("the text is too long to speak in one request", error.message)
        assertEquals(1, server.requestCount)
    }

    @Test
    fun `a server with no voice engine is not asked again`() {
        server.enqueue(
            MockResponse()
                .setResponseCode(501)
                .setBody("""{"error":{"code":"no_voice_engine","message":"this server has no voice engine"}}"""),
        )

        val error = speakFails()

        assertEquals("this server has no voice engine", error.message)
        assertEquals(1, server.requestCount)
    }

    @Test
    fun `with no voice recorded nothing is sent`() {
        val error = speakFails(synthesizer(wav = null))

        assertEquals("No voice has been recorded", error.message)
        assertEquals(0, server.requestCount)
    }

    @Test
    fun `fails when the server cannot be reached`() {
        server.shutdown()

        val error = speakFails()

        assertTrue(error.message.orEmpty().startsWith("Could not reach the server"))
    }

    @Test
    fun `stop stops the playback`() {
        synthesizer().stop()

        assertEquals(1, player.stops)
    }
}
