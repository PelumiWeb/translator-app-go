import SwiftUI

// The voice note cards, in every state of the design's board: the user's in warm white whatever happens to them, the
// other person's in raised petrol with mint.

/// Dots for a note whose own waveform is not known: steady, varied, the same each time for the same note.
func dots(for id: String) -> [Float] {
    let seed = id.unicodeScalars.reduce(0) { ($0 &* 31 &+ Int($1.value)) % 9973 }
    return (0..<22).map { Float(0.25 + 0.75 * abs(sin(Double(seed % 97) + Double($0) * 1.7))) }
}

/// The user's note, from the moment the server has it.
struct MyNoteCard: View {
    let message: Message
    let playing: Bool
    let playingLength: Duration?
    let onPlay: () -> Void
    let onRetry: () -> Void
    let onRecordVoice: () -> Void

    var body: some View {
        let delivery = message.delivery
        let listener = delivery.map { Language($0.lang).localName } ?? ""
        VStack(alignment: .leading, spacing: 12) {
            if let delivery, delivery.status == .ready, delivery.voice != .phone {
                PlayRow(
                    playing: playing, length: playing ? playingLength ?? message.duration : message.duration,
                    dots: dots(for: message.id), fill: Palette.onYou, icon: Palette.you, dotColor: Palette.onYou,
                    durationColor: Palette.onYou, label: "Play what they hear", onPlay: onPlay
                )
            }
            Text(message.transcript).font(.pearlMessage).foregroundStyle(Palette.onYou)
            switch (delivery?.status, delivery?.voice) {
            case (nil, _):
                status("Sent")
            case (.translating?, _):
                StepsBar(done: 1)
                status("Translating into \(listener)")
            case (.speaking?, _):
                HStack(spacing: 10) {
                    BreathingRing()
                    status("Preparing your voice in \(listener)")
                }
            case (.failed?, _):
                Divider().overlay(Palette.onYou.opacity(0.15))
                HStack(spacing: 10) {
                    Image(systemName: "exclamationmark.circle")
                    Text("Couldn't translate this. Your words are kept.").font(.pearlSmall)
                    Spacer(minLength: 4)
                    Button("Retry", action: onRetry).font(.pearlSmall.weight(.medium)).underline()
                }
                .foregroundStyle(Palette.onYou)
            case (.ready?, .phone?):
                HStack(spacing: 6) {
                    Image(systemName: "iphone")
                    Text("Delivered in the phone's voice ·")
                    Button("Record yours", action: onRecordVoice).underline()
                }
                .font(.pearlMeta).foregroundStyle(Palette.onYou.opacity(0.7))
            case (.ready?, let voice):
                Label {
                    Text((voice == .original ? "Delivered" : "Delivered in your voice") + " · " + message.createdAt.formatted(date: .omitted, time: .shortened))
                } icon: {
                    Image(systemName: "checkmark")
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

/// The user's note before the server has it: being written down, sending, or not sent.
struct PendingNoteCard: View {
    let note: RoomViewModel.PendingNote
    let onRetry: () -> Void

    var body: some View {
        VStack(alignment: .leading, spacing: 12) {
            switch note.stage {
            case .writing:
                // Words arrive one by one; the last few are still being decided, so they are fainter.
                let words = note.text.split(separator: " ").map(String.init)
                let settled = max(words.count - 2, 0)
                (Text(words.prefix(settled).joined(separator: " ") + (settled > 0 ? " " : ""))
                    + Text(words.dropFirst(settled).joined(separator: " ")).foregroundStyle(Palette.onYou.opacity(0.44))
                    + Text("▍"))
                    .font(.pearlMessage).foregroundStyle(Palette.onYou)
                status("Writing down what you said…")
            case .sending:
                Text(note.text).font(.pearlMessage).foregroundStyle(Palette.onYou)
                StepsBar(done: 0)
                status("Sending")
            case .failed:
                Text(note.text).font(.pearlMessage).foregroundStyle(Palette.onYou)
                Divider().overlay(Palette.onYou.opacity(0.15))
                HStack(spacing: 10) {
                    Image(systemName: "exclamationmark.circle")
                    Text("Couldn't send this. Your words are kept.").font(.pearlSmall)
                    Spacer(minLength: 4)
                    Button("Retry", action: onRetry).font(.pearlSmall.weight(.medium)).underline()
                }
                .foregroundStyle(Palette.onYou)
                .buttonStyle(.plain)
            }
        }
        .padding(16)
        .frame(maxWidth: 270, alignment: .leading)
        .background(Palette.you, in: RoundedRectangle(cornerRadius: 12))
    }

    private func status(_ text: String) -> some View {
        Text(text).font(.custom("IBMPlexSans-Medm", size: 13, relativeTo: .footnote)).foregroundStyle(Palette.onYou)
    }
}

/// The other person's note, as the user hears it: in their language, in the sender's voice.
struct TheirNoteCard: View {
    let message: Message
    let myLanguage: String?
    let playing: Bool
    let playingLength: Duration?
    let fresh: Bool
    let played: Bool
    let showOriginal: Bool
    let onPlay: () -> Void
    let onToggleOriginal: () -> Void

    var body: some View {
        let rendition = message.rendition(for: myLanguage)
        let ready = rendition?.status == .ready
        let phoneVoice = ready && rendition?.voice == .phone
        let text = ready ? rendition?.text.nilIfEmpty : nil
        let original = Language(message.lang).ownName
        VStack(alignment: .leading, spacing: 12) {
            if ready {
                HStack(spacing: 10) {
                    PlayRow(
                        playing: playing, length: playing ? playingLength ?? message.duration : message.duration,
                        dots: dots(for: message.id),
                        fill: phoneVoice ? .clear : Palette.them, icon: phoneVoice ? Palette.them : Palette.onThem,
                        dotColor: phoneVoice ? Palette.fg.opacity(0.4) : Palette.them.opacity(fresh && !playing ? 0.45 : 1),
                        durationColor: Palette.fg2, outline: phoneVoice, showsLength: !fresh, label: "Play", onPlay: onPlay
                    )
                    if fresh && !playing {
                        Circle().fill(Palette.them).frame(width: 10, height: 10).accessibilityLabel("New")
                    }
                }
            }
            if let text {
                SpokenText(text: text, playing: playing, length: playingLength)
            } else if rendition == nil || rendition?.status == .failed {
                Text(message.transcript).font(.pearlMessage).foregroundStyle(Palette.fg)
            } else {
                Text(rendition?.status == .speaking ? "Preparing their voice…" : "Translating from \(original)…")
                    .font(.pearlMessage).foregroundStyle(Palette.fg2)
            }
            if showOriginal, text != nil {
                Divider().overlay(Palette.fg.opacity(0.08))
                Text("Original · \(original)").font(.pearlLabel).foregroundStyle(Palette.them)
                Text(message.transcript).font(.custom("IBMPlexSans", size: 15, relativeTo: .body)).foregroundStyle(Palette.fg.opacity(0.72))
            }
            meta(rendition: rendition, ready: ready, phoneVoice: phoneVoice, original: original)
        }
        .padding(16)
        .frame(maxWidth: 290, alignment: .leading)
        .background(Palette.bgRaised, in: RoundedRectangle(cornerRadius: 12))
    }

    /// The meta line: what state the note is in, and the original a tap away.
    @ViewBuilder
    private func meta(rendition: Rendition?, ready: Bool, phoneVoice: Bool, original: String) -> some View {
        if rendition?.status == .failed {
            Text("Couldn't be translated · Original · \(original)").font(.pearlMeta).foregroundStyle(Palette.fg2)
        } else if ready {
            if phoneVoice {
                Label("Spoken by your phone's voice", systemImage: "iphone").font(.pearlMeta).foregroundStyle(Palette.fg2)
            } else if playing {
                Text("Playing in their voice").font(.pearlMeta).foregroundStyle(Palette.them)
            } else {
                HStack(spacing: 0) {
                    Text(fresh ? "New · \(clock(message.duration)) · in their voice · " : played ? "Played · " : "In their voice · ")
                        .foregroundStyle(Palette.fg2)
                    if rendition?.voice != .original {
                        Button(showOriginal ? "Hide original" : "Show original", action: onToggleOriginal)
                            .underline().foregroundStyle(Palette.them)
                    }
                }
                .font(.pearlMeta)
                .buttonStyle(.plain)
            }
        }
    }
}

/// The translation, lighting up word by word as it plays.
private struct SpokenText: View {
    let text: String
    let playing: Bool
    let length: Duration?

    @State private var started: Date?

    var body: some View {
        TimelineView(.animation(paused: !playing || length == nil)) { context in
            let words = text.split(separator: " ").map(String.init)
            let total = length.map { max($0 / .seconds(1), 0.1) } ?? 1
            let progress = playing ? started.map { min(context.date.timeIntervalSince($0) / total, 1) } ?? 0 : 1
            let lit = max(Int(Double(words.count) * progress), playing ? 1 : words.count)
            (Text(words.prefix(lit).joined(separator: " "))
                + Text(lit < words.count ? " " + words.dropFirst(lit).joined(separator: " ") : "").foregroundStyle(Palette.fg.opacity(0.45)))
                .font(.pearlMessage).foregroundStyle(Palette.fg)
        }
        .onChange(of: length) { _, length in started = length == nil ? nil : .now }
        .onChange(of: playing) { _, playing in if !playing { started = nil } }
    }
}

private struct PlayRow: View {
    let playing: Bool
    let length: Duration
    let dots: [Float]
    let fill: Color
    let icon: Color
    let dotColor: Color
    let durationColor: Color
    var outline = false
    var showsLength = true
    let label: String
    let onPlay: () -> Void

    @State private var started: Date?

    var body: some View {
        HStack(spacing: 12) {
            Button(action: onPlay) {
                Image(systemName: playing ? "pause.fill" : "play.fill")
                    .font(.system(size: 15))
                    .foregroundStyle(icon)
                    .frame(width: 44, height: 44)
                    .background(fill, in: Circle())
                    .overlay { if outline { Circle().stroke(Palette.them, lineWidth: 1.5) } }
            }
            .buttonStyle(.plain)
            .accessibilityLabel(playing ? "Stop" : label)
            TimelineView(.animation(paused: !playing)) { context in
                let total = max(length / .seconds(1), 0.1)
                let progress = playing ? started.map { min(context.date.timeIntervalSince($0) / total, 1) } ?? 0 : 1
                DotWaveform(levels: dots, color: dotColor, played: progress)
            }
            if showsLength {
                Text(clock(length)).font(.pearlMeta).monospacedDigit().foregroundStyle(durationColor)
            }
        }
        .onChange(of: playing, initial: true) { _, playing in started = playing ? .now : nil }
    }
}

/// The design's progress bar of a note on its way: sent, translating, voice, delivered.
private struct StepsBar: View {
    let done: Int

    var body: some View {
        HStack(spacing: 6) {
            ForEach(0..<4, id: \.self) { index in
                Capsule().fill(Palette.onYou.opacity(index <= done ? 1 : 0.18)).frame(height: 3)
            }
        }
        .accessibilityHidden(true)
    }
}

/// A dot in a ring, breathing slowly while the voice is made, so the card never looks frozen.
private struct BreathingRing: View {
    @Environment(\.accessibilityReduceMotion) private var reduceMotion
    @State private var breathing = false

    var body: some View {
        Circle()
            .stroke(Palette.onYou.opacity(0.5), lineWidth: 1.5)
            .frame(width: 28, height: 28)
            .overlay {
                Circle().fill(Palette.onYou).frame(width: 14, height: 14)
                    .scaleEffect(breathing ? 1.1 : 0.85)
            }
            .onAppear {
                guard !reduceMotion else { return }
                withAnimation(.easeInOut(duration: 1.6).repeatForever(autoreverses: true)) { breathing = true }
            }
            .accessibilityHidden(true)
    }
}

private extension String {
    var nilIfEmpty: String? { isEmpty ? nil : self }
}
