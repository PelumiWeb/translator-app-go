package com.example.ptranslate.ui

import com.example.ptranslate.core.audio.AudioPlayer
import com.example.ptranslate.core.audio.AudioRecorder
import com.example.ptranslate.core.audio.PcmAudio
import com.example.ptranslate.core.audio.Recording
import com.example.ptranslate.core.voice.VoiceSampleState
import com.example.ptranslate.core.voice.VoiceSampleStore
import com.example.ptranslate.core.voice.VoiceSetupPreference
import kotlinx.coroutines.CompletableDeferred
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.ExperimentalCoroutinesApi
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.test.StandardTestDispatcher
import kotlinx.coroutines.test.advanceTimeBy
import kotlinx.coroutines.test.resetMain
import kotlinx.coroutines.test.runCurrent
import kotlinx.coroutines.test.runTest
import kotlinx.coroutines.test.setMain
import org.junit.After
import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertNull
import org.junit.Assert.assertTrue
import org.junit.Before
import org.junit.Test

@OptIn(ExperimentalCoroutinesApi::class)
class VoiceSetupViewModelTest {

    private fun speech(seconds: Int) = PcmAudio(ShortArray(seconds * 16_000) { if (it % 2 == 0) 2000 else -2000 })

    private class FakeRecorder(var audio: PcmAudio) : AudioRecorder {
        var cancelled = 0
        override fun start(): Recording = object : Recording {
            override suspend fun stop() = audio
            override fun cancel() {
                cancelled++
            }
        }
    }

    private class FakePlayer : AudioPlayer {
        val played = mutableListOf<PcmAudio>()
        var stops = 0
        var hold: CompletableDeferred<Unit>? = null

        override suspend fun play(audio: PcmAudio) {
            played += audio
            hold?.await()
        }

        override fun stop() {
            stops++
            hold?.complete(Unit)
        }
    }

    private class FakeStore : VoiceSampleStore {
        override val state = MutableStateFlow<VoiceSampleState>(VoiceSampleState.None)
        var saved: PcmAudio? = null

        override suspend fun save(audio: PcmAudio) {
            saved = audio
            state.value = VoiceSampleState.Present(audio.durationMs)
        }

        override suspend fun loadWav(): ByteArray? = null

        override suspend fun delete() {
            saved = null
            state.value = VoiceSampleState.None
        }
    }

    private class FakePreference(override var skipped: Boolean = false) : VoiceSetupPreference

    private val dispatcher = StandardTestDispatcher()
    private val recorder = FakeRecorder(speech(20))
    private val player = FakePlayer()
    private val store = FakeStore()
    private val preference = FakePreference()

    // The clock follows the test's virtual time, so "12 seconds pass" is exact.
    private fun viewModel() = VoiceSetupViewModel(
        recorder = recorder,
        player = player,
        samples = store,
        preference = preference,
        maxRecordingMs = 30_000,
        now = { dispatcher.scheduler.currentTime },
    )

    @Before
    fun setUp() {
        Dispatchers.setMain(dispatcher)
    }

    @After
    fun tearDown() {
        Dispatchers.resetMain()
    }

    @Test
    fun `record, review, and keep the recording`() = runTest(dispatcher) {
        val viewModel = viewModel()
        assertEquals(VoiceSetupStage.INTRO, viewModel.state.value.stage)

        viewModel.onRecordClicked()
        advanceTimeBy(12_050)
        assertEquals(VoiceSetupStage.RECORDING, viewModel.state.value.stage)
        assertEquals(12_000L, viewModel.state.value.elapsedMs)

        viewModel.onRecordClicked() // stop
        runCurrent()
        val review = viewModel.state.value
        assertEquals(VoiceSetupStage.REVIEW, review.stage)
        assertEquals(20_000L, review.recordedMs)
        assertNull(review.problem)
        assertNull("nothing is saved until the user says so", store.saved)

        viewModel.onUseClicked()
        runCurrent()

        assertEquals(recorder.audio, store.saved)
        assertTrue(viewModel.state.value.finished)
        assertFalse(preference.skipped)
    }

