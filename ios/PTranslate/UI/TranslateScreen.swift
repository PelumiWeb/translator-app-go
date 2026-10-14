import SwiftUI

struct TranslateScreen: View {
    let viewModel: TranslateViewModel

    var body: some View {
        TranslateContent(
            phase: viewModel.phase,
            status: viewModel.status,
            text: viewModel.text,
            error: viewModel.error,
            onRecord: viewModel.recordTapped
        )
    }
}

/// Stateless, so it can be previewed without a view model.
private struct TranslateContent: View {
    let phase: RecordingPhase
    let status: String
    let text: String
    let error: String?
    let onRecord: () -> Void

    var body: some View {
        VStack(alignment: .leading, spacing: 16) {
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
    TranslateContent(phase: .working, status: "Transcribing", text: "this is a fake", error: nil, onRecord: {})
}
