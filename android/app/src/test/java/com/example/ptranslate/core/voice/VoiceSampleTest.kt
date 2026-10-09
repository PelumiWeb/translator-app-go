package com.example.ptranslate.core.voice

import com.example.ptranslate.core.audio.PcmAudio
import com.example.ptranslate.core.audio.wavToPcm
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.runBlocking
import org.junit.Assert.assertEquals
import org.junit.Assert.assertNull
import org.junit.Assert.assertTrue
import org.junit.Rule
import org.junit.Test
import org.junit.rules.TemporaryFolder
import java.io.File

class VoiceSampleTest {

    @get:Rule
    val folder = TemporaryFolder()

    /** Audio of the given length that is loud enough to count as speech. */
    private fun speech(seconds: Int) = PcmAudio(ShortArray(seconds * 16_000) { if (it % 2 == 0) 2000 else -2000 })

    private fun store(directory: File = File(folder.root, "voice")) = FileVoiceSampleStore(directory, Dispatchers.IO)

    // --- what makes a recording usable ---

    @Test
    fun `a clear recording of the passage is accepted`() {
        assertNull(VoiceSampleRules.problemWith(speech(20)))
        assertNull("exactly the minimum", VoiceSampleRules.problemWith(speech(10)))
    }

    @Test
    fun `a recording that is too short says how long it was`() {
        assertEquals(
            "That was only 4 seconds. About 20 are needed: please read the whole passage",
            VoiceSampleRules.problemWith(speech(4)),
        )
    }

    @Test
    fun `a silent recording is reported as a microphone problem, whatever its length`() {
        val silence = PcmAudio(ShortArray(20 * 16_000))

        assertEquals("Nothing was heard. Check the microphone and try again", VoiceSampleRules.problemWith(silence))
    }

    // --- keeping it on the phone ---

    @Test
    fun `starts with no recording`() = runBlocking {
        val store = store()

        assertEquals(VoiceSampleState.None, store.state.value)
        assertNull(store.loadWav())
    }

    @Test
    fun `saves the recording as a WAV file that can be read back`() = runBlocking {
        val store = store()
        val audio = speech(12)

        store.save(audio)

        assertEquals(VoiceSampleState.Present(12_000), store.state.value)
        val wav = store.loadWav()!!
        assertEquals(audio.samples.toList(), wavToPcm(wav).samples.toList())
    }

    @Test
    fun `the recording is still there after the app restarts`() = runBlocking {
        val directory = File(folder.root, "voice")
        store(directory).save(speech(15))

        val afterRestart = store(directory)

        assertEquals(VoiceSampleState.Present(15_000), afterRestart.state.value)
        assertTrue(afterRestart.loadWav()!!.size > 44)
    }

    @Test
    fun `a new recording replaces the old one`() = runBlocking {
        val store = store()
        store.save(speech(12))

        store.save(speech(20))

        assertEquals(VoiceSampleState.Present(20_000), store.state.value)
        assertEquals(20 * 16_000, wavToPcm(store.loadWav()!!).samples.size)
        // Only the one file: no leftovers from the first, no temporary file.
        assertEquals(listOf("voice-sample.wav"), File(folder.root, "voice").list()!!.toList())
    }

    @Test
    fun `deleting removes it from the phone`() = runBlocking {
        val directory = File(folder.root, "voice")
        val store = store(directory)
        store.save(speech(12))

        store.delete()

        assertEquals(VoiceSampleState.None, store.state.value)
        assertNull(store.loadWav())
        assertEquals(emptyList<String>(), directory.list()!!.toList())
        store.delete() // deleting when there is nothing is harmless
    }
}
