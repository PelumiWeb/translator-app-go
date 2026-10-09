package com.example.ptranslate.core.pipeline

import com.example.ptranslate.core.FakeSynthesizer
import com.example.ptranslate.core.FakeTranslator
import com.example.ptranslate.core.Language
import com.example.ptranslate.core.audio.PcmAudio
import com.example.ptranslate.core.speech.SpeechException
import com.example.ptranslate.core.stt.Transcriber
import com.example.ptranslate.core.stt.Transcript
import com.example.ptranslate.core.stt.TranscriptEvent
import com.example.ptranslate.core.stt.TranscriptionException
import com.example.ptranslate.core.stt.TranscriptionStage
import com.example.ptranslate.core.translate.TranslationException
import kotlinx.coroutines.flow.Flow
import kotlinx.coroutines.flow.catch
import kotlinx.coroutines.flow.flow
import kotlinx.coroutines.flow.flowOf
import kotlinx.coroutines.flow.toList
import kotlinx.coroutines.runBlocking
import org.junit.Assert.assertEquals
import org.junit.Assert.assertThrows
import org.junit.Test

class SpeechTranslationPipelineTest {

    private val english = Language("en")
    private val spanish = Language("es")
    private val audio = PcmAudio(ShortArray(16_000))
    private val transcript = Transcript("hello world", confidence = 0.9f, Transcript.Source.ON_DEVICE)

    private val synthesizer = FakeSynthesizer()

    private val speaking = object : Transcriber {
        override fun transcribe(audio: PcmAudio, language: Language): Flow<TranscriptEvent> = flowOf(
            TranscriptEvent.StageChanged(TranscriptionStage.PROCESSING),
            TranscriptEvent.Partial("hello"),
            TranscriptEvent.Final(transcript),
        )
    }

    @Test
    fun `transcribes, then translates`() = runBlocking {
        val translator = FakeTranslator()

        val events = SpeechTranslationPipeline(speaking, translator, synthesizer).process(audio, english, spanish).toList()

        assertEquals(
            listOf(
                PipelineEvent.Transcribing(TranscriptionStage.PROCESSING),
                PipelineEvent.PartialTranscript("hello"),
                PipelineEvent.Transcribed(transcript),
                PipelineEvent.Translating,
                PipelineEvent.Translated("[en>es] hello world"),
                PipelineEvent.Speaking,
                PipelineEvent.Spoken,
            ),
            events,
        )
        assertEquals(listOf("translate en>es"), translator.calls)
        // Spoken in the listener's language, not the speaker's.
        assertEquals(listOf("es: [en>es] hello world"), synthesizer.spoken)
    }

    @Test
    fun `downloads the language pack first when it is missing`() = runBlocking {
        val translator = FakeTranslator(ready = false)

        val events = SpeechTranslationPipeline(speaking, translator, synthesizer).process(audio, english, spanish).toList()

        assertEquals(
            listOf(
                PipelineEvent.Transcribed(transcript),
                PipelineEvent.DownloadingLanguages,
                PipelineEvent.Translating,
                PipelineEvent.Translated("[en>es] hello world"),
                PipelineEvent.Speaking,
                PipelineEvent.Spoken,
            ),
            events.drop(2),
        )
        assertEquals(listOf("prepare en>es", "translate en>es"), translator.calls)
    }

    @Test
    fun `the same source and target language skips the translator`() = runBlocking {
        val translator = FakeTranslator(ready = false)

        val events = SpeechTranslationPipeline(speaking, translator, synthesizer).process(audio, english, english).toList()

        assertEquals(PipelineEvent.Translated("hello world"), events.last())
        assertEquals(emptyList<String>(), translator.calls)
        // Repeating someone's words back in their own language is not interpreting.
        assertEquals(emptyList<String>(), synthesizer.spoken)
    }

    @Test
    fun `a failed translation still delivers the transcript first`() = runBlocking {
        val translator = FakeTranslator().apply { translateError = TranslationException("Translation failed") }
        var failure: Throwable? = null

        val events = SpeechTranslationPipeline(speaking, translator, synthesizer)
            .process(audio, english, spanish)
            .catch { failure = it }
            .toList()

        assertEquals(PipelineEvent.Transcribed(transcript), events[2])
        assertEquals(PipelineEvent.Translating, events.last())
        assertEquals("Translation failed", failure?.message)
    }

    @Test
    fun `a failed language pack download is reported`() = runBlocking {
        val translator = FakeTranslator(ready = false).apply {
            prepareError = TranslationException("Could not download the language pack. Check the connection")
        }
        var failure: Throwable? = null

        val events = SpeechTranslationPipeline(speaking, translator, synthesizer)
            .process(audio, english, spanish)
            .catch { failure = it }
            .toList()

        assertEquals(PipelineEvent.DownloadingLanguages, events.last())
        assertEquals("Could not download the language pack. Check the connection", failure?.message)
        assertEquals(listOf("prepare en>es"), translator.calls)
    }

    @Test
    fun `with speaking turned off it ends at the translated text`() = runBlocking {
        val events = SpeechTranslationPipeline(speaking, FakeTranslator(), synthesizer)
            .process(audio, english, spanish, speak = false)
            .toList()

        assertEquals(PipelineEvent.Translated("[en>es] hello world"), events.last())
        assertEquals(emptyList<String>(), synthesizer.spoken)
    }

    @Test
    fun `a translation that cannot be spoken is still delivered as text`() = runBlocking {
        synthesizer.failure = SpeechException("This phone has no voice for Spanish")

        val events = SpeechTranslationPipeline(speaking, FakeTranslator(), synthesizer)
            .process(audio, english, spanish)
            .toList() // completes: not being able to speak is not a failed run

        assertEquals(
            listOf(
                PipelineEvent.Translated("[en>es] hello world"),
                PipelineEvent.Speaking,
                PipelineEvent.SpeechFailed("This phone has no voice for Spanish"),
            ),
            events.takeLast(3),
        )
    }

    @Test
    fun `a failed transcription never reaches the translator`() {
        val failing = object : Transcriber {
            override fun transcribe(audio: PcmAudio, language: Language): Flow<TranscriptEvent> =
                flow { throw TranscriptionException("No speech was recognised") }
        }
        val translator = FakeTranslator()

        val error = assertThrows(TranscriptionException::class.java) {
            runBlocking { SpeechTranslationPipeline(failing, translator, synthesizer).process(audio, english, spanish).toList() }
        }

        assertEquals("No speech was recognised", error.message)
        assertEquals(emptyList<String>(), translator.calls)
        assertEquals(emptyList<String>(), synthesizer.spoken)
    }
}
