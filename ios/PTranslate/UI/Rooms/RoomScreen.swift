import AVFoundation
import SwiftUI

/// A conversation: the messages, newest at the bottom, and the composer: type, or record a voice note.
struct RoomScreen: View {
    @Bindable var viewModel: RoomViewModel
    let onRecordVoice: () -> Void

    @Environment(\.openURL) private var openURL
    @Environment(\.scenePhase) private var scenePhase
    @State private var confirmingBlock = false

    var body: some View {
        let other = viewModel.room?.other
        VStack(spacing: 0) {
            ScrollViewReader { scroller in
                ScrollView {
                    LazyVStack(spacing: 14) {
                        ForEach(viewModel.messages) { message in
                            HStack {
                                if message.mine {
                                    Spacer(minLength: 40)
                                    card(message)
                                } else {
                                    // Press and hold one of theirs to report it.
                                    card(message).contextMenu {
                                        Button { viewModel.reportTapped(message) } label: {
                                            Label(message.isText ? "Report this message" : "Report this voice note", systemImage: "flag")
                                        }
                                    }
                                    Spacer(minLength: 40)
                                }
                            }
                            .id(message.id)
                        }
                        ForEach(viewModel.pending) { note in
                            HStack {
                                Spacer(minLength: 40)
                                PendingNoteCard(note: note) { viewModel.retryPending(note) }
                            }
                            .id(note.id)
                        }
                    }
                    .padding(.horizontal, 12)
                    .padding(.vertical, 16)
                }
                .scrollDismissesKeyboard(.interactively)
                .overlay {
                    if !viewModel.loading && viewModel.messages.isEmpty && viewModel.pending.isEmpty {
                        Text(emptyText(other))
                            .font(.pearlMessage).foregroundStyle(Palette.fg2)
                            .multilineTextAlignment(.center).padding(.horizontal, 40)
                    }
                }
                .onChange(of: viewModel.messages.count + viewModel.pending.count) {
                    let last = viewModel.pending.last?.id ?? viewModel.messages.last?.id
                    if let last { withAnimation { scroller.scrollTo(last, anchor: .bottom) } }
                }
            }
            composer(other: other)
        }
        .background(Palette.bg)
        .overlay(alignment: .top) {
            if !viewModel.connected {
                Toast(text: "Connection lost. Reconnecting…").padding(.top, 8)
            }
        }
        .toolbar {
            ToolbarItem(placement: .principal) {
                VStack(spacing: 0) {
                    Text(other?.shownName ?? (viewModel.room == nil ? "" : "Deleted account")).font(.pearlTitle).foregroundStyle(Palette.fg)
                    if let language = other?.lang {
                        Text("hears \(language.ownName)").font(.pearlMeta).foregroundStyle(Palette.them)
                    }
                }
                .accessibilityElement(children: .combine)
            }
            if let other {
                ToolbarItem(placement: .topBarTrailing) {
                    Menu {
                        Button { viewModel.reportPersonTapped() } label: {
                            Label("Report \(other.shownName)", systemImage: "flag")
                        }
                        if viewModel.youBlocked {
                            Button { viewModel.unblockTapped() } label: {
                                Label("Unblock \(other.shownName)", systemImage: "hand.raised.slash")
                            }
                        } else {
                            Button(role: .destructive) { confirmingBlock = true } label: {
                                Label("Block \(other.shownName)", systemImage: "hand.raised")
                            }
                        }
                    } label: {
                        Image(systemName: "ellipsis.circle").foregroundStyle(Palette.fg)
                    }
                    .accessibilityLabel("More")
                }
            }
        }
        .confirmationDialog("Block \(other?.shownName ?? "them")?", isPresented: $confirmingBlock, titleVisibility: .visible) {
            Button("Block", role: .destructive, action: viewModel.blockConfirmed)
        } message: {
            Text("Neither of you will be able to send messages here until you unblock them. They won't be told.")
        }
        .sheet(item: $viewModel.reporting) { target in
            ReportSheet(target: target, name: other?.shownName ?? "them") { reason, note, alsoBlock in
                await viewModel.sendReport(reason: reason, note: note, alsoBlock: alsoBlock)
            }
        }
        .alert("Thanks for telling us", isPresented: $viewModel.reportSent) {
            Button("OK", role: .cancel) {}
        } message: {
            Text("We look at every report within a day.")
        }
        .navigationBarTitleDisplayMode(.inline)
        .toolbar(.hidden, for: .tabBar)
        .task { await viewModel.follow() }
        .onChange(of: scenePhase) { _, phase in
            // Back from Settings with the microphone allowed puts the mic back.
            if phase == .active { viewModel.permissionChecked(granted: AVAudioApplication.shared.recordPermission == .granted) }
        }
        .sensoryFeedback(trigger: viewModel.notes.phase) { _, phase in
            switch phase {
            case .recording: .impact(weight: .light)
            case .stopped: .success
            case .idle, .paused: nil
            }
        }
    }

