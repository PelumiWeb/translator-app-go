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
            speaking: viewModel.speaking,
            text: viewModel.text,
            translation: viewModel.translation,
            details: viewModel.details,
            routing: viewModel.routing,
            error: viewModel.error,
            // Closures, not the methods themselves: passing `viewModel.routeSelected` here crashes the Swift 6.3
            // compiler while it converts the main-actor method into the binding's setter.
            route: Binding(get: { viewModel.route }, set: { viewModel.routeSelected($0) }),
            source: Binding(get: { viewModel.source }, set: { viewModel.sourceSelected($0) }),
            target: Binding(get: { viewModel.target }, set: { viewModel.targetSelected($0) }),
            sourceLanguages: viewModel.sourceLanguages,
            targetLanguages: viewModel.targetLanguages,
            model: viewModel.model,
            speed: DeviceSpeedStatus.State(
                realTimeFactor: viewModel.deviceSpeed,
                measuring: viewModel.measuringSpeed,
                tooSlow: viewModel.deviceTooSlow
            ),
            savedVoice: savedVoice,
            onRecord: viewModel.recordTapped,
            onDownloadModel: viewModel.downloadModelTapped,
            onMeasureSpeed: viewModel.measureSpeedTapped,
            onVoice: onVoiceTapped
        )
        .task { await viewModel.loadLanguages() }
    }
}

/// Stateless, so it can be previewed without a view model.
private struct TranslateContent: View {
    let phase: RecordingPhase
    let status: String
    let speaking: Bool
    let text: String
    let translation: String
    let details: String?
    let routing: String?
    let error: String?
    @Binding var route: TranscriptionRoute
    @Binding var source: Language
    @Binding var target: Language
    let sourceLanguages: [Language]
    let targetLanguages: [Language]
    let model: ModelState
    let speed: DeviceSpeedStatus.State
    let savedVoice: Duration?
    let onRecord: () -> Void
    let onDownloadModel: () -> Void
    let onMeasureSpeed: () -> Void
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

                HStack(alignment: .top, spacing: 16) {
                    LanguagePicker(label: "From", selection: $source, options: sourceLanguages)
                    LanguagePicker(label: "To", selection: $target, options: targetLanguages)
                }
                .disabled(phase != .idle)

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

                // The model and its speed only matter when this device may be used.
                if route != .cloud {
                    ModelStatus(model: model, onDownload: onDownloadModel)
                    if case .ready = model {
                        DeviceSpeedStatus(
                            state: speed,
                            automatic: route == .automatic,
                            onMeasure: onMeasureSpeed
                        )
                        .disabled(phase != .idle)
                    }
                }

                Button(phase == .recording ? "Stop" : "Record", action: onRecord)
                    .buttonStyle(.borderedProminent)
                    .disabled(phase == .working)
                HStack(spacing: 8) {
                    // While something is being worked on and nothing can be heard yet.
                    if phase == .working && !speaking {
                        ProgressView()
                            .controlSize(.small)
                    }
                    Text(status)
                        .font(.subheadline.weight(.medium))
                }
                Text(text)
                    .font(.title3)
                if !translation.isEmpty {
                    Text(translation)
                        .font(.title2)
                }
                if let details {
                    Text(details)
                        .font(.footnote)
                }
                if let routing {
                    Text(routing)
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

private struct LanguagePicker: View {
    let label: String
    @Binding var selection: Language
    let options: [Language]

    var body: some View {
        VStack(alignment: .leading, spacing: 2) {
            Text(label)
                .font(.footnote)
            Picker(label, selection: $selection) {
                // Until the list has loaded, the current choice is its only entry, so the picker has something to
                // show.
                ForEach(sorted(options.isEmpty ? [selection] : options), id: \.self) { language in
                    Text(language.displayName).tag(language)
                }
            }
            .pickerStyle(.menu)
        }
        .frame(maxWidth: .infinity, alignment: .leading)
    }

    /// By the name the user reads, which depends on the phone's language.
    private func sorted(_ languages: [Language]) -> [Language] {
        languages.sorted { $0.displayName.localizedStandardCompare($1.displayName) == .orderedAscending }
    }
}

/// How fast this device runs the model, and what that means for routing.
private struct DeviceSpeedStatus: View {
    struct State {
        var realTimeFactor: Double?
        var measuring: Bool
        var tooSlow: Bool
    }

    let state: State
    let automatic: Bool
    let onMeasure: () -> Void

    var body: some View {
        HStack(spacing: 8) {
            Text(description)
                .font(.footnote)
                .frame(maxWidth: .infinity, alignment: .leading)
            Button("Measure again", action: onMeasure)
                .disabled(state.measuring)
        }
    }

    private var description: String {
        if state.measuring {
            return "Measuring this device's speed"
        }
        guard let speed = state.realTimeFactor else {
            return "This device's speed has not been measured"
        }
        let figure = String(format: "Speed: %.2fx real time", speed)
        return state.tooSlow && automatic ? "\(figure). Too slow, so the server is used" : figure
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

private extension Language {
    /// Such as "Spanish", or "Espagnol" on a phone set to French.
    var displayName: String {
        let name = Locale.current.localizedString(forLanguageCode: tag) ?? tag
        return name.prefix(1).uppercased() + name.dropFirst()
    }
}

private extension Int64 {
    var megabytes: Int64 { self / (1024 * 1024) }
}

private extension TranscriptionRoute {
    var label: String {
        switch self {
        case .automatic: "Automatic"
        case .onDevice: "This device"
        case .cloud: "Server"
        }
    }

    // The automatic route can upload audio without asking each time, so it says so.
    var description: String {
        switch self {
        case .automatic: "Transcribes on this device when it can. Otherwise your recording is sent to the server."
        case .onDevice: "Transcribes on this device only. Nothing is uploaded."
        case .cloud: "Every recording is sent to the server."
        }
    }
}

#Preview {
    TranslateContent(
        phase: .idle, status: "Done", speaking: false, text: "this is a fake", translation: "esto es falso",
        details: "On device, 1.2 s for 4.0 s of audio (0.30x real time), confidence 0.87",
        routing: "Sent to the server: this device was not confident in its own result", error: nil,
        route: .constant(.automatic), source: .constant(.english), target: .constant(.spanish),
        sourceLanguages: [.english, .spanish], targetLanguages: [.english, .spanish],
        model: .ready(URL(filePath: "/model.bin")),
        speed: DeviceSpeedStatus.State(realTimeFactor: 0.31, measuring: false, tooSlow: false),
        savedVoice: .seconds(21), onRecord: {}, onDownloadModel: {}, onMeasureSpeed: {}, onVoice: {}
    )
}
