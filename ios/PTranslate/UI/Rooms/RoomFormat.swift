import Foundation

/// The link in a person's QR code and in what they share: opening it offers to add them.
func addLink(_ handle: String) -> URL {
    URL(string: "timbre://add/\(handle)")!
}

/// The ID in a scanned code or opened link: timbre://add/<id>. Anything else is not ours.
func handle(fromCode raw: String) -> String? {
    let prefix = "timbre://add/"
    guard raw.lowercased().hasPrefix(prefix) else { return nil }
    let id = raw.dropFirst(prefix.count).trimmingCharacters(in: CharacterSet(charactersIn: "/")).lowercased()
    return id.isEmpty ? nil : id
}

/// When something was sent, as a list shows it: the time today, "Yesterday", the weekday this week, else the date.
func roomTime(_ date: Date, now: Date = .now, calendar: Calendar = .current) -> String {
    let days = calendar.dateComponents([.day], from: calendar.startOfDay(for: date), to: calendar.startOfDay(for: now)).day ?? 0
    switch days {
    case 0: return date.formatted(date: .omitted, time: .shortened)
    case 1: return "Yesterday"
    case 2...6: return date.formatted(.dateTime.weekday(.abbreviated))
    default: return date.formatted(.dateTime.day().month(.abbreviated))
    }
}

extension Message {
    /// A room's last message, as one line of the list.
    func preview(myLanguage: String?) -> String {
        if mine { return "You: \(transcript)" }
        if let rendition = rendition(for: myLanguage), rendition.status == .ready, !rendition.text.isEmpty {
            return rendition.text
        }
        return "Voice note · \(clock(duration))"
    }
}

extension Profile {
    /// How a person is named on screen.
    var shownName: String {
        if !displayName.isEmpty { return displayName }
        return handle.isEmpty ? "Someone" : "@\(handle)"
    }
}
