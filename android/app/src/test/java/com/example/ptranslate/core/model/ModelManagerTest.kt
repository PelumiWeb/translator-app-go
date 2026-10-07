package com.example.ptranslate.core.model

import com.example.ptranslate.core.net.BackendClient
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.flow.toList
import kotlinx.coroutines.launch
import kotlinx.coroutines.runBlocking
import okhttp3.OkHttpClient
import okhttp3.mockwebserver.Dispatcher
import okhttp3.mockwebserver.MockResponse
import okhttp3.mockwebserver.MockWebServer
import okhttp3.mockwebserver.RecordedRequest
import okhttp3.mockwebserver.SocketPolicy
import okio.Buffer
import org.junit.After
import org.junit.Assert.assertArrayEquals
import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertNull
import org.junit.Assert.assertThrows
import org.junit.Assert.assertTrue
import org.junit.Before
import org.junit.Rule
import org.junit.Test
import org.junit.rules.TemporaryFolder
import java.io.File
import java.security.MessageDigest

/**
 * Runs the real [BackendClient] and real files against a local fake server
 * that behaves like the Go one: a manifest, and a model endpoint that honours
 * Range and If-Range.
 */
class ModelManagerTest {

    @get:Rule
    val folder = TemporaryFolder()

    private lateinit var server: MockWebServer
    private lateinit var directory: File

    // 600 KB of non-repeating bytes: enough for several progress updates.
    private val model = ByteArray(600_000) { (it * 31 + it / 7).toByte() }
    private val modelSha = sha256(model)

    /** What the fake server does; tests swap parts of it. */
    private var manifestSha = modelSha
    private var served = model
    private var honourRanges = true
    private var dropAfterBytes: Int? = null
    private val modelRequests = mutableListOf<RecordedRequest>()

    @Before
    fun setUp() {
        directory = folder.newFolder("models")
        server = MockWebServer()
        server.dispatcher = object : Dispatcher() {
            override fun dispatch(request: RecordedRequest): MockResponse = when (request.path) {
                "/v1/models/manifest" -> MockResponse().setBody(
                    """{"models":[{"id":"base","version":"v1","size":${model.size},""" +
                        """"sha256":"$manifestSha","url":"/v1/models/base"}]}""",
                )
                "/v1/models/base" -> {
                    modelRequests += request
                    serveModel(request)
                }
                else -> MockResponse().setResponseCode(404)
            }
        }
        server.start()
    }

    @After
    fun tearDown() {
        server.shutdown()
    }

    private fun serveModel(request: RecordedRequest): MockResponse {
        val offset = request.getHeader("Range")?.removePrefix("bytes=")?.removeSuffix("-")?.toInt()
        val sameFile = request.getHeader("If-Range") == "\"${sha256(served)}\""

        val response = if (honourRanges && offset != null && sameFile) {
            MockResponse()
                .setResponseCode(206)
                .setHeader("Content-Range", "bytes $offset-${served.size - 1}/${served.size}")
                .setBody(Buffer().write(served, offset, served.size - offset))
        } else {
            MockResponse().setBody(Buffer().write(served))
        }

        dropAfterBytes?.let { limit ->
            // Send only the first part of the body, then cut the connection.
            response.setBody(Buffer().write(served, 0, limit))
            response.setHeader("Content-Length", served.size)
            response.socketPolicy = SocketPolicy.DISCONNECT_DURING_RESPONSE_BODY
        }
        return response
    }

    private fun manager() = ModelManager(
        backend = BackendClient(server.url("/").toString(), OkHttpClient()),
        directory = directory,
        modelId = "base",
        ioDispatcher = Dispatchers.IO,
    )

    private val installed get() = File(directory, "base.bin")
    private val partial get() = File(directory, "base.part")
    private val partialSha get() = File(directory, "base.part.sha256")

    private fun ensureFails(manager: ModelManager): ModelException =
        assertThrows(ModelException::class.java) { runBlocking { manager.ensureInstalled() } }

    @Test
    fun `downloads, verifies and installs the model`() = runBlocking {
        val manager = manager()
        assertEquals(ModelState.Missing, manager.state.value)

        val file = manager.ensureInstalled()

        assertEquals(installed, file)
        assertArrayEquals(model, installed.readBytes())
        assertEquals(ModelState.Ready(installed), manager.state.value)
        assertFalse("partial file left behind", partial.exists() || partialSha.exists())
        assertNull("a fresh download must not ask for a range", modelRequests.single().getHeader("Range"))
    }

