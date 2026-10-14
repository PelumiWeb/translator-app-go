/// One event from a `text/event-stream` response.
struct ServerSentEvent: Equatable, Sendable {
    var id: String?
    var type: String?
    var data: String
}

/// Reads the server-sent events format a byte at a time. Each line is `field: value`, an empty line ends an
/// event, and a line starting with ":" is a comment, which the server sends as a keep-alive.
///
/// `URLSession.AsyncBytes.lines` cannot be used for this: it leaves out empty lines, and those are what end each
/// event.
struct ServerSentEventParser {
    private var line: [UInt8] = []
    private var id: String?
    private var type: String?
    private var data: [String] = []

    /// Takes the next byte of the stream, and returns an event when that byte completed one.
    mutating func consume(_ byte: UInt8) -> ServerSentEvent? {
        guard byte == UInt8(ascii: "\n") else {
            line.append(byte)
            return nil
        }
        if line.last == UInt8(ascii: "\r") {
            line.removeLast()
        }
        defer { line.removeAll(keepingCapacity: true) }
        return endOfLine(String(decoding: line, as: UTF8.self))
    }

    private mutating func endOfLine(_ text: String) -> ServerSentEvent? {
        if text.isEmpty {
            // The id is kept for later events, as the format says; the rest belongs to this event only.
            defer {
                type = nil
                data = []
            }
            guard !data.isEmpty else { return nil }
            return ServerSentEvent(id: id, type: type, data: data.joined(separator: "\n"))
        }
        if text.hasPrefix(":") {
            return nil
        }

        let (field, value) = Self.split(text)
        switch field {
        case "id": id = value
        case "event": type = value
        case "data": data.append(value)
        default: break // "retry", or a field added later
        }
        return nil
    }

    private static func split(_ text: String) -> (field: String, value: String) {
        guard let colon = text.firstIndex(of: ":") else { return (text, "") }
        var value = text[text.index(after: colon)...]
        if value.first == " " {
            value = value.dropFirst()
        }
        return (String(text[..<colon]), String(value))
    }
}
