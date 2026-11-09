package com.example.ptranslate.core.net

import com.example.ptranslate.core.Language
import kotlinx.coroutines.test.runTest
import okhttp3.OkHttpClient
import okhttp3.mockwebserver.MockResponse
import okhttp3.mockwebserver.MockWebServer
import org.junit.After
import org.junit.Assert.assertEquals
import org.junit.Assert.assertNull
import org.junit.Test

/** The same cases as the iOS tests. */
class BackendClientTest {
    private val server = MockWebServer()

    @After
    fun tearDown() = server.shutdown()

    private fun accepted(body: String) = MockResponse().setResponseCode(202).setHeader("Content-Type", "application/json").setBody(body)

    @Test
    fun `transcribing and speaking carry the session token`() = runTest {
        server.enqueue(accepted("""{"id":"j1","status":"queued","events_url":"/v1/jobs/j1/events"}"""))
        server.enqueue(accepted("""{"id":"j2","status":"queued","events_url":"/v1/jobs/j2/events","audio_url":"/v1/jobs/j2/audio"}"""))
        val client = BackendClient(server.url("/").toString(), OkHttpClient(), token = { "tok" })

        client.createJob(byteArrayOf(1, 2, 3), Language("en"), idempotencyKey = "k1")
        client.createSpeech(byteArrayOf(1, 2, 3), "hola", Language("es"), idempotencyKey = "k2")

        assertEquals("Bearer tok", server.takeRequest().getHeader("Authorization"))
        assertEquals("Bearer tok", server.takeRequest().getHeader("Authorization"))
    }

    @Test
    fun `with no one signed in nothing pretends to be a token`() = runTest {
        server.enqueue(accepted("""{"id":"j1","status":"queued","events_url":"/v1/jobs/j1/events"}"""))
        val client = BackendClient(server.url("/").toString(), OkHttpClient(), token = { null })

        client.createJob(byteArrayOf(1, 2, 3), Language("en"), idempotencyKey = "k1")

        assertNull(server.takeRequest().getHeader("Authorization"))
    }
}