    @Test
    fun `rejects a download whose checksum does not match the manifest`() {
        manifestSha = "0".repeat(64)
        val manager = manager()

        val error = ensureFails(manager)

        assertEquals("The downloaded model is corrupt (checksum mismatch)", error.message)
        assertEquals(ModelState.Failed("The downloaded model is corrupt (checksum mismatch)"), manager.state.value)
        assertFalse("a corrupt model must never be installed", installed.exists())
        assertFalse("a corrupt download must not be kept for resuming", partial.exists())
    }

    @Test
    fun `resumes a partial download from where it stopped`() = runBlocking {
        partial.writeBytes(model.copyOf(200_000))
        partialSha.writeText(modelSha)

        manager().ensureInstalled()

        val request = modelRequests.single()
        assertEquals("bytes=200000-", request.getHeader("Range"))
        assertEquals("\"$modelSha\"", request.getHeader("If-Range"))
        assertArrayEquals(model, installed.readBytes())
    }

    @Test
    fun `starts over when the server ignores the range and sends everything`() = runBlocking {
        honourRanges = false
        partial.writeBytes(model.copyOf(200_000))
        partialSha.writeText(modelSha)

        manager().ensureInstalled()

        // Had the full body been appended to the 200 KB, the file would be
        // 800 KB and fail its checksum.
        assertArrayEquals(model, installed.readBytes())
    }

    @Test
    fun `discards a partial download that belongs to a different version`() = runBlocking {
        partial.writeBytes(ByteArray(200_000) { 7 }) // the start of some older model
        partialSha.writeText("f".repeat(64))

        manager().ensureInstalled()

        assertNull("must not resume onto another file's bytes", modelRequests.single().getHeader("Range"))
        assertArrayEquals(model, installed.readBytes())
    }

    @Test
    fun `discards a partial download with no record of what it belongs to`() = runBlocking {
        partial.writeBytes(model.copyOf(200_000))

        manager().ensureInstalled()

        assertNull(modelRequests.single().getHeader("Range"))
        assertArrayEquals(model, installed.readBytes())
    }

    @Test
    fun `a dropped connection keeps the partial file, and the next attempt finishes it`() = runBlocking {
        val manager = manager()
        dropAfterBytes = 250_000

        val error = ensureFails(manager)

        assertEquals("The download was interrupted", error.message)
        assertFalse(installed.exists())
        val kept = partial.length()
        assertTrue("kept $kept bytes", kept in 1..250_000)

        dropAfterBytes = null
        manager.ensureInstalled()

        assertEquals("bytes=$kept-", modelRequests.last().getHeader("Range"))
        assertArrayEquals(model, installed.readBytes())
        assertEquals(ModelState.Ready(installed), manager.state.value)
    }

    @Test
    fun `a fully downloaded but unverified file is verified without downloading again`() = runBlocking {
        partial.writeBytes(model)
        partialSha.writeText(modelSha)

        manager().ensureInstalled()

        assertTrue("no model request expected", modelRequests.isEmpty())
        assertArrayEquals(model, installed.readBytes())
    }

    @Test
    fun `an installed model is used without contacting the server`() = runBlocking {
        installed.writeBytes(model)
        server.shutdown() // offline

        val manager = manager()

        assertEquals(ModelState.Ready(installed), manager.state.value)
        assertEquals(installed, manager.ensureInstalled())
    }

    @Test
    fun `fails when the server does not offer the model`() {
        val manager = ModelManager(
            BackendClient(server.url("/").toString(), OkHttpClient()),
            directory,
            modelId = "large",
            ioDispatcher = Dispatchers.IO,
        )

        assertEquals("The server does not offer the model large", ensureFails(manager).message)
    }

    @Test
    fun `fails when the server cannot be reached`() {
        server.shutdown()

        val error = ensureFails(manager())

        assertTrue(error.message.orEmpty().startsWith("Could not reach the server"))
        assertFalse(installed.exists())
    }

    @Test
    fun `reports progress, then verifying, then ready`() = runBlocking {
        val manager = manager()
        val seen = mutableListOf<ModelState>()
        // Unconfined, so the collector records each state as it is set.
        val collector = launch(Dispatchers.Unconfined) { manager.state.toList(seen) }

        manager.ensureInstalled()
        collector.cancel()

        val progress = seen.filterIsInstance<ModelState.Downloading>()
        assertTrue("progress updates: ${progress.size}", progress.isNotEmpty())
        assertTrue("progress never goes backwards", progress.zipWithNext().all { (a, b) -> a.bytes <= b.bytes })
        assertTrue("totals are the model size", progress.all { it.total == model.size.toLong() })
        assertEquals(ModelState.Missing, seen.first())
        assertEquals(ModelState.Ready(installed), seen.last())
    }

    private fun sha256(bytes: ByteArray): String =
        MessageDigest.getInstance("SHA-256").digest(bytes).joinToString("") { "%02x".format(it) }
}
