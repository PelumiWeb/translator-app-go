package com.example.ptranslate.core.rooms

import com.example.ptranslate.core.Language
import com.example.ptranslate.core.audio.AudioPlayer
import com.example.ptranslate.core.audio.PcmAudio
import com.example.ptranslate.core.audio.wavToPcm
import com.example.ptranslate.core.net.RoomsClient
import com.example.ptranslate.core.speech.SpeechSynthesizer
import kotlinx.coroutines.CoroutineDispatcher
import kotlinx.coroutines.withContext

/**
 * Plays a voice message as a listener hears it: the audio the server made in
 * the sender's voice, or their own recording, or when the sender keeps no
 * voice, the translation read by this phone.
 */
class MessagePlayer(
    private val rooms: RoomsClient,
    private val player: AudioPlayer,
    private val phoneVoice: SpeechSynthesizer,
    private val decodeDispatcher: CoroutineDispatcher,
) {
    // The last few clips, so playing a note again does not fetch it again.
    private val cache = object : LinkedHashMap<String, PcmAudio>(8, 0.75f, true) {
        override fun removeEldestEntry(eldest: MutableMap.MutableEntry<String, PcmAudio>) = size > 8
    }

    /**
     * Plays [rendition] and returns when it has finished or been stopped. [onStart] is called when it becomes
     * audible, with its length when that is known (the phone's voice does not say).
     */
    suspend fun play(rendition: Rendition, onStart: (durationMs: Long?) -> Unit = {}) {
        val url = rendition.audioUrl
        if (url != null) {
            val audio = synchronized(cache) { cache[url] } ?: withContext(decodeDispatcher) {
                wavToPcm(rooms.audio(url))
            }.also { synchronized(cache) { cache[url] = it } }
            onStart(audio.durationMs)
            player.play(audio)
        } else if (rendition.text.isNotEmpty()) {
            phoneVoice.speak(rendition.text, Language(rendition.lang)) { onStart(null) }
        }
    }

    fun stop() {
        player.stop()
        phoneVoice.stop()
    }
}
