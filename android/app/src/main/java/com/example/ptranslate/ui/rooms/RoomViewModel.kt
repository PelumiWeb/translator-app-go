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
import com.example.ptranslate.core.audio.toWav
import com.example.ptranslate.core.net.BackendException
import com.example.ptranslate.core.net.RoomsClient
import com.example.ptranslate.core.rooms.Message
import com.example.ptranslate.core.rooms.MessagePlayer
import com.example.ptranslate.core.rooms.Rendition
import com.example.ptranslate.core.rooms.RenditionStatus
import com.example.ptranslate.core.rooms.Room
import com.example.ptranslate.core.rooms.RoomChange
import com.example.ptranslate.core.rooms.RoomFeed
import com.example.ptranslate.core.rooms.audible
import com.example.ptranslate.core.stt.SilentAudioException
import com.example.ptranslate.core.stt.TranscriptEvent
import com.example.ptranslate.core.stt.Transcriber
import com.example.ptranslate.core.stt.TranscriptionException
import kotlinx.coroutines.CancellationException
import kotlinx.coroutines.Job
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.asStateFlow
import kotlinx.coroutines.flow.update
import kotlinx.coroutines.launch
import java.util.UUID

/** A message of the user's on its way, before the server has it: a voice note, or typed text. */
data class PendingNote(
    val id: String,
    val text: String,
    val stage: PendingStage,
    val durationMs: Long,
    val isText: Boolean = false,
)

enum class PendingStage {
    /** Turning the recording into words, on the phone or the server. */
    WRITING,
    SENDING,

    /** It could not be sent; the words are kept, and it can be sent again. */
    FAILED,
}

/** Why a recording was not sent, shown above the composer. */
enum class RecordingHint { TOO_SHORT, SILENT, NO_SPEECH, UNCLEAR }

data class RoomUiState(
    val room: Room? = null,
    val messages: List<Message> = emptyList(),
    val pending: List<PendingNote> = emptyList(),
    /** The voice note being recorded, paused, or stopped to listen to. */
    val note: NoteState = NoteState(),
    /** The stopped note is playing, to be listened to before it is sent. */
    val reviewing: Boolean = false,
    /** The stopped note's dots. */
    val reviewLevels: List<Float> = emptyList(),
    /** What the user is typing. */
    val draft: String = "",
    val hint: RecordingHint? = null,
    val micDenied: Boolean = false,
    /** The message playing now, and its length once it is audible. */
    val playing: String? = null,
    val playingMs: Long? = null,
    val played: Set<String> = emptySet(),
    /** Their voice notes that arrived unseen and have not been played. */
    val fresh: Set<String> = emptySet(),
    /** Their messages showing the original words. */
    val originals: Set<String> = emptySet(),
    /** Text messages the user asked to hear, whose voice is being made: each plays once it is. */
    val waitingToHear: Set<String> = emptySet(),
    val connected: Boolean = true,
    val loading: Boolean = true,
    val problem: String? = null,
)

/**
 * A room: its messages, kept up to date by the feed, and the composer. A voice note is recorded (with pauses, and a
 * listen before sending if the user stops it), written down on this phone, and sent with its words; a text message is
 * sent as typed. The server translates both, speaks a voice note in the sender's voice, and a text message when
 * someone asks to hear it.
 */
