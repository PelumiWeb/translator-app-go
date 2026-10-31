package com.example.ptranslate.ui.rooms

import androidx.lifecycle.ViewModel
import androidx.lifecycle.ViewModelProvider
import androidx.lifecycle.ViewModelProvider.AndroidViewModelFactory.Companion.APPLICATION_KEY
import androidx.lifecycle.viewModelScope
import androidx.lifecycle.viewmodel.initializer
import androidx.lifecycle.viewmodel.viewModelFactory
import com.example.ptranslate.PtranslateApp
import com.example.ptranslate.core.Language
import com.example.ptranslate.core.account.Account
import com.example.ptranslate.core.audio.AudioRecorder
import com.example.ptranslate.core.audio.PcmAudio
import com.example.ptranslate.core.audio.Recording
import com.example.ptranslate.core.audio.toWav
import com.example.ptranslate.core.net.BackendException
import com.example.ptranslate.core.net.RoomsClient
import com.example.ptranslate.core.rooms.Message
import com.example.ptranslate.core.rooms.MessagePlayer
import com.example.ptranslate.core.rooms.RenditionStatus
import com.example.ptranslate.core.rooms.Room
import com.example.ptranslate.core.rooms.RoomChange
import com.example.ptranslate.core.rooms.RoomFeed
import com.example.ptranslate.core.stt.SilentAudioException
import com.example.ptranslate.core.stt.TranscriptEvent
import com.example.ptranslate.core.stt.Transcriber
import com.example.ptranslate.core.stt.TranscriptionException
import kotlinx.coroutines.CancellationException
import kotlinx.coroutines.Job
import kotlinx.coroutines.delay
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.asStateFlow
import kotlinx.coroutines.flow.update
import kotlinx.coroutines.launch
import java.util.UUID

/** A note of the user's on its way, before the server has it. */
data class PendingNote(val id: String, val text: String, val stage: PendingStage, val durationMs: Long)

enum class PendingStage {
    /** Turning the recording into words, on the phone or the server. */
    WRITING,
    SENDING,

    /** It could not be sent; the words are kept, and it can be sent again. */
    FAILED,
}

/** Why a recording was not sent, shown above the mic. */
enum class RecordingHint { TOO_SHORT, SILENT, NO_SPEECH, UNCLEAR }

data class RoomUiState(
    val room: Room? = null,
    val messages: List<Message> = emptyList(),
    val pending: List<PendingNote> = emptyList(),
    val recording: Boolean = false,
    val elapsedMs: Long = 0,
    val hint: RecordingHint? = null,
    val micDenied: Boolean = false,
    /** The message playing now, and its length once it is audible. */
    val playing: String? = null,
    val playingMs: Long? = null,
    val played: Set<String> = emptySet(),
    /** Their notes that arrived unseen and have not been played. */
    val fresh: Set<String> = emptySet(),
    /** Their notes showing the original words. */
    val originals: Set<String> = emptySet(),
    val connected: Boolean = true,
    val loading: Boolean = true,
    val problem: String? = null,
)

/**
 * A room: its voice notes, kept up to date by the feed, and the composer.
 * The user's note is recorded, written down on this phone, and sent with its
 * words; the server translates it and speaks it in their voice.
 */
