package com.example.ptranslate.core.rooms

import com.example.ptranslate.core.account.Profile
import kotlinx.serialization.SerialName
import kotlinx.serialization.Serializable

/** A conversation, as the signed-in user sees it. For now it has two people. */
@Serializable
data class Room(
    val id: String,
    /** Everyone in it but the user. Empty when the other person has deleted their account. */
    val people: List<Profile> = emptyList(),
    @SerialName("last_message") val lastMessage: Message? = null,
    /** Messages from the others the user has not seen. */
    val unread: Int = 0,
    @SerialName("created_at") val createdAt: String = "",
) {
    /** The other person, in a room of two. */
    val other: Profile? get() = people.firstOrNull()
}

/** A message as it was sent, a voice note or typed text, and what it has become for its listeners. */
@Serializable
data class Message(
    val id: String,
    @SerialName("room_id") val roomId: String,
    @SerialName("sender_id") val senderId: String,
    val mine: Boolean,
    /** "voice" or "text". A string, not an enum, so a kind from a newer server still reads. */
    val kind: String = KIND_VOICE,
    /** The language it was said or typed in. */
    val lang: String,
    /** What was said, or for a text message, typed. */
    val transcript: String,
    /** 0 for a text message. */
    @SerialName("duration_ms") val durationMs: Long,
    @SerialName("created_at") val createdAt: String,
    /** The sender sees one per language the others speak; a listener sees their own only. */
    val renditions: List<Rendition> = emptyList(),
) {
    /** Typed words, heard only when someone asks: its voice is made then. */
    val isText: Boolean get() = kind == KIND_TEXT

    companion object {
        const val KIND_VOICE = "voice"
        const val KIND_TEXT = "text"
    }
}

/** A message for the listeners of one language: translated, then spoken in the sender's voice. */
@Serializable
data class Rendition(
    val lang: String,
    val status: RenditionStatus = RenditionStatus.TRANSLATING,
    /** The translation, once made. */
    val text: String = "",
    /** Who speaks it, once ready. */
    val voice: RenditionVoice = RenditionVoice.NONE,
    /** Where to fetch the audio, when there is audio to fetch. */
    @SerialName("audio_url") val audioUrl: String? = null,
)

@Serializable
enum class RenditionStatus {
    @SerialName("translating") TRANSLATING,
    @SerialName("speaking") SPEAKING,
    @SerialName("ready") READY,
    @SerialName("failed") FAILED,
}

@Serializable
enum class RenditionVoice {
    /** Not ready yet. */
    NONE,

    /** The sender's own voice, made for this language. */
    @SerialName("own") OWN,

    /** The sender's recording, for listeners of the language it was said in. */
    @SerialName("original") ORIGINAL,

    /** The sender keeps no voice: the listener's phone reads the text. */
    @SerialName("phone") PHONE,
}

/** Whether there is something to hear, now: a voice note ready, or a text message whose voice has been made. */
val Rendition.audible: Boolean get() = status == RenditionStatus.READY && voice != RenditionVoice.NONE

/** What the server's stream of room changes says. */
sealed interface RoomChange {
    /** The stream is open: anything fetched from now on is at least as new as what it will report. */
    data object Ready : RoomChange

    /** Something in a room changed: a new message, or [messageId] moved on. */
    data class Changed(val roomId: String, val messageId: String?) : RoomChange
}
