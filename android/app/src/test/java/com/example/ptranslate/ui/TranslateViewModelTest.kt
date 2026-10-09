package com.example.ptranslate.ui

import com.example.ptranslate.core.FakeTranslator
import com.example.ptranslate.core.Language
import com.example.ptranslate.core.audio.AudioRecorder
import com.example.ptranslate.core.audio.PcmAudio
import com.example.ptranslate.core.audio.Recording
import com.example.ptranslate.core.model.ModelException
import com.example.ptranslate.core.model.ModelInstaller
import com.example.ptranslate.core.model.ModelState
import com.example.ptranslate.core.pipeline.SpeechTranslationPipeline
import com.example.ptranslate.core.stt.DeviceSpeed
import com.example.ptranslate.core.stt.RoutingNote
import com.example.ptranslate.core.stt.Transcriber
import com.example.ptranslate.core.stt.Transcript
import com.example.ptranslate.core.stt.TranscriptEvent
import com.example.ptranslate.core.stt.TranscriptionException
import com.example.ptranslate.core.stt.TranscriptionRoute
import com.example.ptranslate.core.stt.TranscriptionStage
import com.example.ptranslate.core.translate.TranslationException
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
import java.io.File

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

    private val route = MutableStateFlow(TranscriptionRoute.AUTO)
    private var clockMs = 0L

    private val translator = FakeTranslator()

    private class FakeModels : ModelInstaller {
        override val state = MutableStateFlow<ModelState>(ModelState.Missing)
        var installCalls = 0
        var failWith: String? = null

        override suspend fun ensureInstalled(): File {
            installCalls++
            failWith?.let {
                state.value = ModelState.Failed(it)
                throw ModelException(it)
            }
            return File("model.bin").also { state.value = ModelState.Ready(it) }
        }
    }

    private val models = FakeModels()

    private class FakeSpeed : DeviceSpeed {
        override val realTimeFactor = MutableStateFlow<Float?>(null)
        override val measuring = MutableStateFlow(false)
        var measurements = 0

        override suspend fun measureAgain() {
            measurements++
            realTimeFactor.value = 0.5f
        }
    }

    private val speed = FakeSpeed()

    private fun newViewModel(recorder: AudioRecorder, transcriber: Transcriber) =
        TranslateViewModel(
            recorder = recorder,
            pipeline = SpeechTranslationPipeline(transcriber, translator),
            route = route,
            models = models,
            speed = speed,
            sourceLanguages = listOf(Language("en"), Language("es")),
            targetLanguages = listOf(Language("en"), Language("es"), Language("fr")),
            now = { clockMs },
        )

    /** A transcriber that answers at once with [text]. */
    private fun saying(text: String) = object : Transcriber {
        override fun transcribe(audio: PcmAudio, language: Language): Flow<TranscriptEvent> =
            flow { emit(TranscriptEvent.Final(Transcript(text, confidence = null, Transcript.Source.ON_DEVICE))) }
    }

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
    fun `the route can be chosen, but not during a recording`() = runTest(dispatcher) {
        val viewModel = newViewModel(FakeRecorder(), FakeTranscriber())
        assertEquals(TranscriptionRoute.AUTO, viewModel.state.value.route)

        viewModel.onRouteSelected(TranscriptionRoute.CLOUD)
        assertEquals(TranscriptionRoute.CLOUD, route.value)
        assertEquals(TranscriptionRoute.CLOUD, viewModel.state.value.route)

        viewModel.onRecordClicked() // now recording
        viewModel.onRouteSelected(TranscriptionRoute.ON_DEVICE)
        assertEquals(TranscriptionRoute.CLOUD, route.value)
        assertEquals(TranscriptionRoute.CLOUD, viewModel.state.value.route)
    }

    @Test
    fun `says why a result was sent to the server`() = runTest(dispatcher) {
        val transcriber = FakeTranscriber()
        val viewModel = newViewModel(FakeRecorder(), transcriber)

        viewModel.onRecordClicked()
        viewModel.onRecordClicked()
        runCurrent()
        transcriber.events.send(
            TranscriptEvent.Final(
                Transcript("hello", confidence = null, Transcript.Source.CLOUD, RoutingNote.LOW_CONFIDENCE),
            ),
        )
        transcriber.events.close()
        runCurrent()

        assertEquals(
            "Sent to the server: this device was not confident in its own result",
            viewModel.state.value.routing,
        )

        // The explanation belongs to that result and goes with it.
        viewModel.onRecordClicked()
        assertNull(viewModel.state.value.routing)
    }

    @Test
    fun `a result that took the first choice needs no explanation`() = runTest(dispatcher) {
        val viewModel = newViewModel(FakeRecorder(), saying("good morning"))

        viewModel.onRecordClicked()
        viewModel.onRecordClicked()
        runCurrent()

        assertNull(viewModel.state.value.routing)
    }

    @Test
    fun `shows the device's speed and whether it is too slow`() = runTest(dispatcher) {
        val viewModel = newViewModel(FakeRecorder(), FakeTranscriber())
        runCurrent()
        assertNull(viewModel.state.value.deviceSpeed)
        assertEquals(false, viewModel.state.value.deviceTooSlow)

        speed.realTimeFactor.value = 0.4f
        runCurrent()
        assertEquals(0.4f, viewModel.state.value.deviceSpeed)
        assertEquals(false, viewModel.state.value.deviceTooSlow)

        speed.realTimeFactor.value = 1.7f // over the default limit of 1.0
        runCurrent()
        assertEquals(true, viewModel.state.value.deviceTooSlow)

        speed.measuring.value = true
        runCurrent()
        assertEquals(true, viewModel.state.value.measuringSpeed)
    }

    @Test
    fun `measure again asks for a new measurement, once at a time`() = runTest(dispatcher) {
        val viewModel = newViewModel(FakeRecorder(), FakeTranscriber())

        viewModel.onMeasureSpeedClicked()
        runCurrent()
        assertEquals(1, speed.measurements)
        assertEquals(0.5f, viewModel.state.value.deviceSpeed)

        speed.measuring.value = true
        runCurrent()
        viewModel.onMeasureSpeedClicked()
        runCurrent()
        assertEquals(1, speed.measurements)
    }

    @Test
    fun `shows the transcript and its translation into the chosen language`() = runTest(dispatcher) {
        val viewModel = newViewModel(FakeRecorder(), saying("good morning"))
        viewModel.onTargetSelected(Language("fr"))

        viewModel.onRecordClicked()
        viewModel.onRecordClicked()
        runCurrent()

        val state = viewModel.state.value
        assertEquals("good morning", state.text)
        assertEquals("[en>fr] good morning", state.translation)
        assertEquals("Done", state.status)
        assertEquals(Phase.IDLE, state.phase)
    }

    @Test
    fun `the same language on both sides shows the transcript only`() = runTest(dispatcher) {
        val viewModel = newViewModel(FakeRecorder(), saying("good morning"))
        viewModel.onTargetSelected(Language("en"))

        viewModel.onRecordClicked()
        viewModel.onRecordClicked()
        runCurrent()

        assertEquals("good morning", viewModel.state.value.text)
        assertEquals("", viewModel.state.value.translation)
        assertEquals("Done", viewModel.state.value.status)
    }

    @Test
    fun `a failed translation keeps the transcript on screen`() = runTest(dispatcher) {
        translator.translateError = TranslationException("Translation failed")
        val viewModel = newViewModel(FakeRecorder(), saying("good morning"))

        viewModel.onRecordClicked()
        viewModel.onRecordClicked()
        runCurrent()

        val state = viewModel.state.value
        assertEquals("good morning", state.text)
        assertEquals("", state.translation)
        assertEquals("Translation failed", state.error)
        assertEquals(Phase.IDLE, state.phase)
    }

    @Test
    fun `a new recording clears the last result but keeps the languages`() = runTest(dispatcher) {
        val viewModel = newViewModel(FakeRecorder(), saying("good morning"))
        viewModel.onSourceSelected(Language("es"))
        viewModel.onTargetSelected(Language("fr"))
        viewModel.onRecordClicked()
        viewModel.onRecordClicked()
        runCurrent()
        assertEquals("[es>fr] good morning", viewModel.state.value.translation)

        viewModel.onRecordClicked()

        val state = viewModel.state.value
        assertEquals("", state.text)
        assertEquals("", state.translation)
        assertNull(state.details)
        assertEquals(Language("es"), state.source)
        assertEquals(Language("fr"), state.target)
    }

    @Test
    fun `languages cannot be changed during a recording`() = runTest(dispatcher) {
        val viewModel = newViewModel(FakeRecorder(), FakeTranscriber())

        viewModel.onRecordClicked()
        viewModel.onSourceSelected(Language("es"))
        viewModel.onTargetSelected(Language("fr"))

        assertEquals(Language.ENGLISH, viewModel.state.value.source)
        assertEquals(Language.SPANISH, viewModel.state.value.target)
    }

    @Test
    fun `shows the state of the speech model as it changes`() = runTest(dispatcher) {
        val viewModel = newViewModel(FakeRecorder(), FakeTranscriber())
        runCurrent()
        assertEquals(ModelState.Missing, viewModel.state.value.model)

        models.state.value = ModelState.Downloading(bytes = 10, total = 100)
        runCurrent()
        assertEquals(ModelState.Downloading(10, 100), viewModel.state.value.model)

        models.state.value = ModelState.Verifying
        runCurrent()
        assertEquals(ModelState.Verifying, viewModel.state.value.model)
    }

    @Test
    fun `the download button installs the model`() = runTest(dispatcher) {
        val viewModel = newViewModel(FakeRecorder(), FakeTranscriber())

        viewModel.onDownloadModelClicked()
        runCurrent()

        assertEquals(1, models.installCalls)
        assertTrue(viewModel.state.value.model is ModelState.Ready)
    }

    @Test
    fun `a failed download shows its reason and can be tried again`() = runTest(dispatcher) {
        models.failWith = "The download was interrupted"
        val viewModel = newViewModel(FakeRecorder(), FakeTranscriber())

        viewModel.onDownloadModelClicked()
        runCurrent()
        assertEquals(ModelState.Failed("The download was interrupted"), viewModel.state.value.model)

        models.failWith = null
        viewModel.onDownloadModelClicked()
        runCurrent()

        assertEquals(2, models.installCalls)
        assertTrue(viewModel.state.value.model is ModelState.Ready)
    }

    @Test
    fun `the download button does nothing while a download is running`() = runTest(dispatcher) {
        val viewModel = newViewModel(FakeRecorder(), FakeTranscriber())
        models.state.value = ModelState.Downloading(bytes = 10, total = 100)
        runCurrent()

        viewModel.onDownloadModelClicked()
        runCurrent()

        assertEquals(0, models.installCalls)
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
