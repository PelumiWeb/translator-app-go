package com.example.ptranslate.core.stt

import com.example.ptranslate.core.Language
import com.example.ptranslate.core.audio.PcmAudio
import kotlinx.coroutines.flow.Flow
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.flowOf
import kotlinx.coroutines.flow.single
import kotlinx.coroutines.runBlocking
import org.junit.Assert.assertEquals
import org.junit.Test

class RoutingTranscriberTest {

    private class Canned(private val text: String, private val source: Transcript.Source) : Transcriber {
        override fun transcribe(audio: PcmAudio, language: Language): Flow<TranscriptEvent> =
            flowOf(TranscriptEvent.Final(Transcript(text, confidence = null, source)))
    }

    private val route = MutableStateFlow(TranscriptionRoute.ON_DEVICE)
    private val router = RoutingTranscriber(
        onDevice = Canned("from the device", Transcript.Source.ON_DEVICE),
        cloud = Canned("from the cloud", Transcript.Source.CLOUD),
        route = route,
    )

    private fun transcribe(): Transcript = runBlocking {
        (router.transcribe(PcmAudio(ShortArray(16)), Language.ENGLISH).single() as TranscriptEvent.Final).transcript
    }

    @Test
    fun `follows the route for each new recording`() {
        assertEquals("from the device", transcribe().text)

        route.value = TranscriptionRoute.CLOUD
        assertEquals("from the cloud", transcribe().text)

        route.value = TranscriptionRoute.ON_DEVICE
        assertEquals("from the device", transcribe().text)
    }
}
