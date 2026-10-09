package com.example.ptranslate.core.net

import com.example.ptranslate.core.Language
import kotlinx.coroutines.channels.Channel
import kotlinx.coroutines.channels.awaitClose
import kotlinx.coroutines.flow.Flow
import kotlinx.coroutines.flow.buffer
import kotlinx.coroutines.flow.callbackFlow
import kotlinx.coroutines.suspendCancellableCoroutine
import kotlinx.serialization.Serializable
import kotlinx.serialization.SerializationException
import kotlinx.serialization.json.Json
import okhttp3.Call
import okhttp3.Callback
import okhttp3.HttpUrl
import okhttp3.HttpUrl.Companion.toHttpUrl
import okhttp3.MediaType.Companion.toMediaType
import okhttp3.MultipartBody
import okhttp3.OkHttpClient
import okhttp3.Request
import okhttp3.RequestBody.Companion.toRequestBody
import okhttp3.Response
import okhttp3.sse.EventSource
import okhttp3.sse.EventSourceListener
import okhttp3.sse.EventSources
import java.io.Closeable
import java.io.IOException
import java.io.InputStream
import java.util.concurrent.TimeUnit
import kotlin.coroutines.resume
import kotlin.coroutines.resumeWithException

/**
 * @param retryable whether trying again could succeed. True for a connection
 *   problem or a server fault; false when the server understood the request
 *   and refused it, since sending the same thing again gets the same answer.
 */
class BackendException(
    message: String,
    cause: Throwable? = null,
    val retryable: Boolean = false,
) : IOException(message, cause)

/** One entry in a job's event stream, already parsed. */
sealed interface JobEvent {
    data object Queued : JobEvent
    data object Processing : JobEvent
    data class Partial(val text: String) : JobEvent
    data class Done(val text: String, val language: String) : JobEvent
    data class Failed(val message: String) : JobEvent

    /** A type this version of the app does not know. Its number still counts. */
    data object Unknown : JobEvent
}

/**
 * An event with its position in the job's history. [seq] counts 1, 2, 3 ...
 * and is what a reconnecting client sends back so the server can continue
 * from the right place.
 */
data class JobUpdate(val seq: Int, val event: JobEvent)

/** One model the backend offers, as listed in its manifest. */
@Serializable
data class RemoteModel(
    val id: String,
    val version: String,
    val size: Long,
    val sha256: String,
    val url: String,
)

/**
 * An open model download. [partial] is true when the server honoured the
 * requested offset and [body] holds only the bytes from there on; false when
 * it is sending the whole file from the start.
 */
class ModelDownload(val partial: Boolean, val body: InputStream, private val response: Response) : Closeable {
    override fun close() = response.close()
}

/** The only class that knows the backend's URLs and JSON. */
class BackendClient(baseUrl: String, private val http: OkHttpClient) {

    private val baseUrl: HttpUrl = baseUrl.toHttpUrl()

    // An event stream is silent between events (the server's keep-alive is
    // every 15 s), so OkHttp's default 10 s read timeout would cut it off.
    private val streams = EventSources.createFactory(
        http.newBuilder().readTimeout(0, TimeUnit.MILLISECONDS).build(),
    )

    private val json = Json { ignoreUnknownKeys = true }

    /**
     * Uploads the audio and returns the job's id.
     *
     * [idempotencyKey] identifies this upload across attempts. If a response
     * is lost and the upload is sent again with the same key, the server
     * returns the job it already created instead of making a second one.
     */
    suspend fun createJob(wav: ByteArray, sourceLang: Language, idempotencyKey: String): String {
        val form = MultipartBody.Builder()
            .setType(MultipartBody.FORM)
            .addFormDataPart("source_lang", sourceLang.tag)
            .addFormDataPart("audio", "audio.wav", wav.toRequestBody("audio/wav".toMediaType()))
            .build()
        val request = Request.Builder()
            .url(url("/v1/jobs"))
            .header("Idempotency-Key", idempotencyKey)
            .post(form)
            .build()

        val result = http.newCall(request).awaitResult()
        if (!result.isSuccessful) {
            throw BackendException(
                errorMessage(result.body) ?: "The server answered ${result.code}",
                retryable = result.code >= 500,
            )
        }
        return try {
            json.decodeFromString<CreateJobResponse>(result.body).id
        } catch (e: SerializationException) {
            throw BackendException("The server sent an unreadable response", e)
        }
    }

    /** Fetches the list of models the backend offers. */
    suspend fun modelManifest(): List<RemoteModel> {
        val request = Request.Builder().url(url("/v1/models/manifest")).build()

        val result = http.newCall(request).awaitResult()
        if (!result.isSuccessful) {
            throw BackendException(errorMessage(result.body) ?: "The server answered ${result.code}")
        }
        return try {
            json.decodeFromString<ManifestResponse>(result.body).models
        } catch (e: SerializationException) {
            throw BackendException("The server sent an unreadable model list", e)
        }
    }

