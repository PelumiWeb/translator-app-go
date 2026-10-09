package com.example.ptranslate.core.stt

import com.example.ptranslate.core.Language
import com.example.ptranslate.core.audio.PcmAudio
import kotlinx.coroutines.flow.Flow
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.flow
import kotlinx.coroutines.flow.toList
import kotlinx.coroutines.runBlocking
import org.junit.Assert.assertEquals
import org.junit.Assert.assertThrows
import org.junit.Test

class RoutingTranscriberTest {

    /** A transcriber that plays back a script, and counts how often it is used. */
    private class Scripted(private val source: Transcript.Source) : Transcriber {
        var text = "from the ${source.name.lowercase()}"
        var confidence: Float? = null
        var failure: TranscriptionException? = null
        var partial: String? = null
        var calls = 0

        override fun transcribe(audio: PcmAudio, language: Language): Flow<TranscriptEvent> = flow {
            calls++
            emit(TranscriptEvent.StageChanged(TranscriptionStage.PROCESSING))
            partial?.let { emit(TranscriptEvent.Partial(it)) }
            failure?.let { throw it }
            emit(TranscriptEvent.Final(Transcript(text, confidence, source)))
        }
    }

    private val device = Scripted(Transcript.Source.ON_DEVICE).apply { confidence = 0.9f }
    private val cloud = Scripted(Transcript.Source.CLOUD)

    private val route = MutableStateFlow(TranscriptionRoute.AUTO)
    private var modelReady = true
    private var realTimeFactor: Float? = 0.3f

    private val router = RoutingTranscriber(
        onDevice = device,
        cloud = cloud,
        route = route,
        modelReady = { modelReady },
        realTimeFactor = { realTimeFactor },
        thresholds = FallbackThresholds(minConfidence = 0.6f, maxRealTimeFactor = 1.0f),
    )

    private fun events(): List<TranscriptEvent> =
        runBlocking { router.transcribe(PcmAudio(ShortArray(16)), Language.ENGLISH).toList() }

    private fun result(): Transcript = (events().last() as TranscriptEvent.Final).transcript

    // --- the two manual routes ---

    @Test
    fun `the device route uses only the device, whatever the result`() {
        route.value = TranscriptionRoute.ON_DEVICE
        device.confidence = 0.1f

        assertEquals(Transcript("from the on_device", 0.1f, Transcript.Source.ON_DEVICE), result())
        assertEquals(0, cloud.calls)
    }

    @Test
    fun `the cloud route uses only the cloud`() {
        route.value = TranscriptionRoute.CLOUD

        assertEquals(Transcript("from the cloud", null, Transcript.Source.CLOUD), result())
        assertEquals(0, device.calls)
    }

    @Test
    fun `follows the route for each new recording`() {
        route.value = TranscriptionRoute.ON_DEVICE
        assertEquals(Transcript.Source.ON_DEVICE, result().source)

        route.value = TranscriptionRoute.CLOUD
        assertEquals(Transcript.Source.CLOUD, result().source)
    }

    // --- automatic: the device is good enough ---

    @Test
    fun `a confident device result is used and nothing is uploaded`() {
        val transcript = result()

        assertEquals(Transcript("from the on_device", 0.9f, Transcript.Source.ON_DEVICE, note = null), transcript)
        assertEquals(0, cloud.calls)
    }

    @Test
    fun `confidence exactly at the threshold is accepted`() {
        device.confidence = 0.6f

        assertEquals(Transcript.Source.ON_DEVICE, result().source)
        assertEquals(0, cloud.calls)
    }

    @Test
    fun `a result with no confidence figure is accepted`() {
        device.confidence = null

        assertEquals(Transcript.Source.ON_DEVICE, result().source)
        assertEquals(0, cloud.calls)
    }

    @Test
    fun `a device whose speed was never measured is given the chance`() {
        realTimeFactor = null

        assertEquals(Transcript.Source.ON_DEVICE, result().source)
    }