    @ViewBuilder
    private func card(_ message: Message) -> some View {
        if message.isText && message.mine {
            MyTextCard(
                message: message, playing: viewModel.playing == message.id, waiting: viewModel.waitingToHear.contains(message.id),
                onListen: { viewModel.listenTapped(message) }, onRetry: { viewModel.retryTapped(message) }
            )
        } else if message.isText {
            TheirTextCard(
                message: message, myLanguage: viewModel.myLanguage, playing: viewModel.playing == message.id,
                playingLength: viewModel.playingLength, waiting: viewModel.waitingToHear.contains(message.id),
                showOriginal: viewModel.originals.contains(message.id),
                onListen: { viewModel.listenTapped(message) }, onToggleOriginal: { viewModel.originalToggled(message) }
            )
        } else if message.mine {
            MyNoteCard(
                message: message, playing: viewModel.playing == message.id, playingLength: viewModel.playingLength,
                onPlay: { viewModel.playTapped(message) }, onRetry: { viewModel.retryTapped(message) }, onRecordVoice: onRecordVoice
            )
        } else {
            TheirNoteCard(
                message: message, myLanguage: viewModel.myLanguage, playing: viewModel.playing == message.id,
                playingLength: viewModel.playingLength, fresh: viewModel.fresh.contains(message.id),
                played: viewModel.played.contains(message.id), showOriginal: viewModel.originals.contains(message.id),
                onPlay: { viewModel.playTapped(message) }, onToggleOriginal: { viewModel.originalToggled(message) }
            )
        }
    }

    private func emptyText(_ other: Profile?) -> String {
        guard let other else { return "No one else is in this room." }
        return "Say something to \(other.displayName.isEmpty ? "them" : other.displayName), or write it. They'll get it in \(other.lang?.localName ?? "their language"), in your voice."
    }

    @ViewBuilder
    private func composer(other: Profile?) -> some View {
        VStack(spacing: 12) {
            if let problem = viewModel.problem {
                Notice(text: problem)
            }
            if let hint = viewModel.hint {
                Hint(text: hint.words, systemImage: hint == .silent ? "ear" : "arrow.counterclockwise")
            }
            switch viewModel.notes.phase {
            case _ where other == nil && viewModel.room != nil:
                EmptyView()
            case _ where viewModel.youBlocked:
                Banner(
                    systemImage: "hand.raised",
                    title: "You blocked \(other?.shownName ?? "them")",
                    message: "Neither of you can send messages here. They haven't been told.",
                    action: "Unblock",
                    onAction: viewModel.unblockTapped
                )
            case .recording, .paused:
                RecordingComposer(
                    notes: viewModel.notes, onPause: viewModel.pauseTapped, onResume: viewModel.resumeTapped,
                    onStop: viewModel.stopTapped, onSend: viewModel.sendNoteTapped, onCancel: viewModel.cancelTapped
                )
            case .stopped:
                ReviewComposer(
                    audio: viewModel.notes.audio, playing: viewModel.reviewing, onPlay: viewModel.reviewPlayTapped,
                    onDelete: viewModel.cancelTapped, onSend: viewModel.sendNoteTapped
                )
            case .idle:
                // Without the microphone the user can still write; the banner says how to record.
                if viewModel.micDenied {
                    Banner(
                        systemImage: "mic.slash",
                        title: "\(Brand.name) needs your microphone",
                        message: "To record, allow microphone access in Settings.",
                        action: "Open Settings"
                    ) {
                        if let url = URL(string: UIApplication.openSettingsURLString) { openURL(url) }
                    }
                }
                TextComposer(
                    draft: $viewModel.draft, showsMic: !viewModel.micDenied,
                    onSend: viewModel.sendTextTapped, onMic: viewModel.micTapped
                )
            }
        }
        .padding(.horizontal, 16)
        .padding(.bottom, 10)
        .padding(.top, 6)
    }
}

