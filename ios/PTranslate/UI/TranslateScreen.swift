import SwiftUI

/// The interpreter: speak in one language, and it is said in another, in your own voice. Laid out as the design's
/// interpreter: the two languages, what was said and its translation, then the record control and a line saying what
/// is happening, so the screen never looks frozen.
///
/// - Parameters:
///   - hasOwnVoice: whether the user has recorded their voice, which changes some of the wording.
///   - onSettingsTapped: opens settings.
struct TranslateScreen: View {
    let viewModel: TranslateViewModel
    let hasOwnVoice: Bool
    let onSettingsTapped: () -> Void

    var body: some View {
        InterpreterContent(
            state: InterpreterState(
                phase: viewModel.phase,
                stage: viewModel.stage,
                text: viewModel.text,
                translation: viewModel.translation,
                error: viewModel.error,
                source: viewModel.source,
                target: viewModel.target,
                sourceLanguages: viewModel.sourceLanguages,
                targetLanguages: viewModel.targetLanguages
            ),
            hasOwnVoice: hasOwnVoice,
            actions: InterpreterActions(
                record: viewModel.recordTapped,
                stopSpeaking: viewModel.stopSpeakingTapped,
                selectSource: { viewModel.sourceSelected($0) },
                selectTarget: { viewModel.targetSelected($0) },
                swap: viewModel.swapLanguagesTapped,
                settings: onSettingsTapped
            )
        )
        .task { await viewModel.loadLanguages() }
    }
}

/// What the interpreter shows, gathered so the view can be previewed without a view model.
struct InterpreterState {
    var phase: RecordingPhase
    var stage: InterpreterStage
    var text: String
    var translation: String
    var error: String?
    var source: Language
    var target: Language
    var sourceLanguages: [Language]
    var targetLanguages: [Language]
}

struct InterpreterActions {
    var record: () -> Void
    var stopSpeaking: () -> Void
    var selectSource: (Language) -> Void
    var selectTarget: (Language) -> Void
    var swap: () -> Void
    var settings: () -> Void
}

private enum Side: Identifiable {
    case source, target
    var id: Self { self }
}

private struct InterpreterContent: View {
    let state: InterpreterState
    let hasOwnVoice: Bool
    let actions: InterpreterActions

    @State private var picking: Side?
    @State private var recordingSince: Date?

    private var idle: Bool { state.phase == .idle }

    private var speaking: Bool {
        if case .speaking = state.stage { true } else { false }
    }

    var body: some View {
        VStack(spacing: 0) {
            // Settings in a row of its own, so long language names below never run into it.
            HStack {
                Spacer()
                Button(action: actions.settings) {
                    Image(systemName: "slider.horizontal.3")
                        .font(.system(size: 17))
                        .foregroundStyle(Palette.fg2)
                        .frame(width: 36, height: 36)
                }
                .buttonStyle(.glass)
                .buttonBorderShape(.circle)
                .disabled(!idle)
                .accessibilityLabel("Settings")
            }
            .padding(.trailing, -12)

            LanguageChips(
                source: state.source,
                target: state.target,
                enabled: idle,
                swapEnabled: state.sourceLanguages.contains(state.target) && state.targetLanguages.contains(state.source),
                onSourceTapped: { picking = .source },
                onTargetTapped: { picking = .target },
                onSwapTapped: actions.swap
            )
            .padding(.top, 4)

            // What was said and what it became. Long translations scroll; they are never cut short.
            ScrollView {
                VStack(spacing: 16) {
                    if state.text.isEmpty && state.translation.isEmpty && state.stage == .idle {
                        Text("What you say appears here, and under it what they will hear.")
                            .font(.pearlMessage)
                            .foregroundStyle(Palette.fg3)
                    }
                    if !state.text.isEmpty {
                        Text(state.text)
                            .font(.pearlMessage)
                            .foregroundStyle(Palette.fg2)
                    }
                    if !state.translation.isEmpty {
                        Text(state.translation)
                            .font(.pearlTranslation)
                            .tracking(-0.4)
                            .foregroundStyle(Palette.them)
                    }
                    if let error = state.error {
                        Notice(text: error)
                    }
                }
                .multilineTextAlignment(.center)
                .frame(maxWidth: .infinity)
                .padding(.vertical, 24)
            }
            .defaultScrollAnchor(.center)

            TimelineView(.periodic(from: recordingSince ?? .now, by: 0.2)) { context in
                let elapsed = recordingSince.map { Duration.seconds(context.date.timeIntervalSince($0)) } ?? .zero
                VStack(spacing: 4) {
                    RecordControl(
                        state: controlState,
                        label: controlLabel,
                        value: state.stage == .recording ? clock(elapsed) : nil,
                        enabled: state.phase != .working || speaking,
                        area: 220,
                        circle: 88,
                        action: speaking ? actions.stopSpeaking : actions.record
                    )
                    let words = words(elapsed: elapsed)
                    Text(words.headline)
                        .font(.pearlHeadline)
                        .foregroundStyle(Palette.fg)
                        .monospacedDigit()
                    Text(words.subline ?? " ")
                        .font(.pearlSmall)
                        .foregroundStyle(Palette.fg2)
                }
                .multilineTextAlignment(.center)
            }
            .padding(.bottom, 16)
        }
        .padding(.horizontal, 28)
        .frame(maxWidth: .infinity, maxHeight: .infinity)
        .background(Palette.bg.ignoresSafeArea())
        .onChange(of: state.stage) { _, stage in
            recordingSince = stage == .recording ? .now : nil
        }
        .sensoryFeedback(trigger: state.stage) { old, new in
            // A light tap when recording starts, a confirmation when it stops, as the design asks.
            if new == .recording { return .impact(weight: .light) }
            if old == .recording { return .success }
            return nil
        }
        .sheet(item: $picking) { side in
            switch side {
            case .source:
                LanguagePicker(title: "You speak", options: state.sourceLanguages, selected: state.source, onSelect: actions.selectSource)
            case .target:
                LanguagePicker(title: "They hear", options: state.targetLanguages, selected: state.target, onSelect: actions.selectTarget)
            }
        }
    }

