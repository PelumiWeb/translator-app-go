import Foundation

/// A conversation, as the signed-in user sees it. For now it has two people.
struct Room: Decodable, Equatable, Identifiable, Sendable {
    let id: String
    /// Everyone in it but the user. Empty when the other person has deleted their account.
    var people: [Profile]
    var lastMessage: Message?
    /// Messages from the others the user has not seen.
    var unread: Int
    var createdAt: Date

    /// The other person, in a room of two.
    var other: Profile? { people.first }

    enum CodingKeys: String, CodingKey {
        case id, people, unread
        case lastMessage = "last_message"
        case createdAt = "created_at"
    }
}

/// A voice message as it was said, and what it has become for its listeners.
struct Message: Decodable, Equatable, Identifiable, Sendable {
    let id: String
    let roomID: String
    let senderID: String
    let mine: Bool
    /// The language it was said in.
    let lang: String
    let transcript: String
    let durationMS: Int
    let createdAt: Date
    /// The sender sees one per language the others speak; a listener sees their own only.
    var renditions: [Rendition]

    /// What the message became for `language`, the user's: theirs to hear.
    func rendition(for language: String?) -> Rendition? {
        renditions.first { $0.lang == language }
    }

    /// The rendition of the user's own message that the listener gets; in a room of two, there is one.
    var delivery: Rendition? { renditions.first }

    var duration: Duration { .milliseconds(durationMS) }

    enum CodingKeys: String, CodingKey {
        case id, mine, lang, transcript, renditions
        case roomID = "room_id"
        case senderID = "sender_id"
        case durationMS = "duration_ms"
        case createdAt = "created_at"
    }
}

/// A message for the listeners of one language: translated, then spoken in the sender's voice.
struct Rendition: Decodable, Equatable, Sendable {
    enum Status: String, Decodable, Sendable {
        case translating, speaking, ready, failed

        /// A status from a newer server reads as still on its way.
        init(from decoder: any Decoder) throws {
            self = Status(rawValue: try decoder.singleValueContainer().decode(String.self)) ?? .translating
        }
    }

    enum Voice: String, Decodable, Sendable {
        /// The sender's own voice, made for this language.
        case own
        /// The sender's recording, for listeners of the language it was said in.
        case original
        /// The sender keeps no voice: the listener's phone reads the text.
        case phone
        /// Not ready yet, or a kind from a newer server.
        case none

        init(from decoder: any Decoder) throws {
            self = Voice(rawValue: try decoder.singleValueContainer().decode(String.self)) ?? .none
        }
    }

    let lang: String
    var status: Status
    /// The translation, once made.
    var text: String
    var voice: Voice
    /// Where to fetch the audio, when there is audio to fetch.
    var audioURL: String?

    enum CodingKeys: String, CodingKey {
        case lang, status, text, voice
        case audioURL = "audio_url"
    }

    init(lang: String, status: Status, text: String = "", voice: Voice = .none, audioURL: String? = nil) {
        self.lang = lang
        self.status = status
        self.text = text
        self.voice = voice
        self.audioURL = audioURL
    }

    init(from decoder: any Decoder) throws {
        let container = try decoder.container(keyedBy: CodingKeys.self)
        lang = try container.decode(String.self, forKey: .lang)
        status = try container.decodeIfPresent(Status.self, forKey: .status) ?? .translating
        text = try container.decodeIfPresent(String.self, forKey: .text) ?? ""
        voice = try container.decodeIfPresent(Voice.self, forKey: .voice) ?? .none
        audioURL = try container.decodeIfPresent(String.self, forKey: .audioURL)
    }
}

/// What the server's stream of room changes says.
enum RoomChange: Equatable, Sendable {
    /// The stream is open: anything fetched from now on is at least as new as what it will report.
    case ready
    /// Something in a room changed: a new message, or `messageID` moved on.
    case changed(roomID: String, messageID: String?)
}
