package com.example.ptranslate.ui.rooms

import androidx.lifecycle.ViewModel
import androidx.lifecycle.ViewModelProvider
import androidx.lifecycle.ViewModelProvider.AndroidViewModelFactory.Companion.APPLICATION_KEY
import androidx.lifecycle.viewModelScope
import androidx.lifecycle.viewmodel.initializer
import androidx.lifecycle.viewmodel.viewModelFactory
import com.example.ptranslate.PtranslateApp
import com.example.ptranslate.core.account.Account
import com.example.ptranslate.core.account.Handles
import com.example.ptranslate.core.account.Profile
import com.example.ptranslate.core.net.BackendException
import com.example.ptranslate.core.net.RoomsClient
import com.example.ptranslate.core.rooms.Room
import com.example.ptranslate.core.rooms.RoomFeed
import kotlinx.coroutines.CancellationException
import kotlinx.coroutines.Job
import kotlinx.coroutines.channels.Channel
import kotlinx.coroutines.delay
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.asStateFlow
import kotlinx.coroutines.flow.receiveAsFlow
import kotlinx.coroutines.flow.update
import kotlinx.coroutines.launch

/** Looking someone up by ID, in the "Add someone" sheet. */
sealed interface Lookup {
    data object Idle : Lookup
    data object Searching : Lookup
    data class Found(val person: Profile) : Lookup
    data class NotFound(val reason: String) : Lookup
}

data class RoomsUiState(
    val loading: Boolean = true,
    val rooms: List<Room> = emptyList(),
    val query: String = "",
    val connected: Boolean = true,
    val problem: String? = null,
    val lookupHandle: String = "",
    val lookup: Lookup = Lookup.Idle,
    val opening: Boolean = false,
    /** Set once a room has been opened from the sheet, for the screen to go to it. */
    val opened: Room? = null,
) {
    val shown: List<Room>
        get() = if (query.isBlank()) rooms else rooms.filter { room ->
            room.people.any { it.displayName.contains(query, ignoreCase = true) || it.handle.contains(query, ignoreCase = true) }
        }
}

/** The user's rooms, kept up to date by the feed, and adding someone new. */
class RoomsViewModel(
    private val rooms: RoomsClient,
    private val account: Account,
    private val feed: RoomFeed,
    private val lookupDelayMs: Long = 400,
) : ViewModel() {

    private val _state = MutableStateFlow(RoomsUiState())
    val state: StateFlow<RoomsUiState> = _state.asStateFlow()

    // Changes often come in bursts (a message, then its translation, then its
    // voice); a conflated channel turns a burst into one or two reloads.
    private val reloads = Channel<Unit>(Channel.CONFLATED)
    private var lookupJob: Job? = null

    init {
        viewModelScope.launch { reloads.receiveAsFlow().collect { load() } }
        viewModelScope.launch { feed.changes.collect { reloads.trySend(Unit) } }
        viewModelScope.launch { feed.connected.collect { connected -> _state.update { it.copy(connected = connected) } } }
        reloads.trySend(Unit)
    }

    val me: Profile? get() = account.profile

    fun refresh() {
        reloads.trySend(Unit)
    }

    fun onQueryChanged(query: String) {
        _state.update { it.copy(query = query) }
    }

    /** Looks the ID up once typing pauses. */
    fun onLookupChanged(input: String) {
        val handle = Handles.normalise(input)
        lookupJob?.cancel()
        _state.update { it.copy(lookupHandle = handle, lookup = if (Handles.problemWith(handle) == null) Lookup.Searching else Lookup.Idle) }
        if (Handles.problemWith(handle) != null) return
        lookupJob = viewModelScope.launch {
            delay(lookupDelayMs)
            val result = when {
                handle == account.profile?.handle -> Lookup.NotFound("That's your own ID.")
                else -> try {
                    Lookup.Found(account.findUser(handle))
                } catch (e: BackendException) {
                    when {
                        e.code == "user_not_found" -> Lookup.NotFound("No one has the ID @$handle yet. Check the spelling with them.")
                        e.status == 0 -> Lookup.NotFound("No connection, so this ID couldn't be looked up.")
                        else -> Lookup.NotFound("This ID couldn't be looked up just now.")
                    }
                }
            }
            _state.update { if (it.lookupHandle == handle) it.copy(lookup = result) else it }
        }
    }

    /** Opens the room with the person found. It opens at once: their first note from you reaches them straight away. */
    fun onAddClicked() {
        val person = (_state.value.lookup as? Lookup.Found)?.person ?: return
        if (_state.value.opening) return
        _state.update { it.copy(opening = true) }
        viewModelScope.launch {
            try {
                val room = rooms.openRoom(person.handle)
                _state.update { it.copy(opening = false, opened = room, lookupHandle = "", lookup = Lookup.Idle) }
                reloads.trySend(Unit)
            } catch (e: BackendException) {
                _state.update { it.copy(opening = false, lookup = Lookup.NotFound("The room couldn't be opened just now. Please try again.")) }
            }
        }
    }

    fun onOpenedHandled() {
        _state.update { it.copy(opened = null) }
    }

    fun onSheetClosed() {
        lookupJob?.cancel()
        _state.update { it.copy(lookupHandle = "", lookup = Lookup.Idle) }
    }

    private suspend fun load() {
        try {
            val list = rooms.rooms()
            _state.update { it.copy(loading = false, rooms = list, problem = null) }
        } catch (e: CancellationException) {
            throw e
        } catch (e: BackendException) {
            _state.update { it.copy(loading = false, problem = if (e.status == 0) "No connection. Your rooms will appear when you're back online." else "Your rooms couldn't be loaded just now.") }
        }
    }

    companion object {
        val Factory: ViewModelProvider.Factory = viewModelFactory {
            initializer {
                val container = (this[APPLICATION_KEY] as PtranslateApp).container
                RoomsViewModel(container.rooms, container.account, container.roomFeed)
            }
        }
    }
}