    @Test
    fun `a recording that is too short cannot be kept`() = runTest(dispatcher) {
        recorder.audio = speech(4)
        val viewModel = viewModel()

        viewModel.onRecordClicked()
        viewModel.onRecordClicked()
        runCurrent()

        assertTrue(viewModel.state.value.problem.orEmpty().startsWith("That was only 4 seconds"))

        viewModel.onUseClicked()
        runCurrent()

        assertNull(store.saved)
        assertFalse(viewModel.state.value.finished)
    }

    @Test
    fun `record again replaces the recording under review`() = runTest(dispatcher) {
        recorder.audio = speech(4)
        val viewModel = viewModel()
        viewModel.onRecordClicked()
        viewModel.onRecordClicked()
        runCurrent()

        recorder.audio = speech(18)
        viewModel.onRecordClicked() // "Record again"
        assertEquals(VoiceSetupStage.RECORDING, viewModel.state.value.stage)
        assertNull("the old problem is gone", viewModel.state.value.problem)
        viewModel.onRecordClicked()
        runCurrent()

        assertEquals(18_000L, viewModel.state.value.recordedMs)
        assertNull(viewModel.state.value.problem)
    }

    @Test
    fun `recording stops by itself at the time limit`() = runTest(dispatcher) {
        val viewModel = viewModel()

        viewModel.onRecordClicked()
        advanceTimeBy(29_950)
        assertEquals(VoiceSetupStage.RECORDING, viewModel.state.value.stage)

        advanceTimeBy(200)
        assertEquals(VoiceSetupStage.REVIEW, viewModel.state.value.stage)
    }

    @Test
    fun `listen plays the recording under review, and stops it on a second tap`() = runTest(dispatcher) {
        val viewModel = viewModel()
        viewModel.onRecordClicked()
        viewModel.onRecordClicked()
        runCurrent()
        // Set only now: starting a recording stops any playback, which would
        // otherwise release this before anything is playing.
        player.hold = CompletableDeferred()

        viewModel.onListenClicked()
        runCurrent()
        assertEquals(listOf(recorder.audio), player.played)
        assertTrue(viewModel.state.value.playing)

        viewModel.onListenClicked()
        runCurrent()
        assertTrue(player.stops >= 1)
        assertFalse(viewModel.state.value.playing)
    }

    @Test
    fun `not now leaves without saving and does not ask again`() = runTest(dispatcher) {
        val viewModel = viewModel()
        viewModel.onRecordClicked() // mid-recording when they change their mind

        viewModel.onSkipClicked()
        runCurrent()

        assertTrue(viewModel.state.value.finished)
        assertTrue(preference.skipped)
        assertNull(store.saved)
        assertEquals("the microphone is let go", 1, recorder.cancelled)
    }

    @Test
    fun `with a voice already saved, leaving keeps it and changes nothing`() = runTest(dispatcher) {
        store.save(speech(20))
        val viewModel = viewModel()
        runCurrent()
        assertEquals(20_000L, viewModel.state.value.savedMs)

        viewModel.onSkipClicked() // shown as "Keep it"
        runCurrent()

        assertTrue(viewModel.state.value.finished)
        assertFalse(preference.skipped)
        assertEquals(VoiceSampleState.Present(20_000), store.state.value)
    }

    @Test
    fun `deleting removes the saved voice and does not ask again`() = runTest(dispatcher) {
        store.save(speech(20))
        val viewModel = viewModel()
        runCurrent()

        viewModel.onDeleteClicked()
        runCurrent()

        assertEquals(VoiceSampleState.None, store.state.value)
        assertNull(viewModel.state.value.savedMs)
        assertTrue(preference.skipped)
        assertTrue(viewModel.state.value.finished)
    }

    @Test
    fun `starts fresh the next time it is opened`() = runTest(dispatcher) {
        val viewModel = viewModel()
        viewModel.onRecordClicked()
        viewModel.onRecordClicked()
        runCurrent()
        viewModel.onUseClicked()
        runCurrent()

        viewModel.onLeft()

        val state = viewModel.state.value
        assertEquals(VoiceSetupStage.INTRO, state.stage)
        assertFalse(state.finished)
        assertEquals("but it knows a voice is now saved", 20_000L, state.savedMs)
    }
}
