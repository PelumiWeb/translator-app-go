package com.example.ptranslate.ui

import com.example.ptranslate.core.FakeTranslator
import com.example.ptranslate.core.Language
import com.example.ptranslate.core.audio.AudioRecorder
import com.example.ptranslate.core.audio.PcmAudio
import com.example.ptranslate.core.audio.Recording
import com.example.ptranslate.core.pipeline.SpeechTranslationPipeline
import com.example.ptranslate.core.stt.Transcriber
import com.example.ptranslate.core.stt.Transcript
import com.example.ptranslate.core.stt.TranscriptEvent
import com.example.ptranslate.core.stt.TranscriptionException
import com.example.ptranslate.core.stt.TranscriptionRoute
import com.example.ptranslate.core.stt.TranscriptionStage
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.ExperimentalCoroutinesApi
import kotlinx.coroutines.channels.Channel
import kotlinx.coroutines.flow.Flow
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.consumeAsFlow
import kotlinx.coroutines.flow.flow
import kotlinx.coroutines.test.StandardTestDispatcher
import kotlinx.coroutines.test.advanceTimeBy
import kotlinx.coroutines.test.resetMain
import kotlinx.coroutines.test.runCurrent
import kotlinx.coroutines.test.runTest
import kotlinx.coroutines.test.setMain
import org.junit.After
import org.junit.Assert.assertEquals
import org.junit.Assert.assertNull
import org.junit.Assert.assertTrue
import org.junit.Before
import org.junit.Test

@OptIn(ExperimentalCoroutinesApi::class)
class TranslateViewModelTest {

    private class FakeRecorder(private val audio: PcmAudio = PcmAudio(ShortArray(16_000))) : AudioRecorder {
        var startError: Exception? = null
        var cancelled = false

        override fun start(): Recording {
            startError?.let { throw it }
            return object : Recording {
                override suspend fun stop() = audio
                override fun cancel() {
                    cancelled = true
                }
            }
        }
    }

    /** Emits whatever the test sends, so each step can be checked on its own. */
    private class FakeTranscriber : Transcriber {
        val events = Channel<TranscriptEvent>(Channel.UNLIMITED)
        override fun transcribe(audio: PcmAudio, language: Language): Flow<TranscriptEvent> = events.consumeAsFlow()
    }

    private val dispatcher = StandardTestDispatcher()

    private val route = MutableStateFlow(TranscriptionRoute.ON_DEVICE)
    private var clockMs = 0L

    private fun newViewModel(recorder: AudioRecorder, transcriber: Transcriber) =
        TranslateViewModel(recorder, SpeechTranslationPipeline(transcriber, FakeTranslator()), route, now = { clockMs })

    @Before
    fun setUp() {
        // viewModelScope runs on Dispatchers.Main, which does not exist in a
        // JVM test until one is installed.
        Dispatchers.setMain(dispatcher)
    }

    @After
    fun tearDown() {
        Dispatchers.resetMain()
    }

    @Test
    fun `record, stop, then text arrives word by word`() = runTest(dispatcher) {
        val transcriber = FakeTranscriber()
        val viewModel = newViewModel(FakeRecorder(), transcriber)

        viewModel.onRecordClicked()
        assertEquals(Phase.RECORDING, viewModel.state.value.phase)

        viewModel.onRecordClicked()
        runCurrent()
        assertEquals(Phase.WORKING, viewModel.state.value.phase)

        transcriber.events.send(TranscriptEvent.StageChanged(TranscriptionStage.QUEUED))
        runCurrent()
        assertEquals("Queued", viewModel.state.value.status)

        transcriber.events.send(TranscriptEvent.Partial("hello"))
        runCurrent()
        assertEquals("hello", viewModel.state.value.text)

        transcriber.events.send(
            TranscriptEvent.Final(Transcript("hello world", confidence = null, Transcript.Source.CLOUD)),
        )
        transcriber.events.close()
        runCurrent()

        val state = viewModel.state.value
        assertEquals(Phase.IDLE, state.phase)
        assertEquals("Done", state.status)
        assertEquals("hello world", state.text)
        assertNull(state.error)
    }

