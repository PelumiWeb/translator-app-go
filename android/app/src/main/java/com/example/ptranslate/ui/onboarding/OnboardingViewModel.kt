package com.example.ptranslate.ui.onboarding

import androidx.credentials.exceptions.GetCredentialCancellationException
import androidx.credentials.exceptions.GetCredentialException
import androidx.credentials.exceptions.NoCredentialException
import androidx.lifecycle.ViewModel
import androidx.lifecycle.ViewModelProvider
import androidx.lifecycle.ViewModelProvider.AndroidViewModelFactory.Companion.APPLICATION_KEY
import androidx.lifecycle.viewModelScope
import androidx.lifecycle.viewmodel.initializer
import androidx.lifecycle.viewmodel.viewModelFactory
import com.example.ptranslate.PtranslateApp
import com.example.ptranslate.core.Language
import com.example.ptranslate.core.account.Account
import com.example.ptranslate.core.account.Handles
import com.example.ptranslate.core.account.Nonce
import com.example.ptranslate.core.account.OnboardingStep
import com.example.ptranslate.core.audio.wavToPcm
import com.example.ptranslate.core.net.BackendException
import com.example.ptranslate.core.voice.VoiceSampleStore
import kotlinx.coroutines.CancellationException
import kotlinx.coroutines.Job
import kotlinx.coroutines.delay
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.asStateFlow
import kotlinx.coroutines.flow.update
import kotlinx.coroutines.launch

/** Where the ID being typed stands. */
sealed interface HandleState {
    data object Empty : HandleState

    /** It breaks a rule; [rule] says which. */
    data class Invalid(val rule: String) : HandleState
    data object Checking : HandleState
    data object Available : HandleState
    data class Taken(val suggestions: List<String>) : HandleState

    /** The server could not be asked. */
    data class Unchecked(val reason: String) : HandleState
}

data class OnboardingUiState(
    val signingIn: Boolean = false,
    val signInProblem: String? = null,
    val name: String = "",
    val handle: String = "",
    val handleState: HandleState = HandleState.Empty,
    val language: Language? = null,
    val saving: Boolean = false,
    val problem: String? = null,
    /** Editing from the profile: set once the change is saved, for the screen to close. */
    val saved: Boolean = false,
)

/**
 * Signing in and the first steps of signing up: the name, the ID, the
 * language. Each step is saved to the server before moving on, so a person
 * who stops half way picks up where they left off. With [editing], the same
 * steps change the profile later: saving closes the screen instead.
 */