class RoomViewModel(
    private val roomId: String,
    private val rooms: RoomsClient,
    private val account: Account,
    private val feed: RoomFeed,
    private val recorder: AudioRecorder,
    private val transcriber: Transcriber,
    private val player: MessagePlayer,
    private val maxRecordingMs: Long = MAX_NOTE_MS,
    private val now: () -> Long = { System.nanoTime() / 1_000_000 },
) : ViewModel() {

    private val _state = MutableStateFlow(RoomUiState())
    val state: StateFlow<RoomUiState> = _state.asStateFlow()

    private var recording: Recording? = null
    private var timer: Job? = null
    private var playback: Job? = null

    /** The recordings of pending notes, by note, for sending again. */
    private val wavs = mutableMapOf<String, ByteArray>()

    val myLanguage: String? get() = account.profile?.language

    init {
        viewModelScope.launch { load(firstTime = true) }
        viewModelScope.launch {
            feed.changes.collect { change ->
                when (change) {
                    RoomChange.Ready -> load(firstTime = false)
                    is RoomChange.Changed -> if (change.roomId == roomId) onChanged(change.messageId)
                }
            }
        }
        viewModelScope.launch { feed.connected.collect { connected -> _state.update { it.copy(connected = connected) } } }
    }

    /** Starts recording, or stops and sends. */
    fun onMicClicked() {
        if (recording != null) stopAndSend() else startRecording()
    }

    fun onCancelClicked() {
        timer?.cancel()
        recording?.cancel()
        recording = null
        _state.update { it.copy(recording = false, elapsedMs = 0) }
    }

    fun onPermissionDenied() {
        _state.update { it.copy(micDenied = true) }
    }

    fun onPermissionGranted() {
        _state.update { it.copy(micDenied = false) }
    }

    /** Plays a note, or stops it if it is playing. On the user's own note, it plays what the listener hears. */
    fun onPlayClicked(message: Message) {
        if (_state.value.playing == message.id) {
            player.stop()
            return
        }
        val rendition = (if (message.mine) message.delivery else message.renditionFor(myLanguage)) ?: return
        if (rendition.status != RenditionStatus.READY) return
        player.stop()
        playback?.cancel()
        playback = viewModelScope.launch {
            _state.update { it.copy(playing = message.id, playingMs = null, problem = null) }
            try {
                player.play(rendition) { ms -> _state.update { it.copy(playingMs = ms) } }
            } catch (e: CancellationException) {
                throw e
            } catch (e: Exception) {
                _state.update { it.copy(problem = "That note couldn't be played just now.") }
            } finally {
                _state.update {
                    it.copy(
                        playing = if (it.playing == message.id) null else it.playing,
                        played = it.played + message.id,
                        fresh = it.fresh - message.id,
                    )
                }
            }
        }
    }

    fun onOriginalToggled(message: Message) {
        _state.update { it.copy(originals = if (message.id in it.originals) it.originals - message.id else it.originals + message.id) }
    }

    /** Tries again to deliver a note the server could not translate. */
    fun onRetryClicked(message: Message) {
        viewModelScope.launch {
            try {
                upsert(rooms.retry(message.id))
            } catch (e: BackendException) {
                _state.update { it.copy(problem = "That note couldn't be sent again just now.") }
            }
        }
    }

    /** Sends again a note that never reached the server. */
    fun onRetryPending(note: PendingNote) {
        val lang = myLanguage ?: return
        viewModelScope.launch { upload(note.id, note.text, note.durationMs, lang) }
    }

    private fun startRecording() {
        if (_state.value.room?.other == null) return
        player.stop()
        recording = try {
            recorder.start()
        } catch (e: Exception) {
            _state.update { it.copy(problem = "The microphone couldn't be started.") }
            return
        }
        _state.update { it.copy(recording = true, elapsedMs = 0, hint = null, problem = null) }
        val startedAt = now()
        timer = viewModelScope.launch {
            while (true) {
                val elapsed = now() - startedAt
                _state.update { it.copy(elapsedMs = elapsed) }
                if (elapsed >= maxRecordingMs) {
                    stopAndSend()
                    return@launch
                }
                delay(100)
            }
        }
    }

    private fun stopAndSend() {
        val finished = recording ?: return
        recording = null
        _state.update { it.copy(recording = false) }
        viewModelScope.launch {
            val audio = finished.stop()
            when {
                audio.durationMs < MIN_NOTE_MS -> _state.update { it.copy(hint = RecordingHint.TOO_SHORT) }
                audio.isSilent() -> _state.update { it.copy(hint = RecordingHint.SILENT) }
                else -> send(audio)
            }
        }
        // Cancelled last: when the time limit stopped it, the caller is the timer itself.
        timer?.cancel()
    }

    private suspend fun send(audio: PcmAudio) {
        val lang = myLanguage ?: return
        // One ID for the note: its place in the list, and the key that keeps a resend from sending it twice.
        val id = UUID.randomUUID().toString()
        _state.update { it.copy(pending = it.pending + PendingNote(id, "", PendingStage.WRITING, audio.durationMs)) }

        val text = try {
            var finalText = ""
            transcriber.transcribe(audio, Language(lang)).collect { event ->
                when (event) {
                    is TranscriptEvent.Partial -> updatePending(id) { it.copy(text = event.text.trim()) }
                    is TranscriptEvent.Final -> finalText = event.transcript.text.trim()
                    is TranscriptEvent.StageChanged -> Unit
                }
            }
            finalText
        } catch (e: SilentAudioException) {
            dropPending(id, RecordingHint.SILENT)
            return
        } catch (e: TranscriptionException) {
            dropPending(id, RecordingHint.UNCLEAR)
            return
        }
        if (text.isEmpty()) {
            dropPending(id, RecordingHint.NO_SPEECH)
            return
        }
        wavs[id] = audio.toWav()
        upload(id, text, audio.durationMs, lang)
    }

    private suspend fun upload(id: String, text: String, durationMs: Long, lang: String) {
        val wav = wavs[id] ?: return
        updatePending(id) { it.copy(text = text, stage = PendingStage.SENDING) }
        try {
            val message = rooms.sendMessage(roomId, wav, text, durationMs, lang, idempotencyKey = id)
            wavs.remove(id)
            _state.update { it.copy(pending = it.pending.filterNot { note -> note.id == id }) }
            upsert(message)
        } catch (e: BackendException) {
            updatePending(id) { it.copy(stage = PendingStage.FAILED) }
            if (e.code == "room_closed") _state.update { it.copy(problem = "This person has left, so the note can't reach them.") }
        }
    }

    private suspend fun load(firstTime: Boolean) {
        try {
            val room = rooms.room(roomId)
            val messages = rooms.messages(roomId)
            _state.update { current ->
                // Opening the room: the unread notes are the newest of theirs.
                val fresh = if (firstTime) {
                    messages.filterNot { it.mine }.takeLast(room.unread).mapTo(HashSet()) { it.id }
                } else {
                    current.fresh
                }
                current.copy(room = room, messages = merge(current.messages, messages), fresh = fresh, loading = false, problem = null)
            }
            if (room.unread > 0) markRead()
        } catch (e: CancellationException) {
            throw e
        } catch (e: BackendException) {
            _state.update { it.copy(loading = false, problem = if (e.status == 0) "No connection. Notes will appear when you're back online." else "This room couldn't be loaded just now.") }
        }
    }

    private suspend fun onChanged(messageId: String?) {
        if (messageId == null) {
            load(firstTime = false)
            return
        }
        try {
            val message = rooms.message(messageId)
            val isNew = !message.mine && _state.value.messages.none { it.id == message.id }
            upsert(message)
            if (isNew) {
                _state.update { it.copy(fresh = it.fresh + message.id) }
                markRead()
            }
        } catch (e: BackendException) {
            // A note deleted meanwhile, or the network dropped: the next "ready" fetches everything.
        }
    }

    private suspend fun markRead() {
        try {
            rooms.markRead(roomId)
        } catch (_: BackendException) {
            // The dot in the list stays a little longer; nothing else depends on it.
        }
    }

    private fun upsert(message: Message) {
        _state.update { it.copy(messages = merge(it.messages, listOf(message))) }
    }

    private fun updatePending(id: String, change: (PendingNote) -> PendingNote) {
        _state.update { state -> state.copy(pending = state.pending.map { if (it.id == id) change(it) else it }) }
    }

    private fun dropPending(id: String, hint: RecordingHint) {
        _state.update { state -> state.copy(pending = state.pending.filterNot { it.id == id }, hint = hint) }
    }

    override fun onCleared() {
        recording?.cancel()
        player.stop()
    }

    companion object {
        /** A voice note is at most 30 seconds. */
        const val MAX_NOTE_MS = 30_000L

        /** Shorter than this is a tap, not a note. */
        const val MIN_NOTE_MS = 300L

        /** From here on, the composer counts down the seconds left. */
        const val COUNTDOWN_FROM_MS = 25_000L

        /** Newer versions replace older ones; the list stays in the order the notes were sent. */
        fun merge(current: List<Message>, incoming: List<Message>): List<Message> =
            (current.associateBy { it.id } + incoming.associateBy { it.id }).values.sortedBy { it.createdAt }

        fun factory(roomId: String): ViewModelProvider.Factory = viewModelFactory {
            initializer {
                val container = (this[APPLICATION_KEY] as PtranslateApp).container
                RoomViewModel(
                    roomId = roomId,
                    rooms = container.rooms,
                    account = container.account,
                    feed = container.roomFeed,
                    recorder = container.recorder,
                    transcriber = container.transcriber,
                    player = container.messagePlayer,
                )
            }
        }
    }
}
