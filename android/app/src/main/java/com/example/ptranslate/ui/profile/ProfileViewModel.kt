package com.example.ptranslate.ui.profile

import androidx.lifecycle.ViewModel
import androidx.lifecycle.ViewModelProvider
import androidx.lifecycle.ViewModelProvider.AndroidViewModelFactory.Companion.APPLICATION_KEY
import androidx.lifecycle.viewModelScope
import androidx.lifecycle.viewmodel.initializer
import androidx.lifecycle.viewmodel.viewModelFactory
import com.example.ptranslate.PtranslateApp
import com.example.ptranslate.core.account.Account
import com.example.ptranslate.core.account.KeptVoice
import com.example.ptranslate.core.account.Profile
import com.example.ptranslate.core.audio.AudioPlayer
import com.example.ptranslate.core.audio.wavToPcm
import com.example.ptranslate.core.net.BackendException
import com.example.ptranslate.core.voice.VoiceSampleState
import com.example.ptranslate.core.voice.VoiceSampleStore
import kotlinx.coroutines.CancellationException
import kotlinx.coroutines.Job
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.asStateFlow
import kotlinx.coroutines.flow.update
import kotlinx.coroutines.launch

data class ProfileUiState(
    val profile: Profile? = null,
    /** The voice kept on the server, once known. */
    val keptVoice: KeptVoice? = null,
    /** The length of the voice on this phone, if there is one. */
    val voiceMs: Long? = null,
    val playingVoice: Boolean = false,
    val busy: Boolean = false,
    val problem: String? = null,
)

/** The profile: who the user is, their voice, and their account. */
class ProfileViewModel(
    private val account: Account,
    private val voice: VoiceSampleStore,
    private val player: AudioPlayer,
) : ViewModel() {

    private val _state = MutableStateFlow(ProfileUiState(profile = account.profile))
    val state: StateFlow<ProfileUiState> = _state.asStateFlow()

    private var playback: Job? = null

    init {
        viewModelScope.launch { account.session.collect { session -> _state.update { it.copy(profile = session?.profile) } } }
        viewModelScope.launch {
            voice.state.collect { saved -> _state.update { it.copy(voiceMs = (saved as? VoiceSampleState.Present)?.durationMs) } }
        }
        refresh()
    }

    fun refresh() {
        viewModelScope.launch {
            try {
                account.refresh()
                _state.update { it.copy(keptVoice = account.keptVoice()) }
            } catch (_: BackendException) {
                // The profile kept on the phone stays on screen.
            }
        }
    }

    /** Plays the user's voice; on a phone that has not got it yet, fetches it from the server first. */
    fun onPlayVoiceClicked() {
        if (_state.value.playingVoice) {
            player.stop()
            return
        }
        playback = viewModelScope.launch {
            _state.update { it.copy(playingVoice = true, problem = null) }
            try {
                val wav = voice.loadWav() ?: account.keptVoiceAudio().also { voice.save(wavToPcm(it)) }
                player.play(wavToPcm(wav))
            } catch (e: CancellationException) {
                throw e
            } catch (e: Exception) {
                _state.update { it.copy(problem = "Your voice couldn't be played just now.") }
            } finally {
                _state.update { it.copy(playingVoice = false) }
            }
        }
    }

    /** Deletes the voice from the server and the phone. The user's messages then use the phone's voice. */
    fun onDeleteVoiceConfirmed() = busy("Your voice couldn't be deleted just now. Please try again.") {
        account.deleteVoice()
        voice.delete()
        _state.update { it.copy(keptVoice = null) }
    }

    fun onSignOutClicked() = busy("Signing out didn't work just now. Please try again.") {
        account.signOut()
    }

    /** Deletes the account and everything with it: the profile, the voice, the messages. */
    fun onDeleteAccountConfirmed() = busy("Your account couldn't be deleted just now. Please try again.") {
        account.deleteAccount()
    }

    private fun busy(problem: String, work: suspend () -> Unit) {
        if (_state.value.busy) return
        _state.update { it.copy(busy = true, problem = null) }
        viewModelScope.launch {
            try {
                work()
                _state.update { it.copy(busy = false) }
            } catch (e: CancellationException) {
                throw e
            } catch (e: BackendException) {
                _state.update { it.copy(busy = false, problem = problem) }
            }
        }
    }

    override fun onCleared() {
        player.stop()
    }

    companion object {
        val Factory: ViewModelProvider.Factory = viewModelFactory {
            initializer {
                val container = (this[APPLICATION_KEY] as PtranslateApp).container
                ProfileViewModel(container.account, container.voice, container.player)
            }
        }
    }
}
