import AuthenticationServices
import SwiftUI

/// The steps of signing up shown with a progress bar. The speech model step is not designed yet.
let onboardingSteps = 4

/// The first screen: the promise, and a way in.
struct WelcomeScreen: View {
    let onGetStarted: () -> Void

    var body: some View {
        VStack(alignment: .leading, spacing: 0) {
            Text("timbre").font(.custom("IBMPlexSans-Medm", size: 18, relativeTo: .headline)).foregroundStyle(Palette.fg)
            Spacer()
            TwoVoices().frame(maxWidth: .infinity)
            Spacer()
            Text("Talk to anyone. Be heard in your own voice.")
                .font(.pearlHero).foregroundStyle(Palette.fg)
                .accessibilityAddTraits(.isHeader)
            Text("You speak your language. They hear theirs, in your voice.")
                .font(.pearlLead).foregroundStyle(Palette.fg2)
                .padding(.top, 12).padding(.bottom, 28)
            Button("Get started", action: onGetStarted).buttonStyle(PrimaryButtonStyle())
        }
        .padding(.horizontal, 28).padding(.vertical, 24)
        .frame(maxWidth: .infinity, maxHeight: .infinity, alignment: .leading)
        .background(Palette.bg)
    }
}

/// Sign in with Apple, with Apple's own button and sheet.
struct SignInScreen: View {
    let viewModel: OnboardingViewModel
    let onBack: () -> Void

    /// Made fresh for each attempt: its hash goes to Apple, the raw value to the server.
    @State private var nonce = Nonce.random()

    var body: some View {
        VStack(alignment: .leading, spacing: 0) {
            Button(action: onBack) {
                Image(systemName: "chevron.left").font(.system(size: 22)).foregroundStyle(Palette.fg).frame(width: 44, height: 44)
            }
            .accessibilityLabel("Back")
            Text("Sign in to keep your conversations.")
                .font(.pearlScreenTitle).foregroundStyle(Palette.fg)
                .padding(.top, 24)
                .accessibilityAddTraits(.isHeader)
            Text("This is only to find your account on any phone. We never post anything.")
                .font(.pearlLead).foregroundStyle(Palette.fg2).padding(.top, 12)
            Spacer()
            if let problem = viewModel.signInProblem {
                Notice(text: problem).padding(.bottom, 16)
            }
            SignInWithAppleButton(.signIn) { request in
                nonce = Nonce.random()
                request.requestedScopes = [.fullName, .email]
                request.nonce = nonce.hashed
            } onCompletion: { result in
                completed(result)
            }
            .signInWithAppleButtonStyle(.white)
            .frame(height: 52)
            .clipShape(Capsule())
            .disabled(viewModel.signingIn)
            .overlay { if viewModel.signingIn { ProgressView().tint(.black) } }
            Text("Email sign-in is coming soon.")
                .font(.pearlBody).foregroundStyle(Palette.fg3)
                .frame(maxWidth: .infinity).padding(.top, 18)
            Text("By continuing you agree to the Terms and Privacy Policy.")
                .font(.pearlSmall).foregroundStyle(Palette.fg2)
                .multilineTextAlignment(.center)
                .frame(maxWidth: .infinity).padding(.top, 20)
        }
        .padding(.horizontal, 28).padding(.vertical, 16)
        .background(Palette.bg)
    }

    private func completed(_ result: Result<ASAuthorization, any Error>) {
        switch result {
        case .success(let authorization):
            guard
                let credential = authorization.credential as? ASAuthorizationAppleIDCredential,
                let data = credential.identityToken,
                let token = String(data: data, encoding: .utf8)
            else {
                viewModel.signInFailed(cancelled: false)
                return
            }
            // Apple gives the name at the first sign-in only, and never puts it in the token.
            let name = credential.fullName.map { $0.formatted() }.flatMap { $0.isEmpty ? nil : $0 }
            let nonce = nonce
            Task { await viewModel.signIn(identityToken: token, nonce: nonce, name: name) }
        case .failure(let error):
            viewModel.signInFailed(cancelled: (error as? ASAuthorizationError)?.code == .canceled)
        }
    }
}

/// The frame of steps 1 to 3: header, question, content, and the button kept above the keyboard. With no `step`, as
/// when editing from the profile, the header is a back button alone.
private struct StepFrame<Content: View>: View {
    let step: Int?
    let title: String
    var subtitle: String?
    let button: String
    let buttonEnabled: Bool
    let saving: Bool
    let problem: String?
    let onBack: () -> Void
    let onContinue: () -> Void
    @ViewBuilder let content: Content

    var body: some View {
        VStack(alignment: .leading, spacing: 0) {
            if let step {
                OnboardingHeader(step: step, total: onboardingSteps, onBack: onBack)
            } else {
                Button(action: onBack) {
                    Image(systemName: "chevron.left").font(.system(size: 22)).foregroundStyle(Palette.fg).frame(width: 44, height: 44)
                }
                .accessibilityLabel("Back")
            }
            Text(title).font(.pearlScreenTitle).foregroundStyle(Palette.fg)
                .padding(.top, 8)
                .accessibilityAddTraits(.isHeader)
            if let subtitle {
                Text(subtitle).font(.pearlLead).foregroundStyle(Palette.fg2).padding(.top, 8)
            }
            content.padding(.top, 20)
            if let problem {
                Notice(text: problem).padding(.top, 16)
            }
            Spacer(minLength: 0)
        }
        .padding(.horizontal, 20)
        .frame(maxWidth: .infinity, maxHeight: .infinity, alignment: .topLeading)
        .safeAreaInset(edge: .bottom) {
            Button(saving ? "Saving…" : button, action: onContinue)
                .buttonStyle(PrimaryButtonStyle())
                .disabled(!buttonEnabled || saving)
                .padding(.horizontal, 16).padding(.bottom, 8)
        }
        .background(Palette.bg)
    }
}