private extension RoomViewModel.Hint {
    /// The design's words for each recording that was not sent. No blame, and what to try next.
    var words: String {
        switch self {
        case .tooShort: "That was very short. Tap the mic, say something, then send."
        case .silent: "We didn't hear anything. Check nothing is covering the microphone, then try again."
        case .noSpeech: "No words came through. Try again, a little closer to the phone."
        case .unclear: "We couldn't quite make that out. Please say it again."
        }
    }
}

/// Typing a message, with the mic beside it; once there are words, the mic becomes send.
private struct TextComposer: View {
    @Binding var draft: String
    let showsMic: Bool
    let onSend: () -> Void
    let onMic: () -> Void

    var body: some View {
        let canSend = !draft.trimmingCharacters(in: .whitespacesAndNewlines).isEmpty
        HStack(alignment: .bottom, spacing: 10) {
            TextField("Message", text: $draft, axis: .vertical)
                .font(.pearlMessage).foregroundStyle(Palette.fg)
                .lineLimit(1...6)
                .textInputAutocapitalization(.sentences)
                .padding(.horizontal, 18).padding(.vertical, 14)
                .frame(minHeight: 52)
                .background(Palette.bgRaised, in: RoundedRectangle(cornerRadius: 26))
            if canSend {
                YouButton(systemImage: "arrow.up", label: "Send", action: onSend)
            } else if showsMic {
                YouButton(systemImage: "mic", label: "Record a voice note", action: onMic)
            }
        }
    }
}

/// A round button in the colour that means "you".
private struct YouButton: View {
    let systemImage: String
    let label: String
    let action: () -> Void

    var body: some View {
        Button(action: action) {
            Image(systemName: systemImage)
                .font(.system(size: 20, weight: .medium))
                .foregroundStyle(Palette.onYou)
                .frame(width: 52, height: 52)
                .background(Palette.you, in: Circle())
        }
        .buttonStyle(.plain)
        .accessibilityLabel(label)
    }
}

/// Recording, or paused: the live dots and the time, with the 30-second limit; then cancel, pause or resume, stop (to
/// listen before sending), and send.
private struct RecordingComposer: View {
    let notes: NoteRecorder
    let onPause: () -> Void
    let onResume: () -> Void
    let onStop: () -> Void
    let onSend: () -> Void
    let onCancel: () -> Void

