import AVFoundation
import SwiftUI

/// A conversation: the notes, newest at the bottom, and the mic.
struct RoomScreen: View {
    let viewModel: RoomViewModel
    let onRecordVoice: () -> Void

    @Environment(\.openURL) private var openURL
    @Environment(\.scenePhase) private var scenePhase

    var body: some View {
        let other = viewModel.room?.other
        VStack(spacing: 0) {
            ScrollViewReader { scroller in
                ScrollView {
                    LazyVStack(spacing: 14) {
                        ForEach(viewModel.messages) { message in
                            HStack {
                                if message.mine { Spacer(minLength: 40) }
                                card(message)
                                if !message.mine { Spacer(minLength: 40) }
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
        }
        .navigationBarTitleDisplayMode(.inline)
        .toolbar(.hidden, for: .tabBar)
        .task { await viewModel.follow() }
        .onChange(of: scenePhase) { _, phase in
            // Back from Settings with the microphone allowed puts the mic back.
            if phase == .active { viewModel.permissionChecked(granted: AVAudioApplication.shared.recordPermission == .granted) }
        }
        .sensoryFeedback(trigger: viewModel.recording) { _, recording in recording ? .impact(weight: .light) : .success }
    }

    @ViewBuilder
    private func card(_ message: Message) -> some View {
        if message.mine {
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
        return "Say something to \(other.displayName.isEmpty ? "them" : other.displayName). They'll hear it in \(other.lang?.localName ?? "their language"), in your voice."
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
            if other == nil && viewModel.room != nil {
                EmptyView()
            } else if viewModel.micDenied {
                Banner(
                    systemImage: "mic.slash",
                    title: "Timbre needs your microphone",
                    message: "To record, allow microphone access in Settings.",
                    action: "Open Settings"
                ) {
                    if let url = URL(string: UIApplication.openSettingsURLString) { openURL(url) }
                }
            } else if let started = viewModel.recordingStarted {
                RecordingComposer(started: started, onStop: viewModel.micTapped, onCancel: viewModel.cancelTapped)
            } else {
                Button(action: viewModel.micTapped) {
                    Image(systemName: "mic")
                        .font(.system(size: 24))
                        .foregroundStyle(Palette.onYou)
                        .frame(width: 68, height: 68)
                        .background(Palette.you, in: Circle())
                        .padding(14)
                        .background(Palette.youHalo, in: Circle())
                }
                .buttonStyle(.plain)
                .accessibilityLabel("Record a voice note")
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
        case .tooShort: "That was very short. Tap, say something, then tap again to send."
        case .silent: "We didn't hear anything. Check nothing is covering the microphone, then try again."
        case .noSpeech: "No words came through. Try again, a little closer to the phone."
        case .unclear: "We couldn't quite make that out. Please say it again."
        }
    }
}

/// Recording: cancel, the live dots, and stop to send, with the time and the 30-second limit.
private struct RecordingComposer: View {
    let started: Date
    let onStop: () -> Void
    let onCancel: () -> Void

    var body: some View {
        TimelineView(.periodic(from: started, by: 0.1)) { context in
            let elapsed = Duration.seconds(context.date.timeIntervalSince(started))
            let left = max(RoomViewModel.maxNote - elapsed, .zero)
            let time = elapsed >= RoomViewModel.countdownFrom ? "\(Int((left / .seconds(1)).rounded(.up))) s left" : clock(elapsed)
            // The dots move while recording. They do not yet follow the microphone's level.
            let phase = context.date.timeIntervalSince(started) * 4.5
            VStack(spacing: 4) {
                HStack(spacing: 12) {
                    Button(action: onCancel) {
                        Image(systemName: "xmark").font(.system(size: 16)).foregroundStyle(Palette.fg)
                            .frame(width: 44, height: 44)
                            .overlay(Circle().stroke(Palette.fg.opacity(0.28)))
                    }
                    .buttonStyle(.plain)
                    .accessibilityLabel("Cancel")
                    DotWaveform(levels: (0..<16).map { Float(0.3 + 0.7 * (sin(phase + Double($0) * 0.9) + 1) / 2) }, color: Palette.fg)
                    Button(action: onStop) {
                        RoundedRectangle(cornerRadius: 4).fill(Palette.onYou).frame(width: 22, height: 22)
                            .frame(width: 64, height: 64)
                            .background(Palette.you, in: Circle())
                            .padding(10)
                            .background(Palette.youHalo, in: Circle())
                    }
                    .buttonStyle(.plain)
                    .accessibilityLabel("Stop and send")
                    .accessibilityValue(time)
                }
                HStack {
                    Text("Cancel").foregroundStyle(Palette.fg2)
                    Spacer()
                    Text("\(time) · tap to send").monospacedDigit().foregroundStyle(Palette.fg)
                }
                .font(.pearlMeta)
            }
        }
    }
}
