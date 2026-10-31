import Foundation
import Observation

/// A room: its voice notes, kept up to date by the feed, and the composer. The user's note is recorded, written down
/// on this phone, and sent with its words; the server translates it and speaks it in their voice.
@MainActor
@Observable
final class RoomViewModel {
    /// A note of the user's on its way, before the server has it.
    struct PendingNote: Identifiable, Equatable {
        enum Stage: Equatable {
            /// Turning the recording into words, on the phone or the server.
            case writing
            case sending
            /// It could not be sent; the words are kept, and it can be sent again.
            case failed
        }

        let id: String
        var text: String
        var stage: Stage
        let duration: Duration
    }

    /// Why a recording was not sent, shown above the mic.
    enum Hint: Equatable {
        case tooShort, silent, noSpeech, unclear
    }

    /// A voice note is at most 30 seconds; from 25, the composer counts down.
    static let maxNote = Duration.seconds(30)
    static let countdownFrom = Duration.seconds(25)
    /// Shorter than this is a tap, not a note.
    static let minNote = Duration.milliseconds(300)

    private(set) var room: Room?
    private(set) var messages: [Message] = []
    private(set) var pending: [PendingNote] = []
    private(set) var recordingStarted: Date?
    private(set) var hint: Hint?
    private(set) var micDenied = false
    /// The message playing now, and its length once it is audible.
    private(set) var playing: String?
    private(set) var playingLength: Duration?
    private(set) var played: Set<String> = []
    /// Their notes that arrived unseen and have not been played.
    private(set) var fresh: Set<String> = []
    /// Their notes showing the original words.
    private(set) var originals: Set<String> = []
    private(set) var loading = true
    private(set) var problem: String?

    var myLanguage: String? { account.profile?.language }
    var connected: Bool { feed.connected }
    var recording: Bool { recordingStarted != nil }

    @ObservationIgnored private let roomID: String
    @ObservationIgnored private let client: RoomsClient
    @ObservationIgnored private let account: Account
    @ObservationIgnored private let feed: RoomFeed
    @ObservationIgnored private let recorder: any AudioRecorder
    @ObservationIgnored private let transcriber: any Transcriber
    @ObservationIgnored private let player: MessagePlayer
    @ObservationIgnored private let requestMicrophone: () async -> Bool
    @ObservationIgnored private var activeRecording: (any Recording)?
    /// The timer that ends a recording, the note being sent, and the playback: owned here, cancelled on leaving.
    @ObservationIgnored private var autoStop: Task<Void, Never>?
    @ObservationIgnored private var sending: [String: Task<Void, Never>] = [:]
    @ObservationIgnored private var playback: Task<Void, Never>?
    /// The recordings of pending notes, for sending again.
    @ObservationIgnored private var wavs: [String: Data] = [:]

    init(
        roomID: String,
        client: RoomsClient,
        account: Account,
        feed: RoomFeed,
        recorder: any AudioRecorder,
        transcriber: any Transcriber,
        player: MessagePlayer,
        requestMicrophone: @escaping () async -> Bool
    ) {
        self.roomID = roomID
        self.client = client
        self.account = account
        self.feed = feed
        self.recorder = recorder
        self.transcriber = transcriber
        self.player = player
        self.requestMicrophone = requestMicrophone
    }

    isolated deinit {
        activeRecording?.cancel()
        autoStop?.cancel()
        playback?.cancel()
        sending.values.forEach { $0.cancel() }
        player.stop()
    }

    /// Loads the room, then follows its changes for as long as the screen shows. Run it from `.task`.
    func follow() async {
        await load(firstTime: true)
        for await change in feed.changes() {
            switch change {
            case .ready: await load(firstTime: false)
            case .changed(let id, let messageID) where id == roomID: await changed(messageID)
            case .changed: break
            }
        }
    }

    /// Starts recording, or stops and sends.
    func micTapped() {
        if activeRecording != nil {
            stopAndSend()
        } else {
            Task { await startRecording() }
        }
    }

    func cancelTapped() {
        autoStop?.cancel()
        activeRecording?.cancel()
        activeRecording = nil
        recordingStarted = nil
    }

    /// Plays a note, or stops it if it is playing. On the user's own note, it plays what the listener hears.
    func playTapped(_ message: Message) {
        if playing == message.id {
            player.stop()
            return
        }
        guard let rendition = message.mine ? message.delivery : message.rendition(for: myLanguage), rendition.status == .ready else { return }
        player.stop()
        playback?.cancel()
        playback = Task { [self] in
            playing = message.id
            playingLength = nil
            problem = nil
            do {
                try await player.play(rendition) { length in playingLength = length }
            } catch is CancellationError {
            } catch {
                problem = "That note couldn't be played just now."
            }
            if playing == message.id { playing = nil }
            played.insert(message.id)
            fresh.remove(message.id)
        }
    }

    func originalToggled(_ message: Message) {
        if originals.contains(message.id) { originals.remove(message.id) } else { originals.insert(message.id) }
    }

    /// Tries again to deliver a note the server could not translate.
    func retryTapped(_ message: Message) {
        sending[message.id] = Task { [self] in
            do {
                upsert(try await client.retry(message.id))
            } catch {
                problem = "That note couldn't be sent again just now."
            }
        }
    }

