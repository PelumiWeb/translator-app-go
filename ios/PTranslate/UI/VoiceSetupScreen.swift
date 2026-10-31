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

/// Where the user records the sample of their voice that their words are spoken in, and agrees to it being kept.
/// During sign-up, `step` numbers it and `onBack` goes to the step before.
struct VoiceSetupScreen: View {
    let viewModel: VoiceSetupViewModel
    var step: Int?
    var onBack: (() -> Void)?
    let onDone: () -> Void

    var body: some View {
        VoiceSetupContent(
            step: step,
            onBack: onBack,
            keeping: viewModel.keeping,
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
    var step: Int?
    var onBack: (() -> Void)?
    var keeping = false
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
        VStack(spacing: 0) {
            if let step {
                OnboardingHeader(step: step, total: onboardingSteps, onBack: onBack)
                    .padding(.horizontal, 20)
            }
            if stage == .review {
                consent
            } else {
                ScrollView {
                    VStack(spacing: 12) {
                        if stage == .intro { intro } else { recordingInProgress }
                        if let error {
                            Notice(text: error)
                        }
                    }
                    .multilineTextAlignment(.center)
                    .frame(maxWidth: .infinity)
                    .padding(.horizontal, 28)
                    .padding(.vertical, 24)
                }
            }
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
            saved.map { "\($0.components.seconds) seconds. Record a new one, or keep the one you have." }
                ?? "Read the lines below aloud, at your normal pace. About 20 seconds. You'll hear it back before anything is kept."
        )
        .font(.pearlBody)
        .foregroundStyle(Palette.fg2)
        Teleprompter(current: 0)
            .padding(.vertical, 16)
        RecordControl(state: .idle, label: saved == nil ? "Start recording" : "Record a new one", action: actions.record)
        LinkButton(saved == nil ? "Not now, use the phone's voice" : "Keep the one I have", action: actions.skip)
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

    /// After recording: listen back, and agree to the voice being kept, in the design's plain words: what is kept,
    /// what it is for, how to delete it.
    private var consent: some View {
        VStack(spacing: 0) {
            ScrollView {
                VStack(alignment: .leading, spacing: 20) {
                    Text(problem == nil ? "That's your voice. Keep it?" : "Let's try that again.")
                        .font(.pearlScreenTitle)
                        .accessibilityAddTraits(.isHeader)
                    // While it plays, the played part of the waveform fills in.
                    TimelineView(.animation(paused: !playing)) { context in
                        let total = max(recorded / .seconds(1), 0.1)
                        let played = playbackStarted.map { min(context.date.timeIntervalSince($0) / total, 1) } ?? 1
                        HStack(spacing: 12) {
                            Button(action: actions.listen) {
                                Image(systemName: playing ? "pause.fill" : "play.fill")
                                    .font(.system(size: 15))
                                    .foregroundStyle(Palette.you)
                                    .frame(width: 44, height: 44)
                                    .background(Palette.onYou, in: Circle())
                            }
                            .buttonStyle(.plain)
                            .accessibilityLabel(playing ? "Stop listening" : "Listen")
                            DotWaveform(levels: levels, color: Palette.onYou, played: played)
                            Text(clock(recorded)).font(.pearlMeta).monospacedDigit().foregroundStyle(Palette.onYou)
                        }
                        .padding(12)
                        .background(Palette.you, in: RoundedRectangle(cornerRadius: 12))
                    }
                    if let problem {
                        Hint(text: problem, systemImage: "ear")
                    } else {
                        ConsentBlock(title: "What we keep", text: "This \(recorded.components.seconds)-second recording, on our server.")
                        ConsentBlock(
                            title: "What it's for",
                            text: "Only to speak your own words, when someone plays a message you sent. Fish Audio makes the speech for us."
                        )
                        ConsentBlock(title: "Delete it any time", text: "In Profile, under Your voice. Your messages will then use the phone's voice.")
                    }
                    if let error {
                        Notice(text: error)
                    }
                }
                .frame(maxWidth: .infinity, alignment: .leading)
                .padding(.horizontal, 28)
                .padding(.vertical, 16)
            }
            VStack(spacing: 10) {
                if problem == nil {
                    Button(keeping ? "Keeping…" : "Keep my voice", action: actions.use)
                        .buttonStyle(PrimaryButtonStyle())
                        .disabled(keeping)
                    Button("Record again", action: actions.record)
                        .buttonStyle(SecondaryButtonStyle())
                        .disabled(keeping)
                } else {
                    Button("Record again", action: actions.record).buttonStyle(PrimaryButtonStyle())
                }
                LinkButton(saved == nil ? "Use the phone's voice instead" : "Keep the one I have", action: actions.skip)
            }
            .padding(.horizontal, 16)
            .padding(.bottom, 8)
        }
    }
}

private struct ConsentBlock: View {
    let title: String
    let text: String

    var body: some View {
        VStack(alignment: .leading, spacing: 4) {
            Text(title).font(.custom("IBMPlexSans-Medm", size: 17, relativeTo: .body)).foregroundStyle(Palette.fg)
            Text(text).font(.pearlLead).foregroundStyle(Palette.fg2).lineSpacing(4)
        }
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
