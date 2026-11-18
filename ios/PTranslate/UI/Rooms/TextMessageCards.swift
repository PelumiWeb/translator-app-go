import SwiftUI

// Text messages, in the voice notes' colours: the user's in warm white, the other person's in raised petrol with mint.
// The words come first; the voice is a tap away, made the first time anyone asks for it.

/// Where hearing a text message stands, for the listen control.
private enum Listening {
    case idle, preparing, playing

    init(_ rendition: Rendition?, playing: Bool, waiting: Bool) {
        if playing {
            self = .playing
        } else if waiting || rendition?.status == .speaking {
            self = .preparing
        } else {
            self = .idle
        }
    }
}

/// The user's text message, from the moment the server has it.
struct MyTextCard: View {
    let message: Message
    let playing: Bool
    let waiting: Bool
    let onListen: () -> Void
    let onRetry: () -> Void

    var body: some View {
        let delivery = message.delivery
        let listener = delivery.map { Language($0.lang).localName } ?? ""
        VStack(alignment: .leading, spacing: 12) {
            Text(message.transcript).font(.pearlMessage).foregroundStyle(Palette.onYou)
            switch delivery?.status {
            case nil:
                status("Sent")
            case .translating?:
                status("Translating into \(listener)")
            case .failed?:
                Divider().overlay(Palette.onYou.opacity(0.15))
                HStack(spacing: 10) {
                    Image(systemName: "exclamationmark.circle")
                    Text("Couldn't translate this. Your words are kept.").font(.pearlSmall)
                    Spacer(minLength: 4)
                    Button("Retry", action: onRetry).font(.pearlSmall.weight(.medium)).underline()
                }
                .foregroundStyle(Palette.onYou)
            default:
                HStack(spacing: 6) {
                    Image(systemName: "checkmark")
                    Text("Delivered · \(message.createdAt.formatted(date: .omitted, time: .shortened)) ·")
                    switch Listening(delivery, playing: playing, waiting: waiting) {
                    case .playing: Button("Stop", action: onListen).underline()
                    case .preparing: Text("Preparing your voice…")
                    case .idle: Button("Hear it in \(listener)", action: onListen).underline()
                    }
                }
                .font(.pearlMeta).foregroundStyle(Palette.onYou.opacity(0.7))
            }
        }
        .buttonStyle(.plain)
        .padding(16)
        .frame(maxWidth: 270, alignment: .leading)
        .background(Palette.you, in: RoundedRectangle(cornerRadius: 12))
    }

    private func status(_ text: String) -> some View {
        Text(text).font(.custom("IBMPlexSans-Medm", size: 13, relativeTo: .footnote)).foregroundStyle(Palette.onYou)
    }
}

/// The other person's text message, as the user reads it: in their language, with the sender's voice a tap away.
struct TheirTextCard: View {
    let message: Message
    let myLanguage: String?
    let playing: Bool
    let playingLength: Duration?
    let waiting: Bool
    let showOriginal: Bool
    let onListen: () -> Void
    let onToggleOriginal: () -> Void

    var body: some View {
        let rendition = message.rendition(for: myLanguage)
        let words = message.words(for: myLanguage)
        let original = Language(message.lang).ownName
        let translated = rendition.map { $0.lang != message.lang } ?? false
        VStack(alignment: .leading, spacing: 12) {
            if let words {
                SpokenText(text: words, playing: playing, length: playingLength)
            } else if rendition == nil || rendition?.status == .failed {
                Text(message.transcript).font(.pearlMessage).foregroundStyle(Palette.fg)
            } else {
                Text("Translating from \(original)…").font(.pearlMessage).foregroundStyle(Palette.fg2)
            }
            if showOriginal, words != nil, translated {
                Divider().overlay(Palette.fg.opacity(0.08))
                Text("Original · \(original)").font(.pearlLabel).foregroundStyle(Palette.them)
                Text(message.transcript).font(.custom("IBMPlexSans", size: 15, relativeTo: .body)).foregroundStyle(Palette.fg.opacity(0.72))
            }
            if rendition?.status == .failed {
                Text("Couldn't be translated · Original · \(original)").font(.pearlMeta).foregroundStyle(Palette.fg2)
            } else if words != nil {
                let state = Listening(rendition, playing: playing, waiting: waiting)
                let phoneVoice = rendition?.voice == .phone
                HStack(spacing: 10) {
                    ListenButton(state: state, action: onListen)
                    Text(label(state, phoneVoice: phoneVoice))
                        .foregroundStyle(state == .playing ? Palette.them : Palette.fg2)
                    Spacer(minLength: 0)
                    if translated {
                        Button(showOriginal ? "Hide original" : "Show original", action: onToggleOriginal)
                            .underline().foregroundStyle(Palette.them)
                    }
                }
                .font(.pearlMeta)
                .buttonStyle(.plain)
            }
        }
        .padding(16)
        .frame(maxWidth: 290, alignment: .leading)
        .background(Palette.bgRaised, in: RoundedRectangle(cornerRadius: 12))
    }

    private func label(_ state: Listening, phoneVoice: Bool) -> String {
        switch state {
        case .playing: phoneVoice ? "Spoken by your phone's voice" : "Playing in their voice"
        case .preparing: "Preparing their voice…"
        case .idle: phoneVoice ? "Listen · your phone's voice" : "Listen in their voice"
        }
    }
}

/// The mint outline button that plays a text message in its sender's voice, breathing while the voice is made.
private struct ListenButton: View {
    let state: Listening
    let action: () -> Void

    var body: some View {
        Button(action: action) {
            Group {
                switch state {
                case .playing: Image(systemName: "pause.fill").font(.system(size: 13)).foregroundStyle(Palette.onThem)
                case .preparing: BreathingRing(color: Palette.them)
                case .idle: Image(systemName: "speaker.wave.2").font(.system(size: 15)).foregroundStyle(Palette.them)
                }
            }
            .frame(width: 36, height: 36)
            .background(state == .playing ? Palette.them : .clear, in: Circle())
            .overlay(Circle().stroke(Palette.them, lineWidth: 1.5))
        }
        .buttonStyle(.plain)
        .accessibilityLabel(state == .playing ? "Stop" : state == .preparing ? "Preparing their voice" : "Listen")
    }
}
