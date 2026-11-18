package com.example.ptranslate.ui.rooms

import com.example.ptranslate.core.audio.AudioRecorder
import com.example.ptranslate.core.audio.PcmAudio
import com.example.ptranslate.core.audio.Recording
import kotlinx.coroutines.ExperimentalCoroutinesApi
import kotlinx.coroutines.test.TestScope
import kotlinx.coroutines.test.advanceTimeBy
import kotlinx.coroutines.test.runCurrent
import kotlinx.coroutines.test.runTest
import org.junit.Assert.assertEquals
import org.junit.Assert.assertNull
import org.junit.Assert.assertSame
import org.junit.Assert.assertTrue
import org.junit.Test

@OptIn(ExperimentalCoroutinesApi::class)
class NoteRecorderTest {

    /** Records [audio], whatever happens; notes what it was asked. */
    private class FakeRecorder(val audio: PcmAudio = PcmAudio(ShortArray(48_000))) : AudioRecorder {
        val asked = mutableListOf<String>()

        override fun start(): Recording {
            asked += "start"
            return object : Recording {
                override fun pause() {
                    asked += "pause"
                }

                override fun resume() {
                    asked += "resume"
                }

                override suspend fun stop(): PcmAudio {
                    asked += "stop"
                    return audio
                }

                override fun cancel() {
                    asked += "cancel"
                }
            }
        }
    }

    private val recorder = FakeRecorder()

    private fun TestScope.notes() = NoteRecorder(recorder, backgroundScope, maxMs = 30_000, now = { testScheduler.currentTime })

    @Test
    fun `pausing stops the clock and resuming carries it on`() = runTest {
        val notes = notes()
        notes.start()
        advanceTimeBy(3_000)
        runCurrent()

        notes.pause()
        advanceTimeBy(5_000)
        runCurrent()
        assertEquals(NoteState(NotePhase.PAUSED, 3_000), notes.state.value)

        notes.resume()
        advanceTimeBy(2_000)
        runCurrent()
        assertEquals(NoteState(NotePhase.RECORDING, 5_000), notes.state.value)
        assertEquals(listOf("start", "pause", "resume"), recorder.asked)
    }

    @Test
    fun `stopping ends the recording, to listen to before sending`() = runTest {
        val notes = notes()
        notes.start()
        advanceTimeBy(3_000)

        notes.stop()

        assertEquals(NoteState(NotePhase.STOPPED, 3_000), notes.state.value)
        assertSame(recorder.audio, notes.audio)
        assertEquals(listOf("start", "stop"), recorder.asked)
    }

    @Test
    fun `finishing hands over the note from any phase`() = runTest {
        val notes = notes()
        notes.start()
        assertSame(recorder.audio, notes.finish())
        assertEquals(NoteState(), notes.state.value)

        notes.start()
        notes.pause()
        assertSame(recorder.audio, notes.finish())

        notes.start()
        notes.stop()
        assertSame(recorder.audio, notes.finish())
        assertNull(notes.audio)
    }

    @Test
    fun `at the limit the note stops and waits to be sent`() = runTest {
        val notes = notes()
        notes.start()

        advanceTimeBy(30_100)
        runCurrent()

        assertEquals(NotePhase.STOPPED, notes.state.value.phase)
        assertSame(recorder.audio, notes.audio)
    }

    @Test
    fun `time paused does not count toward the limit`() = runTest {
        val notes = notes()
        notes.start()
        advanceTimeBy(20_000)
        notes.pause()
        advanceTimeBy(20_000)
        notes.resume()
        advanceTimeBy(5_000)
        runCurrent()

        assertEquals(NoteState(NotePhase.RECORDING, 25_000), notes.state.value)
    }

    @Test
    fun `cancelling throws the note away`() = runTest {
        val notes = notes()
        notes.start()
        notes.pause()

        notes.cancel()

        assertEquals(NoteState(), notes.state.value)
        assertNull(notes.audio)
        assertTrue("cancel" in recorder.asked)
        assertNull(notes.finish())
    }
}
