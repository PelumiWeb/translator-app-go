import SwiftUI

/// About twenty seconds when read aloud at an ordinary pace. Varied sounds and a natural rhythm give a
/// voice-cloning engine more to work with than a list of words would. The same passage as on Android.
private let passage = """
    When I travel, I like to talk with the people I meet about their food, their music, and the places they love. \
    A good conversation makes the world feel smaller. I am recording this so that when my words are translated, \
    they can still be spoken in my own voice.
    """

/// Where the user records the sample of their voice that translations are spoken in.
struct VoiceSetupScreen: View {
    let viewModel: VoiceSetupViewModel
    let onDone: () -> Void

    var body: some View {
        VoiceSetupContent(
            stage: viewModel.stage,
            recordingStarted: viewModel.recordingStarted,
            recorded: viewModel.recorded,
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
    let problem: String?
    let playing: Bool
    let saved: Duration?
    let error: String?
    let actions: VoiceSetupActions

    var body: some View {
        ScrollView {
            VStack(alignment: .leading, spacing: 16) {
                Text("Record your voice")
                    .font(.title2.bold())
                Text(
                    "Your translations can be spoken in your own voice. To do that, the app needs a short "
                        + "recording of you reading the passage below."
                )
                // Said before the user records, in plain words: what happens to it.
                Text(
                    "The recording stays on this phone. When a translation is spoken, it is sent to the server "
                        + "together with the text, used once to make the speech, and deleted. You can delete it "
                        + "from this phone at any time."
                )
                .font(.footnote)
                .foregroundStyle(.secondary)

                Text(passage)
                    .padding(16)
                    .frame(maxWidth: .infinity, alignment: .leading)
                    .background(.quaternary, in: RoundedRectangle(cornerRadius: 12))

                switch stage {
                case .intro: intro
                case .recording: recordingInProgress
                case .review: review
                }

                if let error {
                    Text(error)
                        .foregroundStyle(.red)
                }
            }
            .frame(maxWidth: .infinity, alignment: .leading)
            .padding(24)
        }
    }

    @ViewBuilder
    private var intro: some View {
        if let saved {
            Text("A recording of your voice (\(saved.components.seconds) s) is saved on this phone.")
        }
        Button(saved == nil ? "Start recording" : "Record a new one", action: actions.record)
            .buttonStyle(.borderedProminent)
        if saved == nil {
            Button("Not now, use the phone's voice", action: actions.skip)
        } else {
            HStack(spacing: 12) {
                Button("Keep it", action: actions.skip)
                    .buttonStyle(.bordered)
                Button("Delete my voice", role: .destructive, action: actions.delete)
            }
        }
    }

    @ViewBuilder
    private var recordingInProgress: some View {
        // Redrawn ten times a second while recording; the elapsed time is never stored anywhere.
        TimelineView(.periodic(from: recordingStarted ?? .now, by: 0.1)) { context in
            let elapsed = context.date.timeIntervalSince(recordingStarted ?? context.date)
            let target = VoiceSampleRules.target / .seconds(1)
            VStack(alignment: .leading, spacing: 8) {
                Text("Recording: \(Int(elapsed)) s")
                    .font(.headline)
                ProgressView(value: min(elapsed / target, 1))
                Text(elapsed >= target ? "That is enough. You can stop" : "Read the passage aloud, at your normal pace")
                    .font(.footnote)
            }
        }
        Button("Stop", action: actions.record)
            .buttonStyle(.borderedProminent)
    }

    @ViewBuilder
    private var review: some View {
        Text("Recorded \(recorded.components.seconds) seconds")
            .font(.headline)
        if let problem {
            Text(problem)
                .foregroundStyle(.red)
        }
        HStack(spacing: 12) {
            Button(playing ? "Stop listening" : "Listen", action: actions.listen)
                .buttonStyle(.bordered)
            Button("Record again", action: actions.record)
                .buttonStyle(.bordered)
        }
        Button("Use this recording", action: actions.use)
            .buttonStyle(.borderedProminent)
            .disabled(problem != nil)
        Button(saved == nil ? "Not now, use the phone's voice" : "Cancel, keep my saved voice", action: actions.skip)
    }
}

private extension VoiceSetupActions {
    /// Buttons that do nothing, for previews.
    static var inert: Self { Self(record: {}, listen: {}, use: {}, skip: {}, delete: {}) }
}

#Preview("Recording") {
    VoiceSetupContent(
        stage: .recording, recordingStarted: .now.addingTimeInterval(-12), recorded: .zero, problem: nil,
        playing: false, saved: nil, error: nil, actions: .inert
    )
}

#Preview("Too short") {
    VoiceSetupContent(
        stage: .review, recordingStarted: nil, recorded: .seconds(4),
        problem: "That was only 4 seconds. About 20 are needed: please read the whole passage",
        playing: false, saved: nil, error: nil, actions: .inert
    )
}
