package com.example.ptranslate.core.speech

import com.example.ptranslate.core.Language
import com.example.ptranslate.core.audio.AudioPlayer
import com.example.ptranslate.core.audio.wavToPcm
import com.example.ptranslate.core.net.BackendClient
import com.example.ptranslate.core.net.BackendException
import com.example.ptranslate.core.net.followJob
import com.example.ptranslate.core.net.retrying
import com.example.ptranslate.core.voice.VoiceSampleStore
import java.io.IOException
import java.util.UUID

/**
 * Speaks in the user's own voice: sends the recorded voice sample and the
 * text to the backend, which imitates the voice, then plays the audio that
 * comes back. Every step survives a connection that drops, as transcription
 * does.
 *
 * The sample goes with every request, and the server deletes it when the job
 * ends: it keeps no voices.
 *
 * @param maxAttempts how many times in a row a step may fail before giving up.
 * @param retryDelayMs how long to wait before try number `attempt + 1`.
 */
class RemoteSpeechSynthesizer(
    private val backend: BackendClient,
    private val voice: VoiceSampleStore,
    private val player: AudioPlayer,
    private val maxAttempts: Int = 5,
    private val retryDelayMs: (attempt: Int) -> Long = { attempt -> attempt * 1000L },
) : SpeechSynthesizer {

    override suspend fun speak(text: String, language: Language, onStart: () -> Unit) {
        val sample = try {
            voice.loadWav()
        } catch (e: IOException) {
            throw SpeechException("Your voice recording could not be read", e)
        } ?: throw SpeechException("No voice has been recorded")

        val wav = try {
            // The same key on every attempt, as for an upload to transcribe.
            val idempotencyKey = UUID.randomUUID().toString()
            val job = retrying(maxAttempts, retryDelayMs) {
                backend.createSpeech(sample, text, language, idempotencyKey)
            }
            backend.followJob(job.id, maxAttempts, retryDelayMs) {}
            retrying(maxAttempts, retryDelayMs) { backend.speechAudio(job.audioUrl) }
        } catch (e: BackendException) {
            throw SpeechException(e.message ?: "The server could not speak the translation", e)
        }

        val audio = try {
            wavToPcm(wav)
        } catch (e: IllegalArgumentException) {
            throw SpeechException("The server sent audio that cannot be played", e)
        }
        onStart()
        player.play(audio)
    }

    override fun stop() = player.stop()
}
