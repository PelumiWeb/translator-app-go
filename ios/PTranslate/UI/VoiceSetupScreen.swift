import SwiftUI

/// About twenty seconds when read aloud at an ordinary pace. Varied sounds and a natural rhythm give a voice-cloning
/// engine more to work with than a list of words would. Split into the lines the teleprompter shows; joined, it must
/// stay word for word the same as on Android and on the server (`provider.OnboardingPassage`), which tells the voice
/// engine what was said.
private let passageLines = [
    "When I travel, I like to talk with the people I meet",
    "about their food, their music, and the places they love.",
    "A good conversation makes the world feel smaller.",
    "I am recording this so that when my words are translated,",
    "they can still be spoken in my own voice.",
]

/// Where the user records the sample of their voice that translations are spoken in.
struct VoiceSetupScreen: View {
    let viewModel: VoiceSetupViewModel
    let onDone: () -> Void

    var body: some View {
        VoiceSetupContent(
            stage: viewModel.stage,
            recordingStarted: viewModel.recordingStarted,
            recorded: viewModel.recorded,
            levels: viewModel.levels,
            problem: viewModel.problem,
            playing: viewModel.playing,
            saved: viewModel.saved,
            error: viewModel.error,
            actions: VoiceSetupActions(
                record: viewModel.recordTapped,
                listen: viewModel.listenTapped,
                use: viewModel.useTapped,
                skip: viewModel.skipTapped,
                delete: viewModel.deleteTapped
            )
        )
        // Leaving is an event, not a state to stay in: tell the caller once, and reset so the screen starts fresh
        // the next time it is opened.
        .onChange(of: viewModel.finished) { _, finished in
            if finished {
                onDone()
                viewModel.left()
            }
        }
    }
}

struct VoiceSetupActions {
    var record: () -> Void
    var listen: () -> Void
    var use: () -> Void
    var skip: () -> Void
    var delete: () -> Void
}

/// Stateless, so it can be previewed without a view model.
private struct VoiceSetupContent: View {
    let stage: VoiceSetupStage
    let recordingStarted: Date?
    let recorded: Duration
    let levels: [Float]
    let problem: String?
    let playing: Bool
    let saved: Duration?
    let error: String?
    let actions: VoiceSetupActions

    @State private var playbackStarted: Date?

    var body: some View {
        ScrollView {
            VStack(spacing: 12) {
                switch stage {
                case .intro: intro
                case .recording: recordingInProgress
                case .review: review
                }
                if let error {
                    Notice(text: error)
                }
            }
            .multilineTextAlignment(.center)
            .frame(maxWidth: .infinity)
            .padding(.horizontal, 28)
            .padding(.vertical, 24)
        }
        .foregroundStyle(Palette.fg)
        .background(Palette.bg.ignoresSafeArea())
        .sensoryFeedback(trigger: stage) { old, new in
            // A light tap when recording starts, a confirmation when it stops, as the design asks.
            if new == .recording { return .impact(weight: .light) }
            if old == .recording { return .success }
            return nil
        }
        .onChange(of: playing) { _, playing in
            playbackStarted = playing ? .now : nil
        }
    }

    @ViewBuilder
    private var intro: some View {
        Text("Your voice")
            .font(.pearlLabel)
        Text(saved == nil ? "Be heard in your own voice" : "Your voice is recorded")
            .font(.pearlTitle)
        Text(
            saved.map { "\($0.components.seconds) seconds, kept on this phone. Record a new one, keep it, or delete it." }
                ?? "Read the lines below aloud, at your normal pace. About 20 seconds."
        )
        .font(.pearlBody)
        .foregroundStyle(Palette.fg2)
        Teleprompter(current: 0)
            .padding(.vertical, 16)
        RecordControl(state: .idle, label: saved == nil ? "Start recording" : "Record a new one", action: actions.record)
        if saved == nil {
            LinkButton("Not now, use the phone's voice", action: actions.skip)
        } else {
            HStack(spacing: 8) {
                LinkButton("Keep it", action: actions.skip)
                LinkButton("Delete my voice", action: actions.delete)
            }
        }
        // Said before the user records, in plain words: what happens to it.
        Text(
            "The recording stays on this phone. When a translation is spoken, it goes with the text to "
                + "our server, and from there to Fish Audio, the service that makes the speech. Our server "
                + "deletes it straight after. You can delete it from this phone at any time."
        )
        .font(.pearlMeta)
        .foregroundStyle(Palette.fg2)
        .padding(.top, 8)
    }