class RoomViewModel(
    private val roomId: String,
    private val rooms: RoomsClient,
    private val account: Account,
    private val feed: RoomFeed,
    recorder: AudioRecorder,
    private val transcriber: Transcriber,
    private val player: MessagePlayer,
    maxRecordingMs: Long = MAX_NOTE_MS,
    now: () -> Long = { System.nanoTime() / 1_000_000 },
) : ViewModel() {

    private val _state = MutableStateFlow(RoomUiState())
    val state: StateFlow<RoomUiState> = _state.asStateFlow()

    private val notes = NoteRecorder(recorder, viewModelScope, maxRecordingMs, now)
    private var playback: Job? = null
    private var review: Job? = null

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
        viewModelScope.launch {
            notes.state.collect { note ->
                val levels = if (note.phase == NotePhase.STOPPED) notes.audio?.levels(REVIEW_DOTS).orEmpty() else emptyList()
                _state.update { it.copy(note = note, reviewLevels = levels) }
            }
        }
    }

    /** Starts recording a voice note. */
    fun onMicClicked() {
        if (_state.value.room?.other == null || _state.value.note.phase != NotePhase.IDLE) return
        player.stop()
        try {
            notes.start()
        } catch (e: Exception) {
            _state.update { it.copy(problem = "The microphone couldn't be started.") }
            return
        }
        _state.update { it.copy(hint = null, problem = null) }
    }

    fun onPauseClicked() = notes.pause()

    fun onResumeClicked() = notes.resume()

    /** Ends the recording without sending it, to listen to first. */
    fun onStopClicked() {
        viewModelScope.launch {
            notes.stop()
            val hint = notes.audio?.let(::hintFor) ?: return@launch
            notes.cancel()
            _state.update { it.copy(hint = hint) }
        }
    }

    /** Sends the voice note, whether it is recording, paused or stopped. */
    fun onSendNoteClicked() {
        stopReview()
        viewModelScope.launch {
            val audio = notes.finish() ?: return@launch
            val hint = hintFor(audio)
            if (hint != null) _state.update { it.copy(hint = hint) } else send(audio)
        }
    }

    /** Throws the voice note away. */
    fun onCancelClicked() {
        stopReview()
        notes.cancel()
    }

    /** Plays the stopped note, or stops it playing. */
    fun onReviewPlayClicked() {
        if (_state.value.reviewing) {
            stopReview()
            return
        }
        val audio = notes.audio ?: return
        player.stop()
        review = viewModelScope.launch {
            _state.update { it.copy(reviewing = true, playing = null) }
            try {
                player.playRecording(audio)
            } finally {
                _state.update { it.copy(reviewing = false) }
            }
        }
    }

    fun onDraftChanged(text: String) {
        _state.update { it.copy(draft = text.take(MAX_TEXT_CHARS)) }
    }

    /** Sends what the user typed. */
    fun onSendTextClicked() {
        val text = _state.value.draft.trim()
        val lang = myLanguage ?: return
        if (text.isEmpty() || _state.value.room?.other == null) return
        // One ID for the message: its place in the list, and the key that keeps a resend from sending it twice.
        val id = UUID.randomUUID().toString()
        _state.update { it.copy(draft = "", hint = null, pending = it.pending + PendingNote(id, text, PendingStage.SENDING, 0, isText = true)) }
        viewModelScope.launch { uploadText(id, text, lang) }
    }

    fun onPermissionDenied() {
        _state.update { it.copy(micDenied = true) }
    }

    fun onPermissionGranted() {
        _state.update { it.copy(micDenied = false) }
    }

    /** Plays a message, or stops it if it is playing. On the user's own, it plays what the listener hears. */
    fun onPlayClicked(message: Message) {
        if (_state.value.playing == message.id) {
            player.stop()
            return
        }
        val rendition = heard(message) ?: return
        if (rendition.status != RenditionStatus.READY) return
        player.stop()
        review?.cancel()
        playback?.cancel()
        playback = viewModelScope.launch {
            _state.update { it.copy(playing = message.id, playingMs = null, problem = null) }
            try {
                player.play(rendition) { ms -> _state.update { it.copy(playingMs = ms) } }
            } catch (e: CancellationException) {
                throw e
            } catch (e: Exception) {
                _state.update { it.copy(problem = "That message couldn't be played just now.") }
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

    /**
     * Plays a text message in the sender's voice, or stops it. The first time anyone asks, the server makes the voice
     * first; it plays as soon as it is ready.
     */
    fun onListenClicked(message: Message) {
        val rendition = heard(message) ?: return
        when {
            _state.value.playing == message.id || rendition.audible -> onPlayClicked(message)
            message.id in _state.value.waitingToHear -> _state.update { it.copy(waitingToHear = it.waitingToHear - message.id) }
            rendition.status == RenditionStatus.SPEAKING -> _state.update { it.copy(waitingToHear = it.waitingToHear + message.id) }
            rendition.status == RenditionStatus.READY -> {
                _state.update { it.copy(waitingToHear = it.waitingToHear + message.id, problem = null) }
                viewModelScope.launch {
                    try {
                        upsert(rooms.requestSpeech(message.id, rendition.lang))
                    } catch (e: BackendException) {
                        _state.update { it.copy(waitingToHear = it.waitingToHear - message.id, problem = "That message couldn't be spoken just now.") }
                    }
                }
            }
        }
    }

    fun onOriginalToggled(message: Message) {
        _state.update { it.copy(originals = if (message.id in it.originals) it.originals - message.id else it.originals + message.id) }
    }

    /** Tries again to deliver a message the server could not translate. */
    fun onRetryClicked(message: Message) {
        viewModelScope.launch {
            try {
                upsert(rooms.retry(message.id))
            } catch (e: BackendException) {
                _state.update { it.copy(problem = "That message couldn't be sent again just now.") }
            }
        }
    }

    /** Sends again a message that never reached the server. */
    fun onRetryPending(note: PendingNote) {
        val lang = myLanguage ?: return
        viewModelScope.launch {
            if (note.isText) uploadText(note.id, note.text, lang) else upload(note.id, note.text, note.durationMs, lang)
        }
    }

    private fun hintFor(audio: PcmAudio): RecordingHint? = when {
        audio.durationMs < MIN_NOTE_MS -> RecordingHint.TOO_SHORT
        audio.isSilent() -> RecordingHint.SILENT
        else -> null
    }

    private fun stopReview() {
        if (review?.isActive == true) player.stop()
        review?.cancel()
    }

    /** What the user hears of a message: on their own, what the listener gets; on someone else's, their language. */
    private fun heard(message: Message): Rendition? = if (message.mine) message.delivery else message.renditionFor(myLanguage)

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
            notSent(id, e)
        }
    }

    private suspend fun uploadText(id: String, text: String, lang: String) {
        updatePending(id) { it.copy(stage = PendingStage.SENDING) }
        try {
            val message = rooms.sendText(roomId, text, lang, idempotencyKey = id)
            _state.update { it.copy(pending = it.pending.filterNot { note -> note.id == id }) }
            upsert(message)
        } catch (e: BackendException) {
            notSent(id, e)
        }
    }

    private fun notSent(id: String, e: BackendException) {
        updatePending(id) { it.copy(stage = PendingStage.FAILED) }
        if (e.code == "room_closed") _state.update { it.copy(problem = "This person has left, so the message can't reach them.") }
    }

    private suspend fun load(firstTime: Boolean) {
        try {
            val room = rooms.room(roomId)
            val messages = rooms.messages(roomId)
            _state.update { current ->
                // Opening the room: the unread notes are the newest of theirs.
                // Only voice notes keep a "new" dot until played; a text is read as it shows.
                val fresh = if (firstTime) {
                    messages.filterNot { it.mine }.takeLast(room.unread).filterNot { it.isText }.mapTo(HashSet()) { it.id }
                } else {
                    current.fresh
                }
                current.copy(room = room, messages = merge(current.messages, messages), fresh = fresh, loading = false, problem = null)
            }
            messages.forEach(::playIfWaited)
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
                if (!message.isText) _state.update { it.copy(fresh = it.fresh + message.id) }
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
        playIfWaited(message)
    }

    /** Plays a text message the user asked to hear, once its voice is made. */
    private fun playIfWaited(message: Message) {
        if (message.id !in _state.value.waitingToHear || heard(message)?.audible != true) return
        _state.update { it.copy(waitingToHear = it.waitingToHear - message.id) }
        onPlayClicked(message)
    }

    private fun updatePending(id: String, change: (PendingNote) -> PendingNote) {
        _state.update { state -> state.copy(pending = state.pending.map { if (it.id == id) change(it) else it }) }
    }

    private fun dropPending(id: String, hint: RecordingHint) {
        _state.update { state -> state.copy(pending = state.pending.filterNot { it.id == id }, hint = hint) }
    }

    override fun onCleared() {
        notes.cancel()
        player.stop()
    }

    companion object {
        /** A voice note is at most 30 seconds. */
        const val MAX_NOTE_MS = 30_000L

        /** As the server allows. */
        const val MAX_TEXT_CHARS = 1000

        private const val REVIEW_DOTS = 22

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
