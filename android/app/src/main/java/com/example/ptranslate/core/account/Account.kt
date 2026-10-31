package com.example.ptranslate.core.account

import android.content.SharedPreferences
import com.example.ptranslate.core.net.AccountClient
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.asStateFlow
import kotlinx.coroutines.flow.update

/**
 * The version of the consent wording on the voice screen. The server keeps it
 * with the voice, so it is known later what each person agreed to. Change it
 * whenever the wording changes.
 */
const val VOICE_CONSENT_VERSION = "2026-10-31"

/** How far the user has come through signing up on this phone. */
enum class OnboardingStep { NAME, HANDLE, LANGUAGE, VOICE, DONE }

/** Remembers [OnboardingStep] across launches. */
interface OnboardingProgress {
    var step: OnboardingStep
}

class SharedPreferencesOnboardingProgress(private val preferences: SharedPreferences) : OnboardingProgress {
    override var step: OnboardingStep
        get() = preferences.getString(KEY, null)?.let { saved -> OnboardingStep.entries.find { it.name == saved } }
            ?: OnboardingStep.NAME
        set(value) = preferences.edit().putString(KEY, value.name).apply()

    private companion object {
        const val KEY = "onboarding_step"
    }
}

/**
 * The signed-in user, if any, kept across launches, and what they can do with
 * their account. The single source of truth for "who is signed in": a
 * session that the server says has ended is forgotten here.
 */
class Account(
    private val client: AccountClient,
    private val store: SessionStore,
    private val progress: OnboardingProgress,
) {
    private val _session = MutableStateFlow(store.load())
    val session: StateFlow<Session?> = _session.asStateFlow()

    private val _step = MutableStateFlow(progress.step)

    /** Where the user is in signing up; [OnboardingStep.DONE] once they can use the app. */
    val step: StateFlow<OnboardingStep> = _step.asStateFlow()

    /** The session's token, for requests. */
    val token: String? get() = _session.value?.token

    val profile: Profile? get() = _session.value?.profile

    /**
     * Signs in with the ID token Google gave the phone; [nonce] is the raw nonce whose hash Google put in it. A new
     * account starts signing up from its name; an account already set up on another phone goes straight in.
     */
    suspend fun signInWithGoogle(idToken: String, nonce: String) {
        val signIn = client.signInWithGoogle(idToken, nonce)
        keep(Session(signIn.token, signIn.profile))
        moveTo(if (signIn.newUser || !signIn.profile.isComplete) OnboardingStep.NAME else OnboardingStep.DONE)
    }

    /** Fetches the profile again, as it is on the server. */
    suspend fun refresh() {
        update(client.me())
    }

    /** Changes the fields given; null leaves one as it is. Throws with code "handle_taken" for an ID someone has. */
    suspend fun updateProfile(displayName: String? = null, handle: String? = null, language: String? = null) {
        update(client.updateProfile(displayName, handle, language))
    }

    suspend fun checkHandle(handle: String): HandleCheck = client.checkHandle(handle)

    suspend fun findUser(handle: String): Profile = client.findUser(handle)

    suspend fun keptVoice(): KeptVoice? = client.voice()

    suspend fun keptVoiceAudio(): ByteArray = client.voiceAudio()

    suspend fun keepVoice(wav: ByteArray): KeptVoice = client.putVoice(wav, VOICE_CONSENT_VERSION)

    suspend fun deleteVoice() = client.deleteVoice()

    /** Records the next step of signing up as reached. */
    fun moveTo(step: OnboardingStep) {
        progress.step = step
        _step.value = step
    }

    /** Signs out here even if the server cannot be told; the session then ends when the account is deleted. */
    suspend fun signOut() {
        try {
            client.signOut()
        } finally {
            forget()
        }
    }

    /** Deletes the account on the server, with the voice and everything said, then signs out. */
    suspend fun deleteAccount() {
        client.deleteAccount()
        forget()
    }

    /** Forgets the session on this phone, as when the server says it has ended. */
    fun forget() {
        store.clear()
        _session.value = null
        moveTo(OnboardingStep.NAME)
    }

    private fun keep(session: Session) {
        store.save(session)
        _session.value = session
    }

    private fun update(profile: Profile) {
        _session.update { current ->
            current?.copy(profile = profile)?.also(store::save)
        }
    }
}
