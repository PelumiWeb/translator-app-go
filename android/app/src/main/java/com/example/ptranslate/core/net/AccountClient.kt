package com.example.ptranslate.core.net

import com.example.ptranslate.core.account.HandleCheck
import com.example.ptranslate.core.account.KeptVoice
import com.example.ptranslate.core.account.Profile
import kotlinx.serialization.SerialName
import kotlinx.serialization.Serializable
import okhttp3.MultipartBody
import okhttp3.RequestBody.Companion.toRequestBody

/** The result of signing in: the session's token, and whether the account was made just now. */
data class SignIn(val token: String, val profile: Profile, val newUser: Boolean)

/** The backend's account endpoints (ADR 0018). */
class AccountClient internal constructor(private val rest: Rest) {

    /**
     * Signs in with the ID token Google gave the phone. [nonce] is the raw nonce whose SHA-256 was given to Google;
     * the server checks that the token carries that hash.
     */
    suspend fun signInWithGoogle(idToken: String, nonce: String): SignIn {
        val result = rest.post("/v1/sessions/google", rest.body(SignInRequest(idToken, nonce)))
        val response = rest.decode<SignInResponse>(result)
        return SignIn(response.token, response.user, response.newUser)
    }

    /** The profile of the session [token] belongs to, before it is kept: used right after signing in. */
    suspend fun me(token: String): Profile =
        rest.decode(rest.send(rest.request("/v1/me").header("Authorization", "Bearer $token").build()))

    suspend fun me(): Profile = rest.decode(rest.get("/v1/me"))

    /** Changes the fields given; null leaves one as it is. */
    suspend fun updateProfile(displayName: String? = null, handle: String? = null, language: String? = null): Profile =
        rest.decode(rest.patch("/v1/me", rest.body(ProfileUpdate(displayName, handle, language))))

    suspend fun checkHandle(handle: String): HandleCheck = rest.decode(rest.get("/v1/handles/$handle"))

    /** Finds someone by their ID. Throws with code "user_not_found" when no one has it. */
    suspend fun findUser(handle: String): Profile = rest.decode(rest.get("/v1/users/$handle"))

    /** Keeps [wav] as the user's voice, with the version of the consent wording they agreed to. */
    suspend fun putVoice(wav: ByteArray, consentVersion: String): KeptVoice {
        val form = MultipartBody.Builder()
            .setType(MultipartBody.FORM)
            .addFormDataPart("consent", consentVersion)
            .addFormDataPart("voice", "voice.wav", wav.toRequestBody(Rest.WAV_TYPE))
            .build()
        return rest.decode(rest.send(rest.request("/v1/me/voice").put(form).build()))
    }

    /** The user's kept voice, or null if they keep none. */
    suspend fun voice(): KeptVoice? =
        try {
            rest.decode<KeptVoice>(rest.get("/v1/me/voice"))
        } catch (e: BackendException) {
            if (e.code == "no_voice") null else throw e
        }

    suspend fun voiceAudio(): ByteArray = rest.get("/v1/me/voice/audio").body

    suspend fun deleteVoice() {
        rest.delete("/v1/me/voice")
    }

    suspend fun signOut() {
        rest.delete("/v1/sessions/current")
    }

    suspend fun deleteAccount() {
        rest.delete("/v1/me")
    }
}

@Serializable
private data class SignInRequest(
    @SerialName("identity_token") val identityToken: String,
    val nonce: String,
)

@Serializable
private data class SignInResponse(
    val token: String,
    val user: Profile,
    @SerialName("new_user") val newUser: Boolean = false,
)

// A property left at its default, null, is not encoded at all (encodeDefaults
// is off), so the server receives only the fields being changed.
@Serializable
private data class ProfileUpdate(
    @SerialName("display_name") val displayName: String? = null,
    val handle: String? = null,
    val language: String? = null,
)
