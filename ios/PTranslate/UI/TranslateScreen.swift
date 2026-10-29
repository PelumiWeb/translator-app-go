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
            details: viewModel.details,
            error: viewModel.error,
            // A closure, not the method itself: passing `viewModel.routeSelected` here crashes the Swift 6.3
            // compiler while it converts the main-actor method into the binding's setter.
            route: Binding(get: { viewModel.route }, set: { viewModel.routeSelected($0) }),
            model: viewModel.model,
            savedVoice: savedVoice,
            onRecord: viewModel.recordTapped,
            onDownloadModel: viewModel.downloadModelTapped,
            onVoice: onVoiceTapped
        )
    }
}

/// Stateless, so it can be previewed without a view model.
private struct TranslateContent: View {
    let phase: RecordingPhase
    let status: String
    let text: String
    let details: String?
    let error: String?
    @Binding var route: TranscriptionRoute
    let model: ModelState
    let savedVoice: Duration?
    let onRecord: () -> Void
    let onDownloadModel: () -> Void
    let onVoice: () -> Void

    var body: some View {
        ScrollView {
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

                VStack(alignment: .leading, spacing: 4) {
                    Picker("Transcribe on", selection: $route) {
                        ForEach(TranscriptionRoute.allCases, id: \.self) { route in
                            Text(route.label).tag(route)
                        }
                    }
                    .pickerStyle(.segmented)
                    .disabled(phase != .idle)
                    Text(route.description)
                        .font(.footnote)
                }

                // The model only matters when this device may be used.
                if route != .cloud {
                    ModelStatus(model: model, onDownload: onDownloadModel)
                }

                Button(phase == .recording ? "Stop" : "Record", action: onRecord)
                    .buttonStyle(.borderedProminent)
                    .disabled(phase == .working)
                Text(status)
                    .font(.subheadline.weight(.medium))
                Text(text)
                    .font(.title3)
                if let details {
                    Text(details)
                        .font(.footnote)
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
}

private struct ModelStatus: View {
    let model: ModelState
    let onDownload: () -> Void

    var body: some View {
        VStack(alignment: .leading, spacing: 8) {
            switch model {
            case .missing:
                Text("The speech model is not on this device yet.")
                Button("Download speech model", action: onDownload)
                    .buttonStyle(.bordered)
            case .downloading(let bytes, let total):
                Text("Downloading speech model: \(bytes.megabytes) of \(total.megabytes) MB")
                ProgressView(value: total > 0 ? Double(bytes) / Double(total) : 0)
            case .verifying:
                Text("Checking the download")
                ProgressView()
            case .failed(let reason):
                Text(reason)
                    .foregroundStyle(.red)
                // The partial download is kept, so this continues, not restarts.
                Button("Try again", action: onDownload)
                    .buttonStyle(.bordered)
            case .ready:
                Text("Speech model ready")
                    .font(.footnote)
            }
        }
    }
}

private extension Int64 {
    var megabytes: Int64 { self / (1024 * 1024) }
}

private extension TranscriptionRoute {
    var label: String {
        switch self {
        case .onDevice: "This device"
        case .cloud: "Server"
        }
    }

    var description: String {
        switch self {
        case .onDevice: "Transcribes on this device only. Nothing is uploaded."
        case .cloud: "Every recording is sent to the server."
        }
    }
}

#Preview {
    TranslateContent(
        phase: .idle, status: "Done", text: "this is a fake",
        details: "On device, 1.2 s for 4.0 s of audio (0.30x real time), confidence 0.87", error: nil,
        route: .constant(.onDevice), model: .downloading(bytes: 20_000_000, total: 59_707_625),
        savedVoice: .seconds(21), onRecord: {}, onDownloadModel: {}, onVoice: {}
    )
}
