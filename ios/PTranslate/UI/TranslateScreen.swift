import SwiftUI

/// - Parameters:
///   - savedVoice: length of the user's saved voice recording, or nil if they have not made one.
///   - onVoiceTapped: opens the screen where it is recorded or changed.
struct TranslateScreen: View {
    let viewModel: TranslateViewModel
    let savedVoice: Duration?
    let onVoiceTapped: () -> Void

    var body: some View {
        TranslateContent(
            phase: viewModel.phase,
            status: viewModel.status,
            text: viewModel.text,
            error: viewModel.error,
            savedVoice: savedVoice,
            onRecord: viewModel.recordTapped,
            onVoice: onVoiceTapped
        )
    }
}

/// Stateless, so it can be previewed without a view model.
private struct TranslateContent: View {
    let phase: RecordingPhase
    let status: String
    let text: String
    let error: String?
    let savedVoice: Duration?
    let onRecord: () -> Void
    let onVoice: () -> Void

    var body: some View {
        VStack(alignment: .leading, spacing: 16) {
            HStack(spacing: 8) {
                Text(
                    savedVoice.map { "Your voice is recorded (\($0.components.seconds) s)" }
                        ?? "Your voice is not recorded. Translations use the phone's voice"
                )
                .font(.footnote)
                .frame(maxWidth: .infinity, alignment: .leading)
                Button(savedVoice == nil ? "Record" : "Change", action: onVoice)
                    .disabled(phase != .idle)
            }
            Button(phase == .recording ? "Stop" : "Record", action: onRecord)
                .buttonStyle(.borderedProminent)
                .disabled(phase == .working)
            Text(status)
                .font(.subheadline.weight(.medium))
            Text(text)
                .font(.title3)
            if let error {
                Text(error)
                    .foregroundStyle(.red)
            }
            Spacer()
        }
        .frame(maxWidth: .infinity, alignment: .leading)
        .padding(24)
    }
}

#Preview {
    TranslateContent(
        phase: .working, status: "Transcribing", text: "this is a fake", error: nil, savedVoice: .seconds(21),
        onRecord: {}, onVoice: {}
    )
}
