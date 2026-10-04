package com.example.ptranslate.core.stt

import java.io.Closeable
import java.io.File

/**
 * A loaded Whisper model. This class is the whole JNI surface: it owns the
 * native pointer and nothing else in the app calls native code.
 *
 * Not thread-safe. A context holds the state of one transcription, so all
 * calls must come from one thread at a time. Call [close] to free the model's
 * memory, which is tens of megabytes outside the Java heap.
 */
class WhisperContext private constructor(private var handle: Long) : Closeable {

    /**
     * Transcribes 16 kHz mono samples in the range -1 to 1. Blocks for as
     * long as the model runs, so call it off the main thread.
     */
    fun transcribe(samples: FloatArray, languageTag: String, threads: Int): Result {
        check(handle != 0L) { "WhisperContext is closed" }

        val utf8 = nativeTranscribe(handle, samples, languageTag, threads)
            ?: throw TranscriptionException("The on-device model failed to run")

        return Result(
            text = String(utf8, Charsets.UTF_8).trim(),
            meanTokenProbability = nativeMeanTokenProbability(handle).takeIf { it >= 0f },
            noSpeechProbability = nativeNoSpeechProbability(handle).takeIf { it >= 0f },
        )
    }

    override fun close() {
        if (handle != 0L) {
            nativeFree(handle)
            handle = 0L
        }
    }

    class Result(
        val text: String,
        /** 0 to 1, or null when the model produced no words. */
        val meanTokenProbability: Float?,
        /** The model's estimate, 0 to 1, that the audio held no speech at all. */
        val noSpeechProbability: Float?,
    )

    companion object {
        init {
            System.loadLibrary("ptranslate_whisper")
        }

        /** @throws TranscriptionException if the file is missing or is not a Whisper model. */
        fun load(model: File): WhisperContext {
            if (!model.isFile) throw TranscriptionException("No model file at ${model.path}")
            val handle = nativeInit(model.absolutePath)
            if (handle == 0L) throw TranscriptionException("${model.name} is not a usable Whisper model")
            return WhisperContext(handle)
        }

        /** The CPU features the native library was built to use. */
        fun systemInfo(): String = nativeSystemInfo()

        // @JvmStatic makes these static methods of WhisperContext, which is
        // what the C++ function names (Java_..._WhisperContext_nativeX) and
        // their jclass parameter expect.
        @JvmStatic private external fun nativeInit(modelPath: String): Long
        @JvmStatic private external fun nativeFree(handle: Long)
        @JvmStatic private external fun nativeTranscribe(
            handle: Long,
            samples: FloatArray,
            language: String,
            threads: Int,
        ): ByteArray?
        @JvmStatic private external fun nativeMeanTokenProbability(handle: Long): Float
        @JvmStatic private external fun nativeNoSpeechProbability(handle: Long): Float
        @JvmStatic private external fun nativeSystemInfo(): String
    }
}
