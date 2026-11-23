import StoreKit
import SwiftUI

/// Where the profile leads.
enum ProfileDestination: Hashable {
    case name, handle, language, privacy, blocked, advanced, recordVoice
}

/// Who the user is, their voice, privacy, the advanced controls, and the account.
struct ProfileScreen: View {
    let viewModel: ProfileViewModel
    let translate: TranslateViewModel
    let subscriptions: Subscriptions

    @State private var confirmingVoice = false
    @State private var confirmingAccount = false
    @State private var paywall = false
    @State private var managing = false
    @State private var restoring = false

    var body: some View {
        List {
            if let profile = viewModel.profile {
                header(profile)
            }
            if let problem = viewModel.problem {
                Notice(text: problem).listRowBackground(Color.clear)
            }

            Section {
                if viewModel.hasVoice {
                    Button(action: viewModel.playVoiceTapped) {
                        HStack(spacing: 12) {
                            Image(systemName: viewModel.playingVoice ? "pause.fill" : "play.fill")
                                .font(.system(size: 15)).foregroundStyle(Palette.onYou)
                                .frame(width: 44, height: 44).background(Palette.you, in: Circle())
                            VStack(alignment: .leading, spacing: 2) {
                                Text(viewModel.playingVoice ? "Playing your voice" : "Play your voice").font(.pearlLead)
                                Text(voiceDetail).font(.pearlSmall).foregroundStyle(Palette.fg2)
                            }
                        }
                    }
                    NavigationLink("Record again", value: ProfileDestination.recordVoice)
                    Button("Delete my voice", role: .destructive) { confirmingVoice = true }
                        .foregroundStyle(Palette.destructive)
                } else {
                    NavigationLink("Record your voice", value: ProfileDestination.recordVoice)
                }
            } header: {
                header("Your voice")
            } footer: {
                Text("Used only to speak your own words. Without it, your messages use the phone's voice.").font(.pearlSmall)
            }
            .listRowBackground(Palette.bgRaised)

            Section {
                row("Name", value: viewModel.profile?.displayName, to: .name)
                row("ID", value: viewModel.profile.map { "@\($0.handle)" }, to: .handle)
                row("Language", value: viewModel.profile?.lang?.localName, to: .language)
            } header: {
                header("You")
            }
            .listRowBackground(Palette.bgRaised)

            if let standing = subscriptions.standing, standing.required || standing.state != .none {
                Section {
                    HStack {
                        Text("Your plan")
                        Spacer()
                        Text(planLine(standing)).foregroundStyle(Palette.fg2)
                    }
                    if standing.subscribed {
                        Button("Manage subscription") { managing = true }
                    } else {
                        Button(standing.state == .none ? "Start your free week" : "Subscribe") { paywall = true }
                    }
                    Button(restoring ? "Restoring…" : "Restore purchases") {
                        restoring = true
                        Task {
                            // Asks Apple for this Apple account's purchases again, then hands them to the server.
                            try? await AppStore.sync()
                            await subscriptions.sync()
                            restoring = false
                        }
                    }
                    .disabled(restoring)
                    if let problem = subscriptions.problem {
                        Text(problem.words).font(.pearlMeta).foregroundStyle(Palette.fg2)
                    }
                } header: {
                    header("Plan")
                }
                .listRowBackground(Palette.bgRaised)
            }

            Section {
                NavigationLink("What we keep, and why", value: ProfileDestination.privacy)
                NavigationLink("Blocked people", value: ProfileDestination.blocked)
            } header: {
                header("Privacy")
            }
            .listRowBackground(Palette.bgRaised)

            Section {
                NavigationLink(value: ProfileDestination.advanced) {
                    LabeledContent {
                        Text(translate.route.shortLabel)
                    } label: {
                        Text("Speech to text")
                        Text("Where your words are written down")
                    }
                }
                NavigationLink(value: ProfileDestination.advanced) {
                    LabeledContent {
                        EmptyView()
                    } label: {
                        Text("Speech model")
                        Text(translate.modelSummary)
                    }
                }
                LabeledContent("This phone's speed", value: translate.speedSummary)
            } header: {
                header("Advanced")
            }
            .listRowBackground(Palette.bgRaised)

            Section {
                Button("Sign out", action: viewModel.signOutTapped)
                Button("Delete account", role: .destructive) { confirmingAccount = true }
                    .foregroundStyle(Palette.destructive)
            } header: {
                header("Account")
            }
            .listRowBackground(Palette.bgRaised)
        }
        .font(.pearlLead)
        .foregroundStyle(Palette.fg)
        .scrollContentBackground(.hidden)
        .background(Palette.bg)
        .disabled(viewModel.busy)
        .task { await viewModel.refresh() }
        .paywall(isPresented: $paywall, subscriptions: subscriptions)
        .manageSubscriptionsSheet(isPresented: $managing)
        .onChange(of: managing) { _, open in
            if !open { Task { await subscriptions.refresh() } }
        }
        .alert("Delete your voice?", isPresented: $confirmingVoice) {
            Button("Delete my voice", role: .destructive, action: viewModel.deleteVoiceConfirmed)
            Button("Cancel", role: .cancel) {}
        } message: {
            Text("Your recording is deleted from our server and this phone. The people you talk to will hear your messages in the phone's voice. You can record again at any time.")
        }
        .alert("Delete your account?", isPresented: $confirmingAccount) {
            Button("Delete account", role: .destructive, action: viewModel.deleteAccountConfirmed)
            Button("Cancel", role: .cancel) {}
        } message: {
            Text(deleteWarning)
        }
    }