    /// Sends again a note that never reached the server.
    func retryPending(_ note: PendingNote) {
        guard let lang = myLanguage else { return }
        sending[note.id] = Task { await upload(note.id, text: note.text, duration: note.duration, lang: lang) }
    }

    func permissionChecked(granted: Bool) {
        if granted { micDenied = false }
    }

    private func startRecording() async {
        guard room?.other != nil else { return }
        guard await requestMicrophone() else {
            micDenied = true
            return
        }
        player.stop()
        do {
            activeRecording = try recorder.start()
        } catch {
            problem = "The microphone couldn't be started."
            return
        }
        hint = nil
        problem = nil
        recordingStarted = .now
        autoStop = Task { [weak self] in
            try? await Task.sleep(for: Self.maxNote)
            guard !Task.isCancelled else { return }
            self?.stopAndSend()
        }
    }

    private func stopAndSend() {
        guard let finished = activeRecording else { return }
        activeRecording = nil
        recordingStarted = nil
        autoStop?.cancel()
        let audio = finished.stop()
        if audio.duration < Self.minNote {
            hint = .tooShort
        } else if audio.isSilent {
            hint = .silent
        } else {
            send(audio)
        }
    }

    private func send(_ audio: PCMAudio) {
        guard let lang = myLanguage else { return }
        // One ID for the note: its place in the list, and the key that keeps a resend from sending it twice.
        let id = UUID().uuidString
        pending.append(PendingNote(id: id, text: "", stage: .writing, duration: audio.duration))
        sending[id] = Task { [self] in
            var text = ""
            do {
                for try await event in transcriber.transcribe(audio, language: Language(lang)) {
                    switch event {
                    case .partial(let words): updatePending(id) { $0.text = words.trimmingCharacters(in: .whitespaces) }
                    case .final(let transcript): text = transcript.text.trimmingCharacters(in: .whitespacesAndNewlines)
                    case .stageChanged: break
                    }
                }
            } catch TranscriptionError.silentAudio {
                drop(id, hint: .silent)
                return
            } catch TranscriptionError.noSpeech {
                drop(id, hint: .noSpeech)
                return
            } catch is CancellationError {
                return
            } catch {
                drop(id, hint: .unclear)
                return
            }
            guard !text.isEmpty else {
                drop(id, hint: .noSpeech)
                return
            }
            wavs[id] = audio.wavData()
            await upload(id, text: text, duration: audio.duration, lang: lang)
        }
    }

    private func upload(_ id: String, text: String, duration: Duration, lang: String) async {
        guard let wav = wavs[id] else { return }
        updatePending(id) {
            $0.text = text
            $0.stage = .sending
        }
        do {
            let message = try await client.sendMessage(
                roomID: roomID, wav: wav, transcript: text,
                durationMS: Int(duration / .milliseconds(1)), lang: lang, idempotencyKey: id
            )
            wavs[id] = nil
            pending.removeAll { $0.id == id }
            upsert(message)
        } catch {
            updatePending(id) { $0.stage = .failed }
            if (error as? BackendError)?.code == "room_closed" {
                problem = "This person has left, so the note can't reach them."
            }
        }
        sending[id] = nil
    }

    private func load(firstTime: Bool) async {
        do {
            let room = try await client.room(roomID)
            let latest = try await client.messages(roomID: roomID)
            self.room = room
            if firstTime {
                // Opening the room: the unread notes are the newest of theirs.
                fresh = Set(latest.filter { !$0.mine }.suffix(room.unread).map(\.id))
            }
            messages = Self.merge(messages, latest)
            problem = nil
            if room.unread > 0 { try? await client.markRead(roomID: roomID) }
        } catch is CancellationError {
        } catch let error as BackendError {
            problem = error.status == 0 ? "No connection. Notes will appear when you're back online." : "This room couldn't be loaded just now."
        } catch {}
        loading = false
    }

    private func changed(_ messageID: String?) async {
        guard let messageID else {
            await load(firstTime: false)
            return
        }
        guard let message = try? await client.message(messageID) else { return }
        let isNew = !message.mine && !messages.contains { $0.id == message.id }
        upsert(message)
        if isNew {
            fresh.insert(message.id)
            try? await client.markRead(roomID: roomID)
        }
    }

    private func upsert(_ message: Message) {
        messages = Self.merge(messages, [message])
    }

    private func updatePending(_ id: String, _ change: (inout PendingNote) -> Void) {
        guard let index = pending.firstIndex(where: { $0.id == id }) else { return }
        change(&pending[index])
    }

    private func drop(_ id: String, hint: Hint) {
        pending.removeAll { $0.id == id }
        self.hint = hint
        sending[id] = nil
    }

    /// Newer versions replace older ones; the list stays in the order the notes were sent.
    static func merge(_ current: [Message], _ incoming: [Message]) -> [Message] {
        var byID = Dictionary(current.map { ($0.id, $0) }, uniquingKeysWith: { $1 })
        for message in incoming { byID[message.id] = message }
        return byID.values.sorted { $0.createdAt < $1.createdAt }
    }
}
