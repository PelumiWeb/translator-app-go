import Foundation

/// The backend's rooms: who the user talks with, and their voice messages.
final class RoomsClient: Sendable {
    private let rest: Rest

    init(rest: Rest) {
        self.rest = rest
    }

    func rooms() async throws -> [Room] {
        try await rest.get("/v1/rooms", as: RoomList.self).rooms
    }

    /// Opens a room with the person whose ID is `handle`, or returns the one the two already have. Throws with code
    /// "user_not_found" or "cannot_add_self".
    func openRoom(handle: String) async throws -> Room {
        try await rest.send("POST", "/v1/rooms", json: ["handle": handle])
    }

    func room(_ id: String) async throws -> Room {
        try await rest.get("/v1/rooms/\(id)")
    }

    /// The newest messages, oldest first, or with `before`, the ones sent before that message.
    func messages(roomID: String, before: String? = nil, limit: Int = 50) async throws -> [Message] {
        var path = "/v1/rooms/\(roomID)/messages?limit=\(limit)"
        if let before { path += "&before=\(before)" }
        return try await rest.get(path, as: MessageList.self).messages
    }

    /// Sends a voice message. `idempotencyKey` stays the same across attempts at the same message, so a resend after
    /// a lost response does not send it twice.
    func sendMessage(roomID: String, wav: Data, transcript: String, durationMS: Int, lang: String, idempotencyKey: String) async throws -> Message {
        var form = MultipartForm()
        form.addField("transcript", value: transcript)
        form.addField("duration_ms", value: String(durationMS))
        form.addField("lang", value: lang)
        form.addFile("audio", filename: "message.wav", contentType: "audio/wav", data: wav)
        var request = await rest.request("/v1/rooms/\(roomID)/messages", method: "POST")
        request.setValue(form.contentType, forHTTPHeaderField: "Content-Type")
        request.setValue(idempotencyKey, forHTTPHeaderField: "Idempotency-Key")
        request.httpBody = form.body()
        return try rest.decode(try await rest.send(request))
    }

    func markRead(roomID: String) async throws {
        try await rest.send(await rest.request("/v1/rooms/\(roomID)/read", method: "POST"))
    }

    func message(_ id: String) async throws -> Message {
        try await rest.get("/v1/messages/\(id)")
    }

    /// Tries again to deliver a message that could not be translated.
    func retry(_ id: String) async throws -> Message {
        try rest.decode(try await rest.send(await rest.request("/v1/messages/\(id)/retry", method: "POST")))
    }

    /// The audio at a rendition's audio URL, as a WAV file.
    func audio(_ url: String) async throws -> Data {
        try await rest.send(await rest.request(url))
    }

    /// Follows changes to the user's rooms until the server closes the stream or it fails. Nothing is replayed: on
    /// `.ready`, fetch what is on screen.
    func changes() async throws -> RoomChanges {
        var request = await rest.request("/v1/me/events")
        request.setValue("text/event-stream", forHTTPHeaderField: "Accept")
        // The stream is silent between changes, apart from the server's keep-alive every 15 seconds.
        request.timeoutInterval = 60
        let bytes: URLSession.AsyncBytes
        let response: URLResponse
        do {
            (bytes, response) = try await rest.session.bytes(for: request)
        } catch {
            throw BackendClient.connectionError(error, "Could not reach the server")
        }
        let status = (response as? HTTPURLResponse)?.statusCode ?? 0
        guard (200..<300).contains(status) else {
            throw BackendError(message: "The server answered \(status)", retryable: status >= 500, status: status)
        }
        return RoomChanges(bytes: bytes)
    }
}

/// The changes to the user's rooms, in the order the server sent them. A failure while reading is a `BackendError`.
struct RoomChanges: AsyncSequence {
    typealias Element = RoomChange

    fileprivate let bytes: URLSession.AsyncBytes

    func makeAsyncIterator() -> AsyncIterator {
        AsyncIterator(bytes: bytes.makeAsyncIterator())
    }

    struct AsyncIterator: AsyncIteratorProtocol {
        fileprivate var bytes: URLSession.AsyncBytes.AsyncIterator
        private var parser = ServerSentEventParser()

        fileprivate init(bytes: URLSession.AsyncBytes.AsyncIterator) {
            self.bytes = bytes
        }

        mutating func next() async throws -> RoomChange? {
            while true {
                let byte: UInt8?
                do {
                    byte = try await bytes.next()
                } catch {
                    throw BackendClient.connectionError(error, "The room stream broke")
                }
                guard let byte else { return nil }
                guard let event = parser.consume(byte) else { continue }
                if let change = RoomChange(type: event.type, data: event.data) { return change }
            }
        }
    }
}

extension RoomChange {
    /// Reads one event of the stream; nil for a type this app does not know.
    init?(type: String?, data: String) {
        switch type {
        case "ready":
            self = .ready
        case "room":
            guard let payload = try? JSONDecoder().decode(ChangePayload.self, from: Data(data.utf8)) else { return nil }
            self = .changed(roomID: payload.roomID, messageID: payload.messageID)
        default:
            return nil
        }
    }
}

private struct ChangePayload: Decodable {
    let roomID: String
    let messageID: String?

    enum CodingKeys: String, CodingKey {
        case roomID = "room_id"
        case messageID = "message_id"
    }
}

private struct RoomList: Decodable {
    let rooms: [Room]
}

private struct MessageList: Decodable {
    let messages: [Message]
}
