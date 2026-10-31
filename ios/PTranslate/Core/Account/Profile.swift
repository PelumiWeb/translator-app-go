import CryptoKit
import Foundation

/// A person with an account: the signed-in user, or someone they talk to.
struct Profile: Codable, Equatable, Sendable {
    let id: String
    var displayName: String = ""
    /// The ID others find them by, without the "@"; empty until chosen.
    var handle: String = ""
    /// The language they speak, as a tag such as "es"; empty until chosen.
    var language: String = ""

    /// Signed up far enough to talk to people: an ID to be found by and a language to hear in.
    var isComplete: Bool { !handle.isEmpty && !language.isEmpty }

    var lang: Language? { language.isEmpty ? nil : Language(language) }

    enum CodingKeys: String, CodingKey {
        case id, handle, language
        case displayName = "display_name"
    }

    init(id: String, displayName: String = "", handle: String = "", language: String = "") {
        self.id = id
        self.displayName = displayName
        self.handle = handle
        self.language = language
    }

    init(from decoder: any Decoder) throws {
        let container = try decoder.container(keyedBy: CodingKeys.self)
        id = try container.decode(String.self, forKey: .id)
        displayName = try container.decodeIfPresent(String.self, forKey: .displayName) ?? ""
        handle = try container.decodeIfPresent(String.self, forKey: .handle) ?? ""
        language = try container.decodeIfPresent(String.self, forKey: .language) ?? ""
    }
}

/// The signed-in user. `token` is the session's secret, sent with every request.
struct Session: Equatable, Sendable {
    let token: String
    var profile: Profile
}

/// Keeps the session across launches.
protocol SessionStore: Sendable {
    func load() -> Session?
    func save(_ session: Session)
    func clear()
}

/// What the server said about an ID the user wants.
struct HandleCheck: Decodable, Equatable, Sendable {
    let handle: String
    let available: Bool
    /// Free IDs like it, when it is taken.
    var suggestions: [String] = []

    enum CodingKeys: String, CodingKey { case handle, available, suggestions }

    init(handle: String, available: Bool, suggestions: [String] = []) {
        self.handle = handle
        self.available = available
        self.suggestions = suggestions
    }

    init(from decoder: any Decoder) throws {
        let container = try decoder.container(keyedBy: CodingKeys.self)
        handle = try container.decode(String.self, forKey: .handle)
        available = try container.decode(Bool.self, forKey: .available)
        suggestions = try container.decodeIfPresent([String].self, forKey: .suggestions) ?? []
    }
}

/// The voice the user keeps on the server, and the consent they gave for it.
struct KeptVoice: Decodable, Equatable, Sendable {
    let consentVersion: String
    let consentedAt: Date

    enum CodingKeys: String, CodingKey {
        case consentVersion = "consent_version"
        case consentedAt = "consented_at"
    }
}

/// The rules an ID must follow, checked on the phone before asking the server.
enum Handles {
    static let min = 3
    static let max = 20

    /// What breaks the rules, in words for the person, or nil if `handle` follows them.
    static func problem(with handle: String) -> String? {
        let allowed = Set("abcdefghijklmnopqrstuvwxyz0123456789._")
        if handle.contains(where: { !allowed.contains($0) }) {
            return "Only letters, numbers, dots and underscores."
        }
        if handle.count < min { return "At least \(min) characters." }
        if handle.count > max { return "At most \(max) characters." }
        return nil
    }

    /// What the person typed, as an ID: no "@", no spaces, lower case.
    static func normalise(_ input: String) -> String {
        var text = input.trimmingCharacters(in: .whitespaces)
        if text.hasPrefix("@") { text.removeFirst() }
        return text.lowercased()
    }
}

/// A nonce for one sign-in. Apple is given `hashed`, and writes it into the token it issues; the server is given
/// `raw`, hashes it, and checks the two match (ADR 0018). A token caught on the way is useless without the raw nonce.
struct Nonce: Sendable {
    let raw: String

    /// SHA-256 of `raw`, as lower-case hex.
    var hashed: String {
        SHA256.hash(data: Data(raw.utf8)).map { String(format: "%02x", $0) }.joined()
    }

    static func random() -> Nonce {
        var generator = SystemRandomNumberGenerator()
        let bytes = (0..<32).map { _ in UInt8.random(in: .min ... .max, using: &generator) }
        let text = Data(bytes).base64EncodedString()
            .replacingOccurrences(of: "+", with: "-")
            .replacingOccurrences(of: "/", with: "_")
            .replacingOccurrences(of: "=", with: "")
        return Nonce(raw: text)
    }
}