struct NameScreen: View {
    @Bindable var viewModel: OnboardingViewModel
    var step: Int? = 1
    var button = "Continue"
    let onBack: () -> Void

    var body: some View {
        StepFrame(
            step: step, title: "What should people call you?", button: button,
            buttonEnabled: !viewModel.name.trimmingCharacters(in: .whitespaces).isEmpty,
            saving: viewModel.saving, problem: viewModel.problem, onBack: onBack, onContinue: viewModel.nameContinue
        ) {
            PearlTextField(label: "Your name", text: $viewModel.name)
                .textContentType(.name)
                .textInputAutocapitalization(.words)
                .submitLabel(.done)
                .onSubmit(viewModel.nameContinue)
            Text("Shown to the people you talk to.").font(.pearlBody).foregroundStyle(Palette.fg2).padding(.top, 14)
        }
    }
}

struct HandleScreen: View {
    let viewModel: OnboardingViewModel
    var step: Int? = 2
    var button = "Continue"
    let onBack: () -> Void

    var body: some View {
        StepFrame(
            step: step, title: "Pick your ID.", subtitle: "This is how people find you.", button: button,
            buttonEnabled: viewModel.handleState == .available,
            saving: viewModel.saving, problem: viewModel.problem, onBack: onBack, onContinue: viewModel.handleContinue
        ) {
            PearlTextField(label: "Your ID", text: Binding(get: { viewModel.handle }, set: { viewModel.handleChanged($0) }), prefix: "@") {
                switch viewModel.handleState {
                case .checking: ProgressView().tint(Palette.fg2)
                case .available: Image(systemName: "checkmark").foregroundStyle(Palette.fg)
                default: EmptyView()
                }
            }
            .textInputAutocapitalization(.never)
            .autocorrectionDisabled()
            .keyboardType(.asciiCapable)
            .submitLabel(.done)
            .onSubmit(viewModel.handleContinue)

            Group {
                switch viewModel.handleState {
                case .available:
                    Text("Available").font(.pearlBody.weight(.medium)).foregroundStyle(Palette.fg)
                case .invalid(let rule), .unchecked(let rule):
                    Text(rule).font(.pearlBody).foregroundStyle(Palette.fg)
                case .taken(let suggestions):
                    VStack(alignment: .leading, spacing: 10) {
                        Text("@\(viewModel.handle) is taken. These are free:").font(.pearlBody).foregroundStyle(Palette.fg2)
                        HStack(spacing: 8) {
                            ForEach(suggestions, id: \.self) { suggestion in
                                SuggestionChip(text: "@\(suggestion)") { viewModel.handleChanged(suggestion) }
                            }
                        }
                    }
                case .checking, .empty:
                    EmptyView()
                }
            }
            .padding(.top, 12)
            Text("Letters, numbers, dots and underscores.").font(.pearlBody).foregroundStyle(Palette.fg2).padding(.top, 24)
        }
    }
}

/// Which language the person speaks: theirs, suggested from the phone, or any of Whisper's 99.
struct LanguageScreen: View {
    @Bindable var viewModel: OnboardingViewModel
    let phoneLanguage: Language?
    var step: Int? = 3
    var button: String?
    let onBack: () -> Void

    @State private var query = ""
    private let all = WhisperLanguages.all.sorted { $0.localName < $1.localName }

    var body: some View {
        StepFrame(
            step: step, title: "Which language do you speak?",
            button: button ?? viewModel.language.map { "Continue in \($0.ownName)" } ?? "Continue",
            buttonEnabled: viewModel.language != nil,
            saving: viewModel.saving, problem: viewModel.problem, onBack: onBack, onContinue: viewModel.languageContinue
        ) {
            PearlTextField(label: "Search languages", text: $query, placeholder: "Search \(all.count) languages", compact: true, systemImage: "magnifyingglass")
            List {
                if let phoneLanguage, query.isEmpty {
                    Section("From your phone") {
                        row(phoneLanguage, detail: Locale.current.region.flatMap { Locale.current.localizedString(forRegionCode: $0.identifier) })
                    }
                }
                Section(query.isEmpty && phoneLanguage != nil ? "All languages" : "") {
                    ForEach(shown, id: \.tag) { row($0, detail: nil) }
                }
            }
            .listStyle(.insetGrouped)
            .scrollContentBackground(.hidden)
            .contentMargins(.horizontal, 0, for: .scrollContent)
        }
    }

    private var shown: [Language] {
        guard !query.isEmpty else { return all }
        return all.filter { $0.ownName.localizedCaseInsensitiveContains(query) || $0.localName.localizedCaseInsensitiveContains(query) }
    }

    private func row(_ language: Language, detail: String?) -> some View {
        Button {
            viewModel.language = language
        } label: {
            HStack {
                VStack(alignment: .leading, spacing: 2) {
                    Text(language.ownName).font(.pearlMessage.leading(.loose)).foregroundStyle(Palette.fg)
                    if let detail { Text(detail).font(.pearlSmall).foregroundStyle(Palette.fg2) }
                }
                Spacer()
                if viewModel.language == language {
                    Image(systemName: "checkmark").foregroundStyle(Palette.fg).accessibilityLabel("Selected")
                } else if detail == nil, language.localName != language.ownName {
                    Text(language.localName).font(.pearlSmall).foregroundStyle(Palette.fg2)
                }
            }
            .contentShape(Rectangle())
        }
        .buttonStyle(.plain)
        .listRowBackground(Palette.bgRaised)
    }
}