class OnboardingViewModel(
    private val account: Account,
    private val voice: VoiceSampleStore,
    private val phoneLanguage: Language?,
    private val editing: Boolean = false,
    private val checkDelayMs: Long = 400,
) : ViewModel() {

    private val _state = MutableStateFlow(initialState())
    val state: StateFlow<OnboardingUiState> = _state.asStateFlow()

    private var check: Job? = null

    init {
        // A prefilled ID is checked at once, as if typed.
        if (_state.value.handle.isNotEmpty()) onHandleChanged(_state.value.handle)
    }

    private fun initialState(): OnboardingUiState {
        val profile = account.profile
        return OnboardingUiState(
            name = profile?.displayName.orEmpty(),
            handle = profile?.handle?.ifEmpty { null } ?: suggestedHandle(profile?.displayName.orEmpty()),
            language = profile?.lang ?: phoneLanguage,
        )
    }

    /**
     * Signs in with an ID token from [getToken], which shows Google's sheet. The nonce is made here and handed to
     * it, so its hash goes to Google and the raw value to the server.
     */
    fun onSignInClicked(getToken: suspend (Nonce) -> String) {
        if (_state.value.signingIn) return
        _state.update { it.copy(signingIn = true, signInProblem = null) }
        viewModelScope.launch {
            try {
                val nonce = Nonce.random()
                account.signInWithGoogle(getToken(nonce), nonce.raw)
                _state.value = initialState()
                if (_state.value.handle.isNotEmpty()) onHandleChanged(_state.value.handle)
                if (account.step.value == OnboardingStep.DONE) restoreVoice()
            } catch (e: CancellationException) {
                throw e
            } catch (e: Exception) {
                _state.update { it.copy(signingIn = false, signInProblem = signInProblem(e)) }
            }
        }
    }

    fun onNameChanged(name: String) {
        _state.update { it.copy(name = name.take(MAX_NAME), problem = null) }
    }

    fun onNameContinue() {
        val name = _state.value.name.trim()
        if (name.isEmpty()) return
        save(next = OnboardingStep.HANDLE) {
            account.updateProfile(displayName = name)
            if (_state.value.handle.isEmpty()) onHandleChanged(suggestedHandle(name))
        }
    }

    /** Checks the ID once typing pauses: the rules here, then whether it is free on the server. */
    fun onHandleChanged(input: String) {
        val handle = Handles.normalise(input).take(Handles.MAX + 1)
        check?.cancel()
        val rule = Handles.problemWith(handle)
        _state.update {
            it.copy(
                handle = handle,
                problem = null,
                handleState = when {
                    handle.isEmpty() -> HandleState.Empty
                    rule != null -> HandleState.Invalid(rule)
                    else -> HandleState.Checking
                },
            )
        }
        if (handle.isEmpty() || rule != null) return
        check = viewModelScope.launch {
            delay(checkDelayMs)
            val result = try {
                val answer = account.checkHandle(handle)
                if (answer.available) HandleState.Available else HandleState.Taken(answer.suggestions)
            } catch (e: BackendException) {
                HandleState.Unchecked(if (e.status == 0) "No connection, so this ID could not be checked." else "This ID could not be checked just now.")
            }
            _state.update { if (it.handle == handle) it.copy(handleState = result) else it }
        }
    }

    fun onHandleContinue() {
        val handle = _state.value.handle
        if (_state.value.handleState != HandleState.Available) return
        save(next = OnboardingStep.LANGUAGE) {
            try {
                account.updateProfile(handle = handle)
            } catch (e: BackendException) {
                if (e.code != "handle_taken") throw e
                // Taken between the check and now: show it as taken, with suggestions.
                val answer = account.checkHandle(handle)
                _state.update { it.copy(handleState = HandleState.Taken(answer.suggestions)) }
                throw StayHere()
            }
        }
    }

    fun onLanguagePicked(language: Language) {
        _state.update { it.copy(language = language, problem = null) }
    }

    fun onLanguageContinue() {
        val language = _state.value.language ?: return
        save(next = OnboardingStep.VOICE) { account.updateProfile(language = language.tag) }
    }

    /** Back to the step before [step]; from the first, back out to sign in again. */
    fun onBack(step: OnboardingStep) {
        when (step) {
            OnboardingStep.NAME -> viewModelScope.launch { account.signOut() }
            OnboardingStep.HANDLE -> account.moveTo(OnboardingStep.NAME)
            OnboardingStep.LANGUAGE -> account.moveTo(OnboardingStep.HANDLE)
            OnboardingStep.VOICE -> account.moveTo(OnboardingStep.LANGUAGE)
            OnboardingStep.DONE -> Unit
        }
    }

    private fun save(next: OnboardingStep, work: suspend () -> Unit) {
        if (_state.value.saving) return
        _state.update { it.copy(saving = true, problem = null) }
        viewModelScope.launch {
            try {
                work()
                if (editing) {
                    _state.update { it.copy(saving = false, saved = true) }
                } else {
                    account.moveTo(next)
                    _state.update { it.copy(saving = false) }
                }
            } catch (e: CancellationException) {
                throw e
            } catch (_: StayHere) {
                _state.update { it.copy(saving = false) }
            } catch (e: BackendException) {
                _state.update {
                    it.copy(saving = false, problem = if (e.status == 0) "No connection. Check you're online, then try again." else "That could not be saved just now. Please try again.")
                }
            }
        }
    }

    /** On a new phone, the voice kept on the server comes back, so the interpreter speaks in it too. */
    private suspend fun restoreVoice() {
        try {
            if (account.keptVoice() != null) voice.save(wavToPcm(account.keptVoiceAudio()))
        } catch (_: BackendException) {
            // Not worth stopping for: the profile shows the voice and can fetch it later.
        }
    }

    private fun signInProblem(e: Exception): String? = when {
        e is GetCredentialCancellationException -> "Sign-in was cancelled. Try again when you're ready."
        e is NoCredentialException -> "There's no Google account on this phone. Add one in Settings, then try again."
        e is GetCredentialException -> "Google sign-in didn't finish. Please try again."
        e is BackendException && e.status == 0 -> "No connection. Check you're online, then try again."
        e is BackendException && e.status == 501 -> "Sign in with Google isn't set up on the server yet."
        e is BackendException && e.code == "invalid_token" -> "That sign-in couldn't be checked. Please try again."
        else -> "Sign-in didn't work just now. Please try again."
    }

    /** Thrown by a step's work to stay on the step without showing a problem: it has shown its own. */
    private class StayHere : Exception()

    companion object {
        const val MAX_NAME = 50

        /** An ID to start from: the first name, as far as an ID can hold it. */
        fun suggestedHandle(name: String): String =
            name.trim().split(" ").firstOrNull().orEmpty().lowercase()
                .filter { it in 'a'..'z' || it in '0'..'9' }
                .take(Handles.MAX)
                .takeIf { it.length >= Handles.MIN }
                .orEmpty()

        val Factory: ViewModelProvider.Factory = viewModelFactory {
            initializer {
                val container = (this[APPLICATION_KEY] as PtranslateApp).container
                OnboardingViewModel(container.account, container.voice, container.phoneLanguage)
            }
        }

        /** The same steps, changing the profile of someone signed up. */
        val EditFactory: ViewModelProvider.Factory = viewModelFactory {
            initializer {
                val container = (this[APPLICATION_KEY] as PtranslateApp).container
                OnboardingViewModel(container.account, container.voice, container.phoneLanguage, editing = true)
            }
        }
    }
}