    var body: some View {
        let paused = notes.phase == .paused
        VStack(spacing: 14) {
            TimelineView(.animation(minimumInterval: 0.1, paused: paused)) { context in
                let elapsed = notes.elapsed
                let left = max(RoomViewModel.maxNote - elapsed, .zero)
                let time = elapsed >= RoomViewModel.countdownFrom ? "\(Int((left / .seconds(1)).rounded(.up))) s left" : clock(elapsed)
                // The dots move while recording and rest while paused. They do not yet follow the microphone's level.
                let phase = context.date.timeIntervalSinceReferenceDate * 4.5
                HStack(spacing: 12) {
                    DotWaveform(
                        levels: (0..<16).map { paused ? 0.3 : Float(0.3 + 0.7 * (sin(phase + Double($0) * 0.9) + 1) / 2) },
                        color: Palette.fg.opacity(paused ? 0.4 : 1)
                    )
                    Text(paused ? "Paused · \(time)" : time).font(.pearlMeta).monospacedDigit().foregroundStyle(Palette.fg)
                }
                .accessibilityElement(children: .ignore)
                .accessibilityLabel(paused ? "Paused, \(time)" : "Recording, \(time)")
            }
            HStack(alignment: .bottom) {
                ComposerControl(systemImage: "xmark", label: "Cancel", action: onCancel)
                Spacer()
                if paused {
                    ComposerControl(systemImage: "mic", label: "Resume", action: onResume)
                } else {
                    ComposerControl(systemImage: "pause.fill", label: "Pause", action: onPause)
                }
                Spacer()
                ComposerControl(systemImage: "stop.fill", label: "Stop", action: onStop)
                Spacer()
                SendControl(action: onSend)
            }
        }
    }
}

/// A stopped note, to listen to before sending: play, its dots and length; then delete or send.
private struct ReviewComposer: View {
    let audio: PCMAudio?
    let playing: Bool
    let onPlay: () -> Void
    let onDelete: () -> Void
    let onSend: () -> Void

    var body: some View {
        VStack(spacing: 14) {
            HStack(spacing: 12) {
                Button(action: onPlay) {
                    Image(systemName: playing ? "pause.fill" : "play.fill")
                        .font(.system(size: 15)).foregroundStyle(Palette.onYou)
                        .frame(width: 44, height: 44)
                        .background(Palette.you, in: Circle())
                }
                .buttonStyle(.plain)
                .accessibilityLabel(playing ? "Stop" : "Listen to your note")
                DotWaveform(levels: audio?.levels(22) ?? Array(repeating: 0.3, count: 22), color: Palette.you)
                Text(clock(audio?.duration ?? .zero)).font(.pearlMeta).monospacedDigit().foregroundStyle(Palette.fg2)
            }
            .padding(.horizontal, 12).padding(.vertical, 10)
            .background(Palette.bgRaised, in: RoundedRectangle(cornerRadius: 12))
            HStack(alignment: .bottom) {
                ComposerControl(systemImage: "trash", label: "Delete", action: onDelete)
                Spacer()
                SendControl(action: onSend)
            }
        }
    }
}

/// An outlined round control with its name under it.
private struct ComposerControl: View {
    let systemImage: String
    let label: String
    let action: () -> Void

    var body: some View {
        Button(action: action) {
            VStack(spacing: 6) {
                Image(systemName: systemImage)
                    .font(.system(size: 18)).foregroundStyle(Palette.fg)
                    .frame(width: 52, height: 52)
                    .overlay(Circle().stroke(Palette.fg.opacity(0.28)))
                Text(label).font(.pearlMeta).foregroundStyle(Palette.fg2)
            }
            .padding(4)
            .contentShape(Rectangle())
        }
        .buttonStyle(.plain)
        .accessibilityElement(children: .ignore)
        .accessibilityLabel(label)
        .accessibilityAddTraits(.isButton)
    }
}

/// Send, the composer's main action: the circle that means "you", in its halo.
private struct SendControl: View {
    let action: () -> Void

    var body: some View {
        Button(action: action) {
            VStack(spacing: 6) {
                Image(systemName: "arrow.up")
                    .font(.system(size: 22, weight: .medium)).foregroundStyle(Palette.onYou)
                    .frame(width: 56, height: 56)
                    .background(Palette.you, in: Circle())
                    .padding(8)
                    .background(Palette.youHalo, in: Circle())
                Text("Send").font(.pearlMeta).foregroundStyle(Palette.fg)
            }
            .padding(4)
        }
        .buttonStyle(.plain)
        .accessibilityElement(children: .ignore)
        .accessibilityLabel("Send")
        .accessibilityAddTraits(.isButton)
    }
}