    /**
     * Opens a model for download, starting at [fromByte]. Blocks until the
     * response headers arrive, so call it off the main thread; the caller
     * reads [ModelDownload.body] and closes the download.
     *
     * A resume sends two headers. `Range` asks for the rest of the file.
     * `If-Range` adds a condition, "only if the file is still this one",
     * using the checksum the server publishes as its ETag. If the file was
     * replaced meanwhile, the server sends the whole new one instead.
     */
    fun openModel(model: RemoteModel, fromByte: Long): ModelDownload {
        val request = Request.Builder().url(url(model.url)).apply {
            if (fromByte > 0) {
                header("Range", "bytes=$fromByte-")
                header("If-Range", "\"${model.sha256}\"")
            }
        }.build()

        val response = try {
            http.newCall(request).execute()
        } catch (e: IOException) {
            throw BackendException("Could not reach the server: ${e.message}", e)
        }
        val body = response.body
        if ((response.code != 200 && response.code != 206) || body == null) {
            response.close()
            throw BackendException("The server answered ${response.code}")
        }
        return ModelDownload(partial = response.code == 206, body = body.byteStream(), response = response)
    }

    /**
     * Follows a job's events until the server closes the stream. Cancelling
     * the collector closes the connection.
     *
     * [afterSeq] is the number of the last event already received, or 0 for
     * the whole history. It is sent as Last-Event-ID, the standard header for
     * resuming an event stream.
     */
    fun jobEvents(jobId: String, afterSeq: Int = 0): Flow<JobUpdate> = callbackFlow {
        val request = Request.Builder().url(url("/v1/jobs/$jobId/events")).apply {
            if (afterSeq > 0) header("Last-Event-ID", afterSeq.toString())
        }.build()

        val listener = object : EventSourceListener() {
            override fun onEvent(eventSource: EventSource, id: String?, type: String?, data: String) {
                // Every event the server sends carries its number as the id.
                val seq = id?.toIntOrNull() ?: return
                try {
                    trySend(JobUpdate(seq, parseEvent(type, data)))
                } catch (e: SerializationException) {
                    close(BackendException("The server sent an unreadable $type event", e))
                }
            }

            override fun onClosed(eventSource: EventSource) {
                close()
            }

            override fun onFailure(eventSource: EventSource, t: Throwable?, response: Response?) {
                if (response != null && !response.isSuccessful) {
                    // The server answered and said no, e.g. 404 for an unknown job.
                    close(BackendException("The server answered ${response.code}", t, retryable = response.code >= 500))
                } else {
                    val reason = t?.message ?: "the connection was lost"
                    close(BackendException("Event stream failed: $reason", t, retryable = true))
                }
            }
        }

        val source = streams.newEventSource(request, listener)
        awaitClose { source.cancel() }
    }
        // OkHttp delivers events from its own thread and cannot be told to
        // wait, so nothing may be dropped if the collector is briefly slow.
        .buffer(Channel.UNLIMITED)

    private fun parseEvent(type: String?, data: String): JobEvent = when (type) {
        "queued" -> JobEvent.Queued
        "processing" -> JobEvent.Processing
        "partial" -> JobEvent.Partial(json.decodeFromString<TextPayload>(data).text)
        "done" -> json.decodeFromString<DonePayload>(data).let { JobEvent.Done(it.text, it.language) }
        "error" -> JobEvent.Failed(json.decodeFromString<MessagePayload>(data).message)
        else -> JobEvent.Unknown // a type added later; older apps skip it
    }

    private fun errorMessage(body: String): String? =
        try {
            json.decodeFromString<ErrorResponse>(body).error.message
        } catch (_: SerializationException) {
            null
        }

    private fun url(path: String): HttpUrl =
        requireNotNull(baseUrl.resolve(path)) { "Invalid path: $path" }
}

private class HttpResult(val code: Int, val body: String) {
    val isSuccessful: Boolean get() = code in 200..299
}

/** Runs the call without blocking a thread, and cancels it with the coroutine. */
private suspend fun Call.awaitResult(): HttpResult = suspendCancellableCoroutine { continuation ->
    continuation.invokeOnCancellation { cancel() }

    enqueue(object : Callback {
        override fun onFailure(call: Call, e: IOException) {
            continuation.resumeWithException(
                BackendException("Could not reach the server: ${e.message}", e, retryable = true),
            )
        }

        override fun onResponse(call: Call, response: Response) {
            // The body is read here, on OkHttp's thread. Reading it after
            // resuming could happen on the main thread, where network I/O
            // throws NetworkOnMainThreadException.
            val result = try {
                response.use { HttpResult(it.code, it.body?.string().orEmpty()) }
            } catch (e: IOException) {
                continuation.resumeWithException(
                    BackendException("Could not read the server's response", e, retryable = true),
                )
                return
            }
            continuation.resume(result)
        }
    })
}

@Serializable
private data class ManifestResponse(val models: List<RemoteModel>)

@Serializable
private data class CreateJobResponse(val id: String)

@Serializable
private data class TextPayload(val text: String)

@Serializable
private data class DonePayload(val text: String, val language: String = "")

@Serializable
private data class MessagePayload(val message: String)

@Serializable
private data class ErrorResponse(val error: Detail) {
    @Serializable
    data class Detail(val code: String = "", val message: String)
}