    @ViewBuilder
    private var recordingInProgress: some View {
        // Redrawn five times a second while recording; the elapsed time is never stored anywhere.
        TimelineView(.periodic(from: recordingStarted ?? .now, by: 0.2)) { context in
            let elapsed = Duration.seconds(context.date.timeIntervalSince(recordingStarted ?? context.date))
            VStack(spacing: 12) {
                Text("Listening")
                    .font(.pearlLabel)
                Text(clock(elapsed))
                    .font(.pearlTimer)
                    .tracking(-0.9)
                    .monospacedDigit()
                Text(
                    elapsed < VoiceSampleRules.minimum ? "Keep going. A few more seconds."
                        : elapsed < VoiceSampleRules.target ? "Keep going. Read to the end."
                        : "That's enough. Tap to stop."
                )
                .font(.pearlSmall)
                .foregroundStyle(Palette.fg2)
                Teleprompter(current: lineAt(elapsed))
                    .padding(.vertical, 24)
                RecordControl(state: .recording, label: "Stop recording", value: clock(elapsed), action: actions.record)
            }
        }
        LinkButton("Not now, use the phone's voice", action: actions.skip)
    }

    @ViewBuilder
    private var review: some View {
        Text("Recorded")
            .font(.pearlLabel)
        Text(clock(recorded))
            .font(.pearlTimer)
            .tracking(-0.9)
            .monospacedDigit()
        Text(problem == nil ? "Listen back, then keep it." : "This one cannot be kept.")
            .font(.pearlSmall)
            .foregroundStyle(Palette.fg2)
        // While it plays, the played part of the waveform fills in.
        TimelineView(.animation(paused: !playing)) { context in
            let total = max(recorded / .seconds(1), 0.1)
            let played = playbackStarted.map { min(context.date.timeIntervalSince($0) / total, 1) } ?? 1
            HStack(spacing: 12) {
                Button(action: actions.listen) {
                    Image(systemName: playing ? "stop.fill" : "play.fill")
                        .font(.system(size: 15))
                        .foregroundStyle(Palette.onYou)
                        .frame(width: 44, height: 44)
                        .background(Palette.you, in: Circle())
                }
                .buttonStyle(.plain)
                .accessibilityLabel(playing ? "Stop listening" : "Listen")
                DotWaveform(levels: levels, color: Palette.you, played: played)
            }
        }
        .padding(.vertical, 16)
        if let problem {
            Notice(text: problem)
        }
        Button("Use this recording", action: actions.use)
            .buttonStyle(PrimaryButtonStyle())
            .disabled(problem != nil)
            .padding(.top, 8)
        LinkButton("Record again", action: actions.record)
        LinkButton(saved == nil ? "Not now, use the phone's voice" : "Cancel, keep my saved voice", action: actions.skip)
    }
}

/// The passage as a teleprompter: the line to read now, large, between the line just read and the one coming next.
private struct Teleprompter: View {
    let current: Int

    var body: some View {
        VStack(spacing: 14) {
            Text(current > 0 ? passageLines[current - 1] : " ")
                .font(.pearlPassageOther)
                .foregroundStyle(Palette.fg4)
                .frame(minHeight: 44, alignment: .bottom)
            Text(passageLines[current])
                .font(.pearlPassageCurrent)
                .tracking(-0.25)
                .foregroundStyle(Palette.fg)
            Text(current + 1 < passageLines.count ? passageLines[current + 1] : " ")
                .font(.pearlPassageOther)
                .foregroundStyle(Palette.fg3)
                .frame(minHeight: 44, alignment: .top)
        }
        .multilineTextAlignment(.center)
        .animation(.easeInOut(duration: 0.4), value: current)
        .contentTransition(.opacity)
    }
}

/// Which line the reader has probably reached: the passage is paced to take the target time, each line its share by
/// length. There is no listening for the words; a reader ahead or behind still has the lines around it.
private func lineAt(_ elapsed: Duration) -> Int {
    let total = passageLines.reduce(0) { $0 + $1.count }
    let reached = elapsed / VoiceSampleRules.target * Double(total)
    var end = 0
    for (index, line) in passageLines.enumerated() {
        end += line.count
        if reached < Double(end) { return index }
    }
    return passageLines.count - 1
}

private extension VoiceSetupActions {
    /// Buttons that do nothing, for previews.
    static var inert: Self { Self(record: {}, listen: {}, use: {}, skip: {}, delete: {}) }
}

#Preview("Recording") {
    VoiceSetupContent(
        stage: .recording, recordingStarted: .now.addingTimeInterval(-12), recorded: .zero, levels: [], problem: nil,
        playing: false, saved: nil, error: nil, actions: .inert
    )
    .preferredColorScheme(.dark)
}

#Preview("Too short") {
    VoiceSetupContent(
        stage: .review, recordingStarted: nil, recorded: .seconds(4), levels: (0..<22).map { Float($0 % 5) / 4 },
        problem: "That was only 4 seconds. About 20 are needed: please read the whole passage",
        playing: false, saved: nil, error: nil, actions: .inert
    )
    .preferredColorScheme(.dark)
}
