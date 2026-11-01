import AVFoundation
import SwiftUI

/// Chooses what the person sees from where they are: not signed in (welcome, then sign in), part way through
/// signing up (the step they reached), or in the app. The account is the one source of truth, so signing out, or a
/// session the server ended, brings them back to the welcome from anywhere.
struct AppRoot: View {
    let container: AppContainer
    let translate: TranslateViewModel
    let voiceSetup: VoiceSetupViewModel
    /// An ID from a shinar://add link, offered once signed up.
    @Binding var addHandle: String?

    var body: some View {
        let account = container.account
        Group {
            if account.session != nil && account.step == .done {
                MainTabs(container: container, translate: translate, voiceSetup: voiceSetup, addHandle: $addHandle)
            } else {
                // A new view, and so a new view model with fresh fields, for each person who signs in.
                OnboardingFlow(container: container, voiceSetup: voiceSetup)
                    .id(account.session?.profile.id ?? "signed out")
            }
        }
        .tint(Palette.them)
        .onChange(of: account.session?.token) { old, new in
            // Signing out, or deleting the account, leaves nothing of the person on this phone.
            if old != nil && new == nil {
                Task { await container.forgetLocalVoice() }
            }
        }
    }
}

/// Welcome, sign in, then the steps of signing up.
private struct OnboardingFlow: View {
    let container: AppContainer
    let voiceSetup: VoiceSetupViewModel

    @State private var viewModel: OnboardingViewModel
    @State private var signingIn = false

    init(container: AppContainer, voiceSetup: VoiceSetupViewModel) {
        self.container = container
        self.voiceSetup = voiceSetup
        _viewModel = State(initialValue: OnboardingViewModel(account: container.account, voice: container.voice, phoneLanguage: container.phoneLanguage))
    }

    var body: some View {
        let account = container.account
        let back = { viewModel.back(from: account.step) }
        Group {
            if account.session == nil {
                if signingIn {
                    SignInScreen(viewModel: viewModel, onBack: { signingIn = false })
                } else {
                    WelcomeScreen(onGetStarted: { signingIn = true })
                }
            } else {
                switch account.step {
                case .name: NameScreen(viewModel: viewModel, onBack: back)
                case .handle: HandleScreen(viewModel: viewModel, onBack: back)
                case .language: LanguageScreen(viewModel: viewModel, phoneLanguage: container.phoneLanguage, onBack: back)
                case .voice, .done:
                    VoiceSetupScreen(viewModel: voiceSetup, step: 4, onBack: back) { account.move(to: .done) }
                }
            }
        }
        .animation(.default, value: account.step)
    }
}

/// The signed-up app: Rooms, the Interpreter and Profile in the system's tab bar, as the design asks on iPhone.
private struct MainTabs: View {
    let container: AppContainer
    let translate: TranslateViewModel
    let voiceSetup: VoiceSetupViewModel
    @Binding var addHandle: String?

    private enum TabID { case rooms, interpreter, profile }

    @State private var tab = TabID.rooms
    @State private var rooms: RoomsViewModel
    @State private var profile: ProfileViewModel
    @State private var roomsPath: [String] = []
    @State private var profilePath: [ProfileDestination] = []

    init(container: AppContainer, translate: TranslateViewModel, voiceSetup: VoiceSetupViewModel, addHandle: Binding<String?>) {
        self.container = container
        self.translate = translate
        self.voiceSetup = voiceSetup
        _addHandle = addHandle
        _rooms = State(initialValue: RoomsViewModel(client: container.rooms, account: container.account, feed: container.roomFeed))
        _profile = State(initialValue: ProfileViewModel(account: container.account, voice: container.voice, player: container.player))
    }

    var body: some View {
        TabView(selection: $tab) {
            Tab("Rooms", systemImage: "bubble.left", value: TabID.rooms) {
                NavigationStack(path: $roomsPath) {
                    RoomsScreen(viewModel: rooms, addHandle: $addHandle) { roomsPath.append($0.id) }
                        .navigationDestination(for: String.self) { roomID in
                            RoomDestination(container: container, roomID: roomID) {
                                tab = .profile
                                profilePath = [.recordVoice]
                            }
                        }
                }
            }
            Tab("Interpreter", systemImage: "arrow.left.arrow.right", value: TabID.interpreter) {
                TranslateScreen(viewModel: translate, hasOwnVoice: container.voice.state.duration != nil)
            }
            Tab("Profile", systemImage: "person", value: TabID.profile) {
                NavigationStack(path: $profilePath) {
                    ProfileScreen(viewModel: profile, translate: translate)
                        .navigationDestination(for: ProfileDestination.self) { destination in
                            profileDestination(destination)
                        }
                }
            }
        }
        .onChange(of: addHandle) { _, handle in
            if handle != nil {
                tab = .rooms
                roomsPath = []
            }
        }
    }

    @ViewBuilder
    private func profileDestination(_ destination: ProfileDestination) -> some View {
        switch destination {
        case .name, .handle, .language:
            EditProfileScreen(container: container, field: destination) { profilePath.removeLast() }
        case .privacy:
            PrivacyScreen()
        case .advanced:
            AdvancedSettings(viewModel: translate)
        case .recordVoice:
            VoiceSetupScreen(viewModel: voiceSetup) {
                profilePath.removeAll { $0 == .recordVoice }
                Task { await profile.refresh() }
            }
            .toolbar(.hidden, for: .tabBar)
        }
    }
}

/// A room, with its own view model for as long as it is open.
private struct RoomDestination: View {
    let onRecordVoice: () -> Void
    @State private var viewModel: RoomViewModel

    init(container: AppContainer, roomID: String, onRecordVoice: @escaping () -> Void) {
        self.onRecordVoice = onRecordVoice
        _viewModel = State(
            initialValue: RoomViewModel(
                roomID: roomID,
                client: container.rooms,
                account: container.account,
                feed: container.roomFeed,
                recorder: container.recorder,
                transcriber: container.transcriber,
                player: container.messagePlayer,
                requestMicrophone: { await AVAudioApplication.requestRecordPermission() }
            )
        )
    }

    var body: some View {
        RoomScreen(viewModel: viewModel, onRecordVoice: onRecordVoice)
    }
}

/// The name, ID or language, changed with the same screens as signing up.
private struct EditProfileScreen: View {
    let field: ProfileDestination
    let onSaved: () -> Void
    let phoneLanguage: Language?
    @State private var viewModel: OnboardingViewModel

    init(container: AppContainer, field: ProfileDestination, onSaved: @escaping () -> Void) {
        self.field = field
        self.onSaved = onSaved
        phoneLanguage = container.phoneLanguage
        _viewModel = State(
            initialValue: OnboardingViewModel(account: container.account, voice: container.voice, phoneLanguage: container.phoneLanguage, editing: true)
        )
    }

    var body: some View {
        Group {
            switch field {
            case .name: NameScreen(viewModel: viewModel, step: nil, button: "Save", onBack: onSaved)
            case .handle: HandleScreen(viewModel: viewModel, step: nil, button: "Save", onBack: onSaved)
            default: LanguageScreen(viewModel: viewModel, phoneLanguage: phoneLanguage, step: nil, button: "Save", onBack: onSaved)
            }
        }
        .navigationBarBackButtonHidden()
        .toolbar(.hidden, for: .tabBar)
        .onChange(of: viewModel.saved) { _, saved in if saved { onSaved() } }
    }
}
