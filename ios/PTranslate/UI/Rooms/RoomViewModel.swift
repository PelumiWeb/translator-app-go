import Foundation
import Observation

/// A room: its messages, kept up to date by the feed, and the composer. A voice note is recorded (with pauses, and a
/// listen before sending if the user stops it), written down on this phone, and sent with its words; a text message is
/// sent as typed. The server translates both, speaks a voice note in the sender's voice, and a text message when
/// someone asks to hear it.
@MainActor
@Observable
final class RoomViewModel {
    /// A message of the user's on its way, before the server has it: a voice note, or typed text.
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
        var isText = false
    }

    /// Why a recording was not sent, shown above the composer.
    enum Hint: Equatable {
        case tooShort, silent, noSpeech, unclear
    }

    /// What the user is reporting: one of the other person's messages, or the person.
    enum ReportTarget: Identifiable, Equatable {
        case message(Message)
        case person(Profile)

        var id: String {
            switch self {
            case .message(let message): "message-\(message.id)"
            case .person(let person): "person-\(person.id)"
            }
        }
    }

    /// A voice note is at most 30 seconds; from 25, the composer counts down.
    static let maxNote = Duration.seconds(30)
    static let countdownFrom = Duration.seconds(25)
    /// Shorter than this is a tap, not a note.
    static let minNote = Duration.milliseconds(300)
    /// As the server allows.
    static let maxTextCharacters = 1000

    private(set) var room: Room?
    private(set) var messages: [Message] = []
    private(set) var pending: [PendingNote] = []
    /// The voice note being recorded, paused, or stopped to listen to.
    let notes: NoteRecorder
    /// The stopped note is playing, to be listened to before it is sent.
    private(set) var reviewing = false
    /// What the user is typing.
    var draft = "" {
        didSet { if draft.count > Self.maxTextCharacters { draft = String(draft.prefix(Self.maxTextCharacters)) } }
    }
    private(set) var hint: Hint?
    private(set) var micDenied = false
    /// The message playing now, and its length once it is audible.
    private(set) var playing: String?
    private(set) var playingLength: Duration?
    private(set) var played: Set<String> = []
    /// Their voice notes that arrived unseen and have not been played.
    private(set) var fresh: Set<String> = []
    /// Their messages showing the original words.
    private(set) var originals: Set<String> = []
    /// Text messages the user asked to hear, whose voice is being made: each plays once it is.
    private(set) var waitingToHear: Set<String> = []
    private(set) var loading = true
    private(set) var problem: String?
    /// What the report sheet is open for; nil when it is closed.
    var reporting: ReportTarget?
    /// A report was sent: the screen says thank you, and when it will be looked at.
    var reportSent = false

    var myLanguage: String? { account.profile?.language }
    /// The user blocked the other person: the composer gives way to a way to unblock.
    var youBlocked: Bool { room?.youBlocked == true }
    var connected: Bool { feed.connected }

    @ObservationIgnored private let roomID: String
    @ObservationIgnored private let client: RoomsClient
    @ObservationIgnored private let account: Account
    @ObservationIgnored private let feed: RoomFeed
    @ObservationIgnored private let transcriber: any Transcriber
    @ObservationIgnored private let player: MessagePlayer
    @ObservationIgnored private let requestMicrophone: () async -> Bool
    /// The notes being sent, the playback and the listen before sending: owned here, cancelled on leaving.
    @ObservationIgnored private var sending: [String: Task<Void, Never>] = [:]
    @ObservationIgnored private var playback: Task<Void, Never>?
    @ObservationIgnored private var review: Task<Void, Never>?
    @ObservationIgnored private var blocking: Task<Void, Never>?
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
        self.notes = NoteRecorder(recorder: recorder, maxNote: Self.maxNote)
        self.transcriber = transcriber
        self.player = player
        self.requestMicrophone = requestMicrophone
    }

    isolated deinit {
        notes.cancel()
        playback?.cancel()
        review?.cancel()
        blocking?.cancel()
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

    /// Starts recording a voice note.
    func micTapped() {
        Task { await startRecording() }
    }

    func pauseTapped() {
        notes.pause()
    }

    func resumeTapped() {
        notes.resume()
    }

    /// Ends the recording without sending it, to listen to first.
    func stopTapped() {
        notes.stop()
        if let audio = notes.audio, let hint = Self.hint(for: audio) {
            notes.cancel()
            self.hint = hint
        }
    }

    /// Sends the voice note, whether it is recording, paused or stopped.
    func sendNoteTapped() {
        stopReview()
        guard let audio = notes.finish() else { return }
        if let hint = Self.hint(for: audio) {
            self.hint = hint
        } else {
            send(audio)
        }
    }

    /// Throws the voice note away.
    func cancelTapped() {
        stopReview()
        notes.cancel()
    }

    /// Plays the stopped note, or stops it playing.
    func reviewPlayTapped() {
        if reviewing {
            stopReview()
            return
        }
        guard let audio = notes.audio else { return }
        player.stop()
        review = Task { [self] in
            reviewing = true
            playing = nil
            try? await player.playRecording(audio)
            reviewing = false
        }
    }

    /// Sends what the user typed.
    func sendTextTapped() {
        let text = draft.trimmingCharacters(in: .whitespacesAndNewlines)
        guard !text.isEmpty, let lang = myLanguage, room?.other != nil else { return }
        // One ID for the message: its place in the list, and the key that keeps a resend from sending it twice.
        let id = UUID().uuidString
        draft = ""
        hint = nil
        pending.append(PendingNote(id: id, text: text, stage: .sending, duration: .zero, isText: true))
        sending[id] = Task { await uploadText(id, text: text, lang: lang) }
    }

    /// Plays a message, or stops it if it is playing. On the user's own, it plays what the listener hears.
    func playTapped(_ message: Message) {
        if playing == message.id {
            player.stop()
            return
        }
        guard let rendition = heard(message), rendition.status == .ready else { return }
        player.stop()
        review?.cancel()
        playback?.cancel()
        playback = Task { [self] in
            playing = message.id
            playingLength = nil
            problem = nil
            do {
                try await player.play(rendition) { length in playingLength = length }
            } catch is CancellationError {
            } catch {
                problem = "That message couldn't be played just now."
            }
            if playing == message.id { playing = nil }
            played.insert(message.id)
            fresh.remove(message.id)
        }
    }

    /// Plays a text message in the sender's voice, or stops it. The first time anyone asks, the server makes the voice
    /// first; it plays as soon as it is ready.
    func listenTapped(_ message: Message) {
        guard let rendition = heard(message) else { return }
        if playing == message.id || rendition.audible {
            playTapped(message)
        } else if waitingToHear.contains(message.id) {
            waitingToHear.remove(message.id)
        } else if rendition.status == .speaking {
            waitingToHear.insert(message.id)
        } else if rendition.status == .ready {
            waitingToHear.insert(message.id)
            problem = nil
            sending[message.id] = Task { [self] in
                do {
                    upsert(try await client.requestSpeech(messageID: message.id, lang: rendition.lang))
                } catch {
                    waitingToHear.remove(message.id)
                    problem = "That message couldn't be spoken just now."
                }
                sending[message.id] = nil
            }
        }
    }

    func originalToggled(_ message: Message) {
        if originals.contains(message.id) { originals.remove(message.id) } else { originals.insert(message.id) }
    }

    /// Tries again to deliver a message the server could not translate.
    func retryTapped(_ message: Message) {
        sending[message.id] = Task { [self] in
            do {
                upsert(try await client.retry(message.id))
            } catch {
                problem = "That message couldn't be sent again just now."
            }
        }
    }

    /// Sends again a message that never reached the server.
    func retryPending(_ note: PendingNote) {
        guard let lang = myLanguage else { return }
        sending[note.id] = Task {
            if note.isText {
                await uploadText(note.id, text: note.text, lang: lang)
            } else {
                await upload(note.id, text: note.text, duration: note.duration, lang: lang)
            }
        }
    }

    /// Blocks the other person, after the screen has asked. Neither can send here until the user unblocks them.
    func blockConfirmed() {
        guard let other = room?.other else { return }
        changeBlock { [client] in try await client.block(userID: other.id) }
    }

    func unblockTapped() {
        guard let other = room?.other else { return }
        changeBlock { [client] in try await client.unblock(userID: other.id) }
    }

    func reportTapped(_ message: Message) {
        guard !message.mine else { return }
        reporting = .message(message)
    }

    func reportPersonTapped() {
        guard let other = room?.other else { return }
        reporting = .person(other)
    }

    /// Sends the report the sheet was open for, and returns what went wrong, or nil once it is sent. A report that
    /// blocks too leaves the room blocked.
    func sendReport(reason: ReportReason, note: String, alsoBlock: Bool) async -> String? {
        guard let target = reporting else { return nil }
        let note = note.trimmingCharacters(in: .whitespacesAndNewlines)
        do {
            switch target {
            case .message(let message):
                try await client.report(messageID: message.id, reason: reason, note: note, block: alsoBlock)
            case .person(let person):
                try await client.report(userID: person.id, reason: reason, note: note, block: alsoBlock)
            }
        } catch {
            return (error as? BackendError)?.status == 0
                ? "No connection. Check you're online, then send it again."
                : "The report couldn't be sent just now. Please try again."
        }
        reporting = nil
        reportSent = true
        if alsoBlock { await load(firstTime: false) }
        return nil
    }

    private func changeBlock(_ change: @escaping @Sendable () async throws -> Void) {
        blocking?.cancel()
        blocking = Task { [self] in
            do {
                try await change()
                problem = nil
                await load(firstTime: false)
            } catch is CancellationError {
            } catch {
                problem = "That didn't work just now. Please try again."
            }
        }
    }

    func permissionChecked(granted: Bool) {
        if granted { micDenied = false }
    }

    private func startRecording() async {
        guard room?.other != nil, notes.phase == .idle else { return }
        guard await requestMicrophone() else {
            micDenied = true
            return
        }
        player.stop()
        do {
            try notes.start()
        } catch {
            problem = "The microphone couldn't be started."
            return
        }
        hint = nil
        problem = nil
    }

    private static func hint(for audio: PCMAudio) -> Hint? {
        if audio.duration < minNote { return .tooShort }
        if audio.isSilent { return .silent }
        return nil
    }

    private func stopReview() {
        if reviewing { player.stop() }
        review?.cancel()
    }

    /// What the user hears of a message: on their own, what the listener gets; on someone else's, their language.
    private func heard(_ message: Message) -> Rendition? {
        message.mine ? message.delivery : message.rendition(for: myLanguage)
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
            notSent(id, error)
        }
        sending[id] = nil
    }

    private func uploadText(_ id: String, text: String, lang: String) async {
        updatePending(id) { $0.stage = .sending }
        do {
            let message = try await client.sendText(roomID: roomID, text: text, lang: lang, idempotencyKey: id)
            pending.removeAll { $0.id == id }
            upsert(message)
        } catch {
            notSent(id, error)
        }
        sending[id] = nil
    }

    private func notSent(_ id: String, _ error: any Error) {
        updatePending(id) { $0.stage = .failed }
        if let refusal = Self.refusal((error as? BackendError)?.code) {
            problem = refusal
        }
    }

    private func load(firstTime: Bool) async {
        do {
            let room = try await client.room(roomID)
            let latest = try await client.messages(roomID: roomID)
            self.room = room
            if firstTime {
                // Opening the room: the unread notes are the newest of theirs. Only voice notes keep a "new" dot
                // until played; a text is read as it shows.
                fresh = Set(latest.filter { !$0.mine }.suffix(room.unread).filter { !$0.isText }.map(\.id))
            }
            messages = Self.merge(messages, latest)
            latest.forEach(playIfWaited)
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
            if !message.isText { fresh.insert(message.id) }
            try? await client.markRead(roomID: roomID)
        }
    }

    private func upsert(_ message: Message) {
        messages = Self.merge(messages, [message])
        playIfWaited(message)
    }

    /// Plays a text message the user asked to hear, once its voice is made.
    private func playIfWaited(_ message: Message) {
        guard waitingToHear.contains(message.id), heard(message)?.audible == true else { return }
        waitingToHear.remove(message.id)
        playTapped(message)
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

    /// What to tell the sender when the server refused their message, or nil when the failed card says enough.
    /// The daily limits count the last 24 hours, so more can be sent later, not at midnight.
    nonisolated static func refusal(_ code: String?) -> String? {
        switch code {
        case "room_closed": "This person has left, so the message can't reach them."
        case "voice_limit_reached": "You've sent as many voice notes as one day allows. You can send more later, or write a message."
        case "text_limit_reached": "You've sent as many messages as one day allows. You can send more later, or send a voice note."
        // The person blocked gets "cannot_send", and only the failed card: they are not told why.
        case "you_blocked": "You've blocked this person. Unblock them to send a message."
        default: nil
        }
    }

    /// Newer versions replace older ones; the list stays in the order the notes were sent.
    static func merge(_ current: [Message], _ incoming: [Message]) -> [Message] {
        var byID = Dictionary(current.map { ($0.id, $0) }, uniquingKeysWith: { $1 })
        for message in incoming { byID[message.id] = message }
        return byID.values.sorted { $0.createdAt < $1.createdAt }
    }
}