    private var controlState: RecordControlState {
        switch state.stage {
        case .recording: .recording
        case .preparingSpeech: .preparingVoice
        case .speaking: .speaking
        case .idle, .done, .failed: .idle
        default: .working
        }
    }

    private var controlLabel: String {
        switch state.stage {
        case .recording: "Stop recording"
        case .speaking: "Stop speaking"
        case .preparingSpeech: "Preparing the voice"
        case .idle, .done, .failed: "Start recording"
        default: "Working"
        }
    }

    /// The headline and the line under it, for each stage of a run.
    private func words(elapsed: Duration) -> (headline: String, subline: String?) {
        let source = state.source.ownName
        let target = state.target.ownName
        switch state.stage {
        case .idle: return ("Tap to speak", "You speak \(source). They hear \(target).")
        case .recording: return ("Listening · \(clock(elapsed))", "Tap to stop when you are done")
        case .preparingAudio, .transcribing: return ("Writing down what you said…", "Usually a second or two")
        case .uploading: return ("Writing down what you said…", "Sending it to the server")
        case .queued: return ("Writing down what you said…", "Waiting for the server")
        case .reconnecting: return ("Writing down what you said…", "Connection lost. Reconnecting")
        case .downloadingLanguage: return ("Getting \(target) ready…", "Only the first time")
        case .translating: return ("Translating into \(target)…", nil)
        case .preparingSpeech: return (hasOwnVoice ? "Preparing your voice…" : "Preparing speech…", "Usually a few seconds")
        case .speaking(.own): return ("Speaking in your voice", "Tap to stop")
        case .speaking(.phone):
            return (
                "Speaking in the phone's voice",
                hasOwnVoice ? "Your voice could not be made this time" : "Record your voice in Settings to hear yourself"
            )
        case .done: return ("Tap to speak again", "You speak \(source). They hear \(target).")
        case .failed: return ("Tap to try again", nil)
        }
    }
}

#Preview("Preparing your voice") {
    InterpreterContent(
        state: InterpreterState(
            phase: .working, stage: .preparingSpeech,
            text: "Where can I buy a ticket for the night train to Seville?",
            translation: "¿Dónde puedo comprar un billete para el tren nocturno a Sevilla?",
            error: nil, source: .english, target: .spanish, sourceLanguages: [.english, .spanish],
            targetLanguages: [.english, .spanish]
        ),
        hasOwnVoice: true,
        actions: InterpreterActions(record: {}, stopSpeaking: {}, selectSource: { _ in }, selectTarget: { _ in }, swap: {}, settings: {})
    )
    .preferredColorScheme(.dark)
}