    // --- automatic: reasons to go to the cloud before trying ---

    @Test
    fun `with no model the recording goes straight to the cloud`() {
        modelReady = false

        assertEquals(Transcript("from the cloud", null, Transcript.Source.CLOUD, RoutingNote.NO_MODEL), result())
        assertEquals(0, device.calls)
    }

    @Test
    fun `a device that is too slow is skipped`() {
        realTimeFactor = 1.8f

        assertEquals(RoutingNote.SLOW_DEVICE, result().note)
        assertEquals(Transcript.Source.CLOUD, result().source)
        assertEquals(0, device.calls)
    }

    @Test
    fun `speed exactly at the limit still counts as fast enough`() {
        realTimeFactor = 1.0f

        assertEquals(Transcript.Source.ON_DEVICE, result().source)
    }

    @Test
    fun `a slow device is used after all when the cloud cannot be reached`() {
        realTimeFactor = 1.8f
        cloud.failure = TranscriptionException("Could not reach the server")

        val transcript = result()

        assertEquals(Transcript.Source.ON_DEVICE, transcript.source)
        assertEquals(RoutingNote.CLOUD_UNAVAILABLE, transcript.note)
    }

    // --- automatic: reasons to go to the cloud after trying ---

    @Test
    fun `a low-confidence result is shown as a draft and replaced by the cloud's`() {
        device.text = "wreck a nice beach"
        device.confidence = 0.42f
        cloud.text = "recognise speech"

        assertEquals(
            listOf(
                TranscriptEvent.StageChanged(TranscriptionStage.PROCESSING), // the device working
                TranscriptEvent.Partial("wreck a nice beach"), // its doubtful result, as a draft
                TranscriptEvent.StageChanged(TranscriptionStage.PROCESSING), // the cloud working
                TranscriptEvent.Final(
                    Transcript("recognise speech", null, Transcript.Source.CLOUD, RoutingNote.LOW_CONFIDENCE),
                ),
            ),
            events(),
        )
    }

    @Test
    fun `a low-confidence result is kept when the cloud cannot be reached`() {
        device.text = "wreck a nice beach"
        device.confidence = 0.42f
        cloud.failure = TranscriptionException("Could not reach the server")

        assertEquals(
            Transcript("wreck a nice beach", 0.42f, Transcript.Source.ON_DEVICE, RoutingNote.CLOUD_UNAVAILABLE),
            result(),
        )
    }

    @Test
    fun `sound without words on the device is given to the cloud`() {
        device.failure = NoSpeechException("No speech was recognised")

        assertEquals(Transcript("from the cloud", null, Transcript.Source.CLOUD, RoutingNote.NO_SPEECH_ON_DEVICE), result())
    }

    @Test
    fun `no words on the device and no cloud reports that no speech was recognised`() {
        device.failure = NoSpeechException("No speech was recognised")
        cloud.failure = TranscriptionException("Could not reach the server")

        val error = assertThrows(NoSpeechException::class.java) { events() }

        // The device's verdict, not the network error: it is the more useful one.
        assertEquals("No speech was recognised", error.message)
    }

    @Test
    fun `a model that fails to run falls back to the cloud`() {
        device.failure = TranscriptionException("The on-device model failed to run")

        assertEquals(RoutingNote.ON_DEVICE_FAILED, result().note)
        assertEquals(Transcript.Source.CLOUD, result().source)
    }

    @Test
    fun `a silent recording is never uploaded`() {
        device.failure = SilentAudioException("The recording was silent (-100 dB). Check the microphone")

        assertThrows(SilentAudioException::class.java) { events() }

        assertEquals(0, cloud.calls)
    }

    @Test
    fun `with no model and no cloud the cloud's error is reported`() {
        modelReady = false
        cloud.failure = TranscriptionException("Could not reach the server")

        val error = assertThrows(TranscriptionException::class.java) { events() }

        assertEquals("Could not reach the server", error.message)
        assertEquals(0, device.calls)
    }
}