    @Test
    fun `a failed transcription shows the error and allows another try`() = runTest(dispatcher) {
        val failing = object : Transcriber {
            override fun transcribe(audio: PcmAudio, language: Language): Flow<TranscriptEvent> =
                flow { throw TranscriptionException("provider exploded") }
        }
        val viewModel = newViewModel(FakeRecorder(), failing)

        viewModel.onRecordClicked()
        viewModel.onRecordClicked()
        runCurrent()

        val state = viewModel.state.value
        assertEquals(Phase.IDLE, state.phase)
        assertEquals("Failed", state.status)
        assertEquals("provider exploded", state.error)
    }

    @Test
    fun `a recording that is too short is not uploaded`() = runTest(dispatcher) {
        val transcriber = FakeTranscriber()
        val recorder = FakeRecorder(audio = PcmAudio(ShortArray(160))) // 10 ms
        val viewModel = newViewModel(recorder, transcriber)

        viewModel.onRecordClicked()
        viewModel.onRecordClicked()
        runCurrent()

        assertEquals(Phase.IDLE, viewModel.state.value.phase)
        assertTrue(viewModel.state.value.error.orEmpty().contains("too short"))
    }

    @Test
    fun `recording stops by itself at the time limit`() = runTest(dispatcher) {
        val transcriber = FakeTranscriber()
        val viewModel = newViewModel(FakeRecorder(), transcriber)

        viewModel.onRecordClicked()
        advanceTimeBy(29_999)
        assertEquals(Phase.RECORDING, viewModel.state.value.phase)

        advanceTimeBy(2)
        assertEquals(Phase.WORKING, viewModel.state.value.phase)

        // The work must survive the timer coroutine that started it.
        transcriber.events.send(TranscriptEvent.Final(Transcript("late", null, Transcript.Source.CLOUD)))
        transcriber.events.close()
        runCurrent()
        assertEquals("late", viewModel.state.value.text)
        assertEquals(Phase.IDLE, viewModel.state.value.phase)
    }

    @Test
    fun `shows where the transcript came from, how long it took, and the confidence`() = runTest(dispatcher) {
        val transcriber = FakeTranscriber()
        val recorder = FakeRecorder(audio = PcmAudio(ShortArray(64_000))) // 4 seconds
        val viewModel = newViewModel(recorder, transcriber)

        viewModel.onRecordClicked()
        viewModel.onRecordClicked()
        runCurrent()
        clockMs += 1_200
        transcriber.events.send(
            TranscriptEvent.Final(Transcript("hello", confidence = 0.874f, Transcript.Source.ON_DEVICE)),
        )
        transcriber.events.close()
        runCurrent()

        assertEquals(
            "On device, 1.2 s for 4.0 s of audio (0.30x real time), confidence 0.87",
            viewModel.state.value.details,
        )
    }

    @Test
    fun `details leave out the confidence when there is none`() = runTest(dispatcher) {
        val transcriber = FakeTranscriber()
        val viewModel = newViewModel(FakeRecorder(), transcriber) // 1 second of audio

        viewModel.onRecordClicked()
        viewModel.onRecordClicked()
        runCurrent()
        clockMs += 3_000
        transcriber.events.send(TranscriptEvent.Final(Transcript("hello", confidence = null, Transcript.Source.CLOUD)))
        transcriber.events.close()
        runCurrent()

        assertEquals("Cloud, 3.0 s for 1.0 s of audio (3.00x real time)", viewModel.state.value.details)
    }

    @Test
    fun `the switch changes the route, but not during a recording`() = runTest(dispatcher) {
        val viewModel = newViewModel(FakeRecorder(), FakeTranscriber())
        assertTrue(viewModel.state.value.onDevice)

        viewModel.onRouteChanged(onDevice = false)
        assertEquals(TranscriptionRoute.CLOUD, route.value)
        assertEquals(false, viewModel.state.value.onDevice)

        viewModel.onRecordClicked() // now recording
        viewModel.onRouteChanged(onDevice = true)
        assertEquals(TranscriptionRoute.CLOUD, route.value)
        assertEquals(false, viewModel.state.value.onDevice)
    }

    @Test
    fun `a microphone that cannot be opened is reported`() = runTest(dispatcher) {
        val recorder = FakeRecorder().apply { startError = IllegalStateException("Could not open the microphone") }
        val viewModel = newViewModel(recorder, FakeTranscriber())

        viewModel.onRecordClicked()

        assertEquals(Phase.IDLE, viewModel.state.value.phase)
        assertEquals("Could not open the microphone", viewModel.state.value.error)
    }
}
