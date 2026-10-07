package com.example.ptranslate.core.model

import com.example.ptranslate.core.net.BackendClient
import com.example.ptranslate.core.net.BackendException
import com.example.ptranslate.core.net.RemoteModel
import kotlinx.coroutines.CoroutineDispatcher
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.asStateFlow
import kotlinx.coroutines.runInterruptible
import kotlinx.coroutines.sync.Mutex
import kotlinx.coroutines.sync.withLock
import java.io.File
import java.io.FileOutputStream
import java.io.IOException

sealed interface ModelState {
    /** Not on this device, and no download running. */
    data object Missing : ModelState
    data class Downloading(val bytes: Long, val total: Long) : ModelState
    data object Verifying : ModelState
    data class Ready(val file: File) : ModelState
    data class Failed(val reason: String) : ModelState
}

class ModelException(message: String, cause: Throwable? = null) : Exception(message, cause)

/**
 * Gets one model from the backend onto the device and keeps it there.
 *
 * Files in [directory], for a model with id "base":
 * - `base.bin`: the installed model. It only ever appears by being renamed
 *   from a fully downloaded, checksum-verified file, so its presence alone
 *   means "safe to load".
 * - `base.part`: a download in progress, kept across failures and restarts so
 *   the next attempt continues from where this one stopped.
 * - `base.part.sha256`: the checksum the partial file belongs to. If the
 *   server's model has changed since, the partial file is thrown away.
 */
class ModelManager(
    private val backend: BackendClient,
    private val directory: File,
    private val modelId: String,
    private val ioDispatcher: CoroutineDispatcher,
) {
    private val installed = File(directory, "$modelId.bin")
    private val partial = File(directory, "$modelId.part")
    private val partialSha = File(directory, "$modelId.part.sha256")

    private val _state = MutableStateFlow(
        if (installed.isFile) ModelState.Ready(installed) else ModelState.Missing,
    )
    val state: StateFlow<ModelState> = _state.asStateFlow()

    // One download at a time: two callers asking at once share the result
    // instead of writing to the same file.
    private val mutex = Mutex()

    /**
     * Returns the installed model, downloading it first if needed. Safe to
     * call again after a failure: it continues the partial download.
     *
     * An installed model is returned without contacting the server, so the
     * app works offline once it has one.
     *
     * @throws ModelException if the model cannot be installed.
     */
    suspend fun ensureInstalled(): File = mutex.withLock {
        if (installed.isFile) {
            _state.value = ModelState.Ready(installed)
            return@withLock installed
        }

        try {
            val model = backend.modelManifest().firstOrNull { it.id == modelId }
                ?: throw ModelException("The server does not offer the model $modelId")

            // runInterruptible: the blocking file and network calls inside
            // are interrupted if the coroutine is cancelled.
            runInterruptible(ioDispatcher) {
                download(model)
                verify(model)
                install()
            }
            _state.value = ModelState.Ready(installed)
            installed
        } catch (e: ModelException) {
            fail(e.message ?: "The model could not be installed", e)
        } catch (e: BackendException) {
            fail(e.message ?: "The server could not be reached", e)
        } catch (e: IOException) {
            // A dropped connection mid-download lands here. The partial file
            // stays, so the next call resumes.
            fail("The download was interrupted", e)
        }
    }

    private fun download(model: RemoteModel) {
        directory.mkdirs()

        // A partial file is only worth keeping if it is the start of this
        // exact model.
        val resumable = partial.isFile &&
            partialSha.isFile && partialSha.readText() == model.sha256 &&
            partial.length() <= model.size
        if (!resumable) partial.delete()
        partialSha.writeText(model.sha256)

        val have = partial.length() // 0 when the file does not exist
        if (have == model.size) return // only verification was left

        backend.openModel(model, fromByte = have).use { download ->
            // The server may ignore the offset and send everything (a plain
            // 200). Appending that to what we have would corrupt the file, so
            // in that case start again from an empty file.
            val append = download.partial
            var written = if (append) have else 0L
            _state.value = ModelState.Downloading(written, model.size)

            FileOutputStream(partial, append).use { out ->
                val buffer = ByteArray(64 * 1024)
                var lastReported = written
                while (true) {
                    val read = download.body.read(buffer)
                    if (read < 0) break
                    out.write(buffer, 0, read)
                    written += read
                    // Often enough for a smooth progress bar, not on every chunk.
                    if (written - lastReported >= PROGRESS_STEP_BYTES) {
                        _state.value = ModelState.Downloading(written, model.size)
                        lastReported = written
                    }
                }
            }
            _state.value = ModelState.Downloading(written, model.size)
        }
    }

    private fun verify(model: RemoteModel) {
        _state.value = ModelState.Verifying
        val actual = Sha256.ofFile(partial)
        if (actual != model.sha256) {
            // Nothing in a corrupt file can be trusted, so none of it is
            // kept for a resume.
            partial.delete()
            partialSha.delete()
            throw ModelException("The downloaded model is corrupt (checksum mismatch)")
        }
    }

    private fun install() {
        // A rename within one directory is atomic: other code sees either no
        // model or the complete one, never a half-written file.
        if (!partial.renameTo(installed)) throw ModelException("Could not save the model")
        partialSha.delete()
    }

    private fun fail(reason: String, cause: Exception): Nothing {
        _state.value = ModelState.Failed(reason)
        throw ModelException(reason, cause)
    }

    private companion object {
        const val PROGRESS_STEP_BYTES = 256 * 1024L
    }
}
