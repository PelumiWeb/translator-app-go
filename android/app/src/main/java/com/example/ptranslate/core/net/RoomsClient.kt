package com.example.ptranslate.core.net

import com.example.ptranslate.core.rooms.Message
import com.example.ptranslate.core.rooms.Room
import com.example.ptranslate.core.rooms.RoomChange
import kotlinx.coroutines.channels.Channel
import kotlinx.coroutines.channels.awaitClose
import kotlinx.coroutines.flow.Flow
import kotlinx.coroutines.flow.buffer
import kotlinx.coroutines.flow.callbackFlow
import kotlinx.serialization.SerialName
import kotlinx.serialization.Serializable
import kotlinx.serialization.SerializationException
import okhttp3.MultipartBody
import okhttp3.RequestBody.Companion.toRequestBody
import okhttp3.Response
import okhttp3.sse.EventSource
import okhttp3.sse.EventSourceListener
import okhttp3.sse.EventSources
import java.util.concurrent.TimeUnit

/** The backend's rooms: who the user talks with, and their voice messages. */
class RoomsClient internal constructor(private val rest: Rest) {

    // As for job events: the stream is silent between changes, apart from the
    // server's keep-alive, so the read timeout must be off.
    private val streams = EventSources.createFactory(
        rest.http.newBuilder().readTimeout(0, TimeUnit.MILLISECONDS).build(),
    )

    suspend fun rooms(): List<Room> = rest.decode<RoomList>(rest.get("/v1/rooms")).rooms

    /**
     * Opens a room with the person whose ID is [handle], or returns the one the two already have. Throws with code
     * "user_not_found" or "cannot_add_self".
     */
    suspend fun openRoom(handle: String): Room = rest.decode(rest.post("/v1/rooms", rest.body(OpenRoom(handle))))

    suspend fun room(id: String): Room = rest.decode(rest.get("/v1/rooms/$id"))

    /** The newest messages, oldest first, or with [before], the ones sent before that message. */
    suspend fun messages(roomId: String, before: String? = null, limit: Int = 50): List<Message> {
        val query = buildString {
            append("?limit=").append(limit)
            if (before != null) append("&before=").append(before)
        }
        return rest.decode<MessageList>(rest.get("/v1/rooms/$roomId/messages$query")).messages
    }

    /**
     * Sends a voice message. [idempotencyKey] stays the same across attempts at the same message, so a resend after
     * a lost response does not send it twice.
     */
    suspend fun sendMessage(
        roomId: String,
        wav: ByteArray,
        transcript: String,
        durationMs: Long,
        lang: String,
        idempotencyKey: String,
    ): Message {
        val form = MultipartBody.Builder()
            .setType(MultipartBody.FORM)
            .addFormDataPart("transcript", transcript)
            .addFormDataPart("duration_ms", durationMs.toString())
            .addFormDataPart("lang", lang)
            .addFormDataPart("audio", "message.wav", wav.toRequestBody(Rest.WAV_TYPE))
            .build()
        val request = rest.request("/v1/rooms/$roomId/messages")
            .header("Idempotency-Key", idempotencyKey)
            .post(form)
            .build()
        return rest.decode(rest.send(request))
    }

    suspend fun markRead(roomId: String) {
        rest.post("/v1/rooms/$roomId/read")
    }

    suspend fun message(id: String): Message = rest.decode(rest.get("/v1/messages/$id"))

    /** Tries again to deliver a message that could not be translated. */
    suspend fun retry(id: String): Message = rest.decode(rest.post("/v1/messages/$id/retry"))

    /** The audio at a rendition's audio URL, as a WAV file. */
    suspend fun audio(url: String): ByteArray = rest.get(url).body

    /**
     * Follows changes to the user's rooms until the server closes the stream or it fails. Nothing is replayed: on
     * [RoomChange.Ready], fetch what is on screen.
     */
    fun changes(): Flow<RoomChange> = callbackFlow {
        val listener = object : EventSourceListener() {
            override fun onEvent(eventSource: EventSource, id: String?, type: String?, data: String) {
                when (type) {
                    "ready" -> trySend(RoomChange.Ready)
                    "room" -> try {
                        val change = rest.json.decodeFromString<ChangePayload>(data)
                        trySend(RoomChange.Changed(change.roomId, change.messageId))
                    } catch (e: SerializationException) {
                        close(BackendException("The server sent an unreadable room event", e))
                    }
                }
            }

            override fun onClosed(eventSource: EventSource) {
                close()
            }

            override fun onFailure(eventSource: EventSource, t: Throwable?, response: Response?) {
                val status = response?.code ?: 0
                close(BackendException("Room changes stopped: ${t?.message ?: "status $status"}", t, retryable = status == 0 || status >= 500, status = status))
            }
        }
        val source = streams.newEventSource(rest.request("/v1/me/events").build(), listener)
        awaitClose { source.cancel() }
    }.buffer(Channel.UNLIMITED)
}

@Serializable
private data class RoomList(val rooms: List<Room>)

@Serializable
private data class MessageList(val messages: List<Message>)

@Serializable
private data class OpenRoom(val handle: String)

@Serializable
private data class ChangePayload(
    @SerialName("room_id") val roomId: String,
    @SerialName("message_id") val messageId: String? = null,
)