    /// Apple asks apps to say that deleting an account does not cancel a subscription, and how to cancel it.
    private var deleteWarning: String {
        let everything = "Everything goes: your profile, your voice, and everything you said in your rooms. This can't be undone."
        guard subscriptions.standing?.subscribed == true else { return everything }
        return everything + " Your subscription is not cancelled with it: Apple keeps charging until you cancel it, "
            + "under Manage subscription above, or in Settings, under your name, then Subscriptions."
    }

    private var voiceDetail: String {
        let parts = [
            viewModel.voiceLength.map(clock),
            viewModel.keptVoice.map { "recorded \($0.consentedAt.formatted(.dateTime.day().month(.abbreviated)))" },
        ]
        return parts.compactMap { $0 }.joined(separator: " · ")
    }

    private func header(_ profile: Profile) -> some View {
        VStack(spacing: 4) {
            Avatar(name: profile.displayName, size: 76, fill: Palette.you, content: Palette.onYou)
            Text(profile.displayName).font(.pearlTitle).padding(.top, 10)
            Text("@\(profile.handle) · speaks \(profile.lang?.localName ?? "")").font(.pearlMessage).foregroundStyle(Palette.fg2)
        }
        .frame(maxWidth: .infinity)
        .listRowBackground(Color.clear)
    }

    private func header(_ title: String) -> some View {
        Text(title).font(.pearlLabel).foregroundStyle(Palette.fg2).textCase(nil)
    }

    private func row(_ title: String, value: String?, to destination: ProfileDestination) -> some View {
        NavigationLink(value: destination) {
            LabeledContent(title, value: value ?? "")
        }
    }
}

private extension TranscriptionRoute {
    var shortLabel: String {
        switch self {
        case .automatic: "Automatic"
        case .onDevice: "This device"
        case .cloud: "Server"
        }
    }
}

private extension TranslateViewModel {
    var modelSummary: String {
        switch model {
        case .ready(let file):
            let bytes = (try? file.resourceValues(forKeys: [.fileSizeKey]).fileSize) ?? 0
            return "Ready · \(bytes / (1024 * 1024)) MB on this phone"
        case .downloading, .verifying: return "Downloading"
        case .missing, .failed: return "Not on this phone"
        }
    }

    var speedSummary: String {
        guard deviceSpeed != nil else { return "Not measured" }
        return deviceTooSlow ? "Slow" : "Fast"
    }
}
