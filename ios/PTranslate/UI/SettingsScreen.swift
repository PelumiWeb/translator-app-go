import SwiftUI

/// Settings: the user's voice, and under "Advanced" the controls that left the interpreter for being about how it
/// works rather than what it does.
///
/// - Parameters:
///   - savedVoice: length of the user's saved voice recording, or nil if they have not made one.
///   - onVoiceTapped: opens the screen where it is recorded, replaced or deleted.
struct SettingsScreen: View {
    let viewModel: TranslateViewModel
    let savedVoice: Duration?
    let onVoiceTapped: () -> Void

    @Environment(\.dismiss) private var dismiss

    var body: some View {
        NavigationStack {
            List {
                Section {
                    Text(
                        savedVoice.map { "Recorded, \($0.components.seconds) seconds. Translations are spoken in your voice." }
                            ?? "Not recorded. Translations are spoken in the phone's voice."
                    )
                    .font(.pearlMessage)
                    Button(savedVoice == nil ? "Record your voice" : "Record again or delete", action: onVoiceTapped)
                        .font(.pearlMessage)
                } header: {
                    sectionHeader("Your voice")
                }
                .listRowBackground(Palette.bgRaised)

                Section {
                    NavigationLink {
                        AdvancedSettings(viewModel: viewModel)
                    } label: {
                        Text("Advanced")
                            .font(.pearlMessage)
                    }
                }
                .listRowBackground(Palette.bgRaised)

                Section {
                    Text("Version \(version). Set in IBM Plex Sans, under the SIL Open Font License.")
                        .font(.pearlSmall)
                        .foregroundStyle(Palette.fg2)
                } header: {
                    sectionHeader("About")
                }
                .listRowBackground(Palette.bgRaised)
            }
            .foregroundStyle(Palette.fg)
            .scrollContentBackground(.hidden)
            .background(Palette.bg)
            .navigationTitle("Settings")
            .toolbar {
                ToolbarItem(placement: .confirmationAction) {
                    Button("Done") { dismiss() }
                }
            }
        }
    }

    private var version: String {
        Bundle.main.object(forInfoDictionaryKey: "CFBundleShortVersionString") as? String ?? "?"
    }
}

private func sectionHeader(_ title: String) -> some View {
    Text(title)
        .font(.pearlLabel)
        .foregroundStyle(Palette.them)
        .textCase(nil)
}

/// How speech is turned into text, the speech model on the phone, its speed, and the last result's details.
private struct AdvancedSettings: View {
    let viewModel: TranslateViewModel

    var body: some View {
        List {
            Section {
                Picker("Turn speech into text", selection: Binding(get: { viewModel.route }, set: { viewModel.routeSelected($0) })) {
                    ForEach(TranscriptionRoute.allCases, id: \.self) { route in
                        Text(route.label).tag(route)
                    }
                }
                .pickerStyle(.inline)
                .labelsHidden()
                .font(.pearlMessage)
            } header: {
                sectionHeader("Turn speech into text")
            } footer: {
                Text(viewModel.route.description)
                    .font(.pearlSmall)
                    .foregroundStyle(Palette.fg2)
            }
            .listRowBackground(Palette.bgRaised)

            Section {
                ModelStatus(model: viewModel.model, onDownload: viewModel.downloadModelTapped)
            } header: {
                sectionHeader("Speech model on this phone")
            }
            .listRowBackground(Palette.bgRaised)

            if case .ready = viewModel.model {
                Section {
                    Text(speedDescription)
                        .font(.pearlMessage)
                    if !viewModel.measuringSpeed {
                        Button("Measure again", action: viewModel.measureSpeedTapped)
                            .font(.pearlMessage)
                    }
                } header: {
                    sectionHeader("This phone's speed")
                }
                .listRowBackground(Palette.bgRaised)
            }

            if viewModel.details != nil || viewModel.routing != nil {
                Section {
                    if let details = viewModel.details {
                        Text(details).font(.pearlSmall)
                    }
                    if let routing = viewModel.routing {
                        Text(routing).font(.pearlSmall)
                    }
                } header: {
                    sectionHeader("Last result")
                }
                .listRowBackground(Palette.bgRaised)
            }
        }
        .foregroundStyle(Palette.fg)
        .scrollContentBackground(.hidden)
        .background(Palette.bg)
        .navigationTitle("Advanced")
        .navigationBarTitleDisplayMode(.inline)
    }

    private var speedDescription: String {
        if viewModel.measuringSpeed { return "Measuring…" }
        guard let speed = viewModel.deviceSpeed else { return "Not measured yet" }
        let figure = String(format: "%.2fx real time", speed)
        return viewModel.deviceTooSlow ? "\(figure). Too slow, so the server is used" : figure
    }
}

/// Where the on-device speech model stands, and the download.
private struct ModelStatus: View {
    let model: ModelState
    let onDownload: () -> Void

    var body: some View {
        switch model {
        case .missing:
            Text("Not on this phone yet. It is 57 MB; Wi-Fi is best.")
                .font(.pearlMessage)
            Button("Download speech model", action: onDownload)
                .buttonStyle(PrimaryButtonStyle())
        case .downloading(let bytes, let total):
            Text("Downloading: \(bytes / 1_048_576) of \(total / 1_048_576) MB")
                .font(.pearlMessage)
            ProgressView(value: total > 0 ? Double(bytes) / Double(total) : 0)
                .tint(Palette.them)
        case .verifying:
            Text("Checking the download")
                .font(.pearlMessage)
            ProgressView()
        case .failed(let reason):
            Text(reason)
                .font(.pearlMessage)
            // The partial download is kept, so this continues, not restarts.
            Button("Try again", action: onDownload)
                .buttonStyle(PrimaryButtonStyle())
        case .ready:
            Text("Ready. Speech can be turned into text on this phone, offline.")
                .font(.pearlMessage)
        }
    }
}

private extension TranscriptionRoute {
    var label: String {
        switch self {
        case .automatic: "Automatic"
        case .onDevice: "This phone only"
        case .cloud: "The server"
        }
    }

    // The automatic route can upload audio without asking each time, so it says so.
    var description: String {
        switch self {
        case .automatic: "On this phone when it can. Otherwise your recording is sent to the server."
        case .onDevice: "On this phone only. Nothing is uploaded."
        case .cloud: "Every recording is sent to the server."
        }
    }
}
