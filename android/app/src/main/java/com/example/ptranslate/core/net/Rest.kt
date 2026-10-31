package com.example.ptranslate.core.net

import kotlinx.serialization.SerializationException
import kotlinx.serialization.json.Json
import okhttp3.HttpUrl
import okhttp3.HttpUrl.Companion.toHttpUrl
import okhttp3.MediaType.Companion.toMediaType
import okhttp3.OkHttpClient
import okhttp3.Request
import okhttp3.RequestBody
import okhttp3.RequestBody.Companion.toRequestBody

/**
 * JSON requests on behalf of the signed-in user, shared by [AccountClient]
 * and [RoomsClient]. Each request carries the session token from [token];
 * a refusal comes back as a [BackendException] with the server's error code.
 * A 401 on a request that carried a token means the session has ended, and
 * [onSessionEnded] is told.
 */
internal class Rest(
    baseUrl: String,
    val http: OkHttpClient,
    private val token: () -> String?,
    private val onSessionEnded: () -> Unit,
) {
    private val baseUrl: HttpUrl = baseUrl.toHttpUrl()

    // An unknown value in an enum, from a newer server, becomes the field's
    // default instead of failing the whole response.
    val json = Json {
        ignoreUnknownKeys = true
        coerceInputValues = true
    }

    fun url(path: String): HttpUrl = requireNotNull(baseUrl.resolve(path)) { "Invalid path: $path" }

    /** A request to [path], with the session token when there is one. */
    fun request(path: String): Request.Builder = Request.Builder().url(url(path)).apply {
        token()?.let { header("Authorization", "Bearer $it") }
    }

    suspend fun send(request: Request): HttpResult {
        val result = http.newCall(request).awaitResult()
        if (result.code == 401 && request.header("Authorization") != null) onSessionEnded()
        if (!result.isSuccessful) throw refusal(result, json)
        return result
    }

    suspend fun get(path: String): HttpResult = send(request(path).build())

    suspend fun post(path: String, body: RequestBody = EMPTY): HttpResult = send(request(path).post(body).build())

    suspend fun patch(path: String, body: RequestBody): HttpResult = send(request(path).patch(body).build())

    suspend fun delete(path: String): HttpResult = send(request(path).delete().build())

    inline fun <reified T> decode(result: HttpResult): T =
        try {
            json.decodeFromString<T>(result.text)
        } catch (e: SerializationException) {
            throw BackendException("The server sent an unreadable response", e)
        } catch (e: IllegalArgumentException) {
            throw BackendException("The server sent an unreadable response", e)
        }

    inline fun <reified T> body(value: T): RequestBody = json.encodeToString(value).toRequestBody(JSON_TYPE)

    companion object {
        val JSON_TYPE = "application/json".toMediaType()
        val WAV_TYPE = "audio/wav".toMediaType()
        private val EMPTY = ByteArray(0).toRequestBody(null)
    }
}
