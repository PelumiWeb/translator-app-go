package com.example.ptranslate.core.rooms

import com.example.ptranslate.core.net.Rest
import com.example.ptranslate.core.net.RoomsClient
import kotlinx.coroutines.flow.take
import kotlinx.coroutines.flow.toList
import kotlinx.coroutines.test.runTest
import okhttp3.OkHttpClient
import okhttp3.mockwebserver.MockResponse
import okhttp3.mockwebserver.MockWebServer
import org.junit.After
import org.junit.Assert.assertEquals
import org.junit.Assert.assertTrue
import org.junit.Test

class RoomsClientTest {
    private val server = MockWebServer()
    private val client = RoomsClient(Rest(server.url("/").toString(), OkHttpClient(), token = { "tok" }, onSessionEnded = {}))

    @After
    fun tearDown() = server.shutdown()

    private fun json(body: String, code: Int = 200) = MockResponse().setResponseCode(code).setHeader("Content-Type", "application/json").setBody(body)

    @Test
    fun `rooms are read with their people, last message and renditions`() = runTest {
        server.enqueue(
            json(
                """{"rooms":[{"id":"r1","people":[{"id":"u2","display_name":"Rosa","handle":"rosa","language":"es"}],
                "last_message":{"id":"m1","room_id":"r1","sender_id":"u2","mine":false,"lang":"es","transcript":"hola",
                "duration_ms":2400,"created_at":"2026-10-31T09:12:00Z","renditions":[
                {"lang":"en","status":"ready","text":"hello","voice":"own","audio_url":"/v1/messages/m1/audio?lang=en"}]},
                "unread":1,"created_at":"2026-10-31T09:00:00Z"}]}""",
            ),
        )

        val rooms = client.rooms()

        val room = rooms.single()
        assertEquals("Rosa", room.other?.displayName)
        assertEquals(1, room.unread)
        val rendition = room.lastMessage!!.renditions.single()
        assertEquals(RenditionStatus.READY, rendition.status)
        assertEquals(RenditionVoice.OWN, rendition.voice)
        assertEquals("/v1/messages/m1/audio?lang=en", rendition.audioUrl)
        assertEquals("Bearer tok", server.takeRequest().getHeader("Authorization"))
    }

    @Test
    fun `a status this app does not know yet is read as on its way`() = runTest {
        server.enqueue(
            json("""{"id":"m1","room_id":"r1","sender_id":"u1","mine":true,"lang":"en","transcript":"hi","duration_ms":900,"created_at":"x","renditions":[{"lang":"es","status":"queued_for_review"}]}"""),
        )
        assertEquals(RenditionStatus.TRANSLATING, client.message("m1").renditions.single().status)
    }

    @Test
    fun `a voice note is sent with its words, length, language and key`() = runTest {
        server.enqueue(
            json("""{"id":"m1","room_id":"r1","sender_id":"u1","mine":true,"lang":"en","transcript":"hello","duration_ms":2400,"created_at":"x","renditions":[{"lang":"es","status":"translating"}]}""", 201),
        )

        client.sendMessage("r1", byteArrayOf(1, 2, 3), "hello", 2400, "en", idempotencyKey = "note-1")

        val request = server.takeRequest()
        assertEquals("/v1/rooms/r1/messages", request.path)
        assertEquals("note-1", request.getHeader("Idempotency-Key"))
        val body = request.body.readUtf8()
        listOf("name=\"transcript\"", "hello", "name=\"duration_ms\"", "2400", "name=\"lang\"", "name=\"audio\"").forEach {
            assertTrue("body has $it", body.contains(it))
        }
    }

    @Test
    fun `the change stream says when it is ready, then what changed`() = runTest {
        server.enqueue(
            MockResponse().setHeader("Content-Type", "text/event-stream").setBody(
                "event: ready\ndata: {}\n\nevent: room\ndata: {\"room_id\":\"r1\",\"message_id\":\"m1\"}\n\nevent: room\ndata: {\"room_id\":\"r2\"}\n\n",
            ),
        )

        val changes = client.changes().take(3).toList()

        assertEquals(listOf(RoomChange.Ready, RoomChange.Changed("r1", "m1"), RoomChange.Changed("r2", null)), changes)
        assertEquals("/v1/me/events", server.takeRequest().path)
    }
}
