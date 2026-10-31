package com.example.ptranslate.core.account

import com.example.ptranslate.core.Language
import kotlinx.serialization.SerialName
import kotlinx.serialization.Serializable

/** A person with an account: the signed-in user, or someone they talk to. */
@Serializable
data class Profile(
    val id: String,
    @SerialName("display_name") val displayName: String = "",
    /** The ID others find them by, without the "@"; empty until chosen. */
    val handle: String = "",
    /** The language they speak, as a tag such as "es"; empty until chosen. */
    val language: String = "",
) {
    /** Signed up far enough to talk to people: an ID to be found by and a language to hear in. */
    val isComplete: Boolean get() = handle.isNotEmpty() && language.isNotEmpty()

    val lang: Language? get() = language.takeIf { it.isNotEmpty() }?.let(::Language)
}

/** The signed-in user. [token] is the session's secret, sent with every request. */
data class Session(val token: String, val profile: Profile)

/** Keeps the session across launches. */
interface SessionStore {
    fun load(): Session?
    fun save(session: Session)
    fun clear()
}

/** What the server said about an ID the user wants. */
@Serializable
data class HandleCheck(
    val handle: String,
    val available: Boolean,
    /** Free IDs like it, when it is taken. */
    val suggestions: List<String> = emptyList(),
)

/** The voice the user keeps on the server, and the consent they gave for it. */
@Serializable
data class KeptVoice(
    @SerialName("consent_version") val consentVersion: String,
    @SerialName("consented_at") val consentedAt: String,
)

/** The rules an ID must follow, checked on the phone before asking the server. */
object Handles {
    const val MIN = 3
    const val MAX = 20

    /** What breaks the rules, in words for the person, or null if [handle] follows them. */
    fun problemWith(handle: String): String? = when {
        handle.any { !(it in 'a'..'z' || it in '0'..'9' || it == '.' || it == '_') } ->
            "Only letters, numbers, dots and underscores."
        handle.length < MIN -> "At least $MIN characters."
        handle.length > MAX -> "At most $MAX characters."
        else -> null
    }

    /** What the person typed, as an ID: no "@", no spaces, lower case. */
    fun normalise(input: String): String = input.trim().removePrefix("@").lowercase()
}
