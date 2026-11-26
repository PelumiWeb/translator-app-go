import Foundation

/// The start of the link in a person's QR code and in what they share. The scheme is in Info.plist too.
private let addPrefix = "shinar://add/"

/// The link in a person's QR code and in what they share: opening it offers to add them.
func addLink(_ handle: String) -> URL {
    URL(string: addPrefix + handle)!
}

/// What goes with the link when someone shares their ID.
func shareMessage(_ handle: String) -> String {
    String(localized: "Talk to me on \(Brand.name), in your own language: my ID is @\(handle)")
}

/// The ID in a scanned code or opened link: shinar://add/<id>. Anything else is not ours.
func handle(fromCode raw: String) -> String? {
    let prefix = addPrefix
    guard raw.lowercased().hasPrefix(prefix) else { return nil }
    let id = raw.dropFirst(prefix.count).trimmingCharacters(in: CharacterSet(charactersIn: "/")).lowercased()
    return id.isEmpty ? nil : id
}

/// When something was sent, as a list shows it: the time today, "Yesterday", the weekday this week, else the date.
func roomTime(_ date: Date, now: Date = .now, calendar: Calendar = .current) -> String {
    let days = calendar.dateComponents([.day], from: calendar.startOfDay(for: date), to: calendar.startOfDay(for: now)).day ?? 0
    switch days {
    case 0: return date.formatted(date: .omitted, time: .shortened)
    case 1: return String(localized: "Yesterday")
    case 2...6: return date.formatted(.dateTime.weekday(.abbreviated))
    default: return date.formatted(.dateTime.day().month(.abbreviated))
    }
}

/// The heading over a day's messages in a room: "Today", "Yesterday", the weekday this week, else the date, with the
/// year when it is not this one.
func dayHeading(_ date: Date, now: Date = .now, calendar: Calendar = .current, locale: Locale = .current) -> String {
    let days = calendar.dateComponents([.day], from: calendar.startOfDay(for: date), to: calendar.startOfDay(for: now)).day ?? 0
    var style = Date.FormatStyle(locale: locale, calendar: calendar, timeZone: calendar.timeZone)
    switch days {
    case 0: return String(localized: "Today")
    case 1: return String(localized: "Yesterday")
    case 2...6: style = style.weekday(.wide)
    default:
        style = style.day().month(.wide)
        if !calendar.isDate(date, equalTo: now, toGranularity: .year) { style = style.year() }
    }
    return date.formatted(style)
}

/// Whether the message at `index` is the first of its day, and so goes under a new heading.
func startsDay(_ messages: [Message], at index: Int, calendar: Calendar = .current) -> Bool {
    index == 0 || !calendar.isDate(messages[index].createdAt, inSameDayAs: messages[index - 1].createdAt)
}

extension Message {
    /// The words the user reads for someone else's message, once there are words to read: a voice note's when it can
    /// be heard, a text message's as soon as it is translated (its voice comes later, if asked for).
    func words(for language: String?) -> String? {
        guard let rendition = rendition(for: language), !rendition.text.isEmpty else { return nil }
        let readable = rendition.status == .ready || (isText && rendition.status == .speaking)
        return readable ? rendition.text : nil
    }

    /// A room's last message, as one line of the list.
    func preview(myLanguage: String?) -> String {
        if mine { return String(localized: "You: \(transcript)") }
        return words(for: myLanguage) ?? (isText ? String(localized: "Message") : String(localized: "Voice note · \(clock(duration))"))
    }
}

extension Profile {
    /// How a person is named on screen.
    var shownName: String {
        if !displayName.isEmpty { return displayName }
        return handle.isEmpty ? String(localized: "Someone") : "@\(handle)"
    }
}
