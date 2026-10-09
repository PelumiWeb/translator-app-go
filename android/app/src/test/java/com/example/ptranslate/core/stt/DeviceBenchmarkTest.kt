package com.example.ptranslate.core.stt

import com.example.ptranslate.core.audio.PcmAudio
import kotlinx.coroutines.runBlocking
import org.junit.Assert.assertEquals
import org.junit.Assert.assertNull
import org.junit.Test

class DeviceBenchmarkTest {

    private class MemoryStore : SpeedStore {
        val saved = mutableMapOf<String, Float>()
        override fun load(modelVersion: String) = saved[modelVersion]
        override fun save(modelVersion: String, realTimeFactor: Float) {
            saved[modelVersion] = realTimeFactor
        }
    }

    private val store = MemoryStore()
    private var modelVersion: String? = "base:100"
    private var clockMs = 0L
    private var transcriptionMs = 4_000L
    private var failure: TranscriptionException? = null
    private val steps = mutableListOf<String>()

    // The clip is ten seconds long, so the real-time factor is the
    // transcription time in seconds divided by ten.
    private val benchmark = DeviceBenchmark(
        modelVersion = { modelVersion },
        sample = { PcmAudio(ShortArray(160_000)) },
        prepare = {
            steps += "prepare"
            clockMs += 9_000 // loading the model is slow, and must not be counted
        },
        transcribe = {
            steps += "transcribe"
            failure?.let { throw it }
            clockMs += transcriptionMs
        },
        store = store,
        now = { clockMs },
    )

    @Test
    fun `measures transcription time against audio length, leaving out model loading`() = runBlocking {
        benchmark.ensureMeasured()

        assertEquals(0.4f, benchmark.realTimeFactor.value)
        assertEquals(listOf("prepare", "transcribe"), steps)
        assertEquals(mapOf("base:100" to 0.4f), store.saved)
        assertEquals(false, benchmark.measuring.value)
    }

    @Test
    fun `uses the stored figure instead of measuring again`() = runBlocking {
        store.saved["base:100"] = 0.25f

        benchmark.ensureMeasured()

        assertEquals(0.25f, benchmark.realTimeFactor.value)
        assertEquals(emptyList<String>(), steps)
    }

    @Test
    fun `a different model is measured afresh`() = runBlocking {
        store.saved["tiny:50"] = 0.1f

        benchmark.ensureMeasured()

        assertEquals(0.4f, benchmark.realTimeFactor.value)
        assertEquals(setOf("tiny:50", "base:100"), store.saved.keys)
    }

    @Test
    fun `measure again replaces the stored figure`() = runBlocking {
        benchmark.ensureMeasured()
        transcriptionMs = 12_000

        benchmark.measureAgain()

        assertEquals(1.2f, benchmark.realTimeFactor.value)
        assertEquals(1.2f, store.saved["base:100"])
    }

    @Test
    fun `does nothing while there is no model`() = runBlocking {
        modelVersion = null

        benchmark.ensureMeasured()
        benchmark.measureAgain()

        assertNull(benchmark.realTimeFactor.value)
        assertEquals(emptyList<String>(), steps)
    }

    @Test
    fun `a model that will not run leaves the speed unknown`() = runBlocking {
        failure = TranscriptionException("The on-device model failed to run")

        benchmark.ensureMeasured()

        assertNull(benchmark.realTimeFactor.value)
        assertEquals(emptyMap<String, Float>(), store.saved)
        assertEquals(false, benchmark.measuring.value)
    }
}
