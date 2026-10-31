import Foundation
import Observation

/// Signing in and the first steps of signing up: the name, the ID, the language. Each step is saved to the server
/// before moving on, so a person who stops half way picks up where they left off. With `editing`, the same steps
/// change the profile later: saving sets `saved` for the screen to close instead.
@MainActor
@Observable
final class OnboardingViewModel {
    /// Where the ID being typed stands.
    enum HandleState: Equatable {
        case empty
        /// It breaks a rule; the text says which.
        case invalid(String)
        case checking
        case available
        case taken(suggestions: [String])
        /// The server could not be asked.
        case unchecked(String)
    }

    static let maxName = 50

    var signingIn = false
    var signInProblem: String?
    var name: String
    private(set) var handle: String
    private(set) var handleState: HandleState = .empty
    var language: Language?
    private(set) var saving = false
    private(set) var problem: String?
    private(set) var saved = false

    @ObservationIgnored private let account: Account
    @ObservationIgnored private let voice: any VoiceSampleStore
    @ObservationIgnored private let editing: Bool
    @ObservationIgnored private let checkDelay: Duration
    /// The check of the ID being typed; replaced, and so cancelled, by the next keystroke.
    @ObservationIgnored private var check: Task<Void, Never>?
    /// Saving a step, or signing out from the first; owned here so it ends with the screen.
    @ObservationIgnored private var work: Task<Void, Never>?

    init(account: Account, voice: any VoiceSampleStore, phoneLanguage: Language?, editing: Bool = false, checkDelay: Duration = .milliseconds(400)) {
        self.account = account
        self.voice = voice
        self.editing = editing
        self.checkDelay = checkDelay
        let profile = account.profile
        name = profile?.displayName ?? ""
        handle = profile.flatMap { $0.handle.isEmpty ? nil : $0.handle } ?? Self.suggestedHandle(from: profile?.displayName ?? "")
        language = profile?.lang ?? phoneLanguage
        if !handle.isEmpty { handleChanged(handle) }
    }

    isolated deinit {
        check?.cancel()
        work?.cancel()
    }

    /// Signs in with the identity token Apple gave the phone, for the nonce whose hash Apple was given.
    func signIn(identityToken: String, nonce: Nonce, name: String?) async {
        signingIn = true
        signInProblem = nil
        defer { signingIn = false }
        do {
            try await account.signInWithApple(identityToken: identityToken, nonce: nonce.raw, name: name)
            let profile = account.profile
            self.name = profile?.displayName ?? ""
            if let handle = profile?.handle, !handle.isEmpty { handleChanged(handle) } else { handleChanged(Self.suggestedHandle(from: self.name)) }
            if account.step == .done { await restoreVoice() }
        } catch let error as BackendError {
            signInProblem = switch true {
            case error.status == 0: "No connection. Check you're online, then try again."
            case error.code == "invalid_token": "That sign-in couldn't be checked. Please try again."
            default: "Sign-in didn't work just now. Please try again."
            }
        } catch {
            signInProblem = "Sign-in didn't work just now. Please try again."
        }
    }

    /// Apple's sheet did not finish: `cancelled` when the person closed it.
    func signInFailed(cancelled: Bool) {
        signInProblem = cancelled ? "Sign-in was cancelled. Try again when you're ready." : "Sign in with Apple didn't finish. Please try again."
    }

    func nameContinue() {
        let trimmed = name.trimmingCharacters(in: .whitespaces)
        guard !trimmed.isEmpty else { return }
        save(next: .handle) { [self] in
            try await account.updateProfile(displayName: String(trimmed.prefix(Self.maxName)))
            if handle.isEmpty { handleChanged(Self.suggestedHandle(from: trimmed)) }
        }
    }

    /// Checks the ID once typing pauses: the rules here, then whether it is free on the server.
    func handleChanged(_ input: String) {
        let typed = String(Handles.normalise(input).prefix(Handles.max + 1))
        check?.cancel()
        handle = typed
        problem = nil
        let rule = Handles.problem(with: typed)
        if typed.isEmpty {
            handleState = .empty
            return
        }
        if let rule {
            handleState = .invalid(rule)
            return
        }
        handleState = .checking
        check = Task { [weak self, account, checkDelay] in
            do {
                try await Task.sleep(for: checkDelay)
                let answer = try await account.checkHandle(typed)
                guard let self, self.handle == typed else { return }
                self.handleState = answer.available ? .available : .taken(suggestions: answer.suggestions)
            } catch is CancellationError {
                // A newer keystroke took over.
            } catch let error as BackendError {
                guard let self, self.handle == typed else { return }
                self.handleState = .unchecked(error.status == 0 ? "No connection, so this ID could not be checked." : "This ID could not be checked just now.")
            } catch {}
        }
    }

    func handleContinue() {
        guard handleState == .available else { return }
        let chosen = handle
        save(next: .language) { [self] in
            do {
                try await account.updateProfile(handle: chosen)
            } catch let error as BackendError where error.code == "handle_taken" {
                // Taken between the check and now: show it as taken, with suggestions.
                let answer = try await account.checkHandle(chosen)
                handleState = .taken(suggestions: answer.suggestions)
                throw StayHere()
            }
        }
    }

    func languageContinue() {
        guard let language else { return }
        save(next: .voice) { [account] in try await account.updateProfile(language: language.tag) }
    }

    /// Back to the step before `step`; from the first, back out to sign in again.
    func back(from step: OnboardingStep) {
        switch step {
        case .name: work = Task { [account] in await account.signOut() }
        case .handle: account.move(to: .name)
        case .language: account.move(to: .handle)
        case .voice: account.move(to: .language)
        case .done: break
        }
    }

    private func save(next: OnboardingStep, work: @escaping @MainActor () async throws -> Void) {
        guard !saving else { return }
        saving = true
        problem = nil
        self.work = Task { [self] in
            defer { saving = false }
            do {
                try await work()
                if editing { saved = true } else { account.move(to: next) }
            } catch is StayHere {
            } catch let error as BackendError {
                problem = error.status == 0 ? "No connection. Check you're online, then try again." : "That could not be saved just now. Please try again."
            } catch {
                problem = "That could not be saved just now. Please try again."
            }
        }
    }

    /// On a new phone, the voice kept on the server comes back, so the interpreter speaks in it too.
    private func restoreVoice() async {
        guard (try? await account.keptVoice()) != nil, let wav = try? await account.keptVoiceAudio(), let audio = try? PCMAudio(wav: wav) else { return }
        try? await voice.save(audio)
    }

    /// Thrown by a step's work to stay on the step without showing a problem: it has shown its own.
    private struct StayHere: Error {}

    /// An ID to start from: the first name, as far as an ID can hold it.
    static func suggestedHandle(from name: String) -> String {
        let first = name.trimmingCharacters(in: .whitespaces).split(separator: " ").first.map(String.init) ?? ""
        let kept = first.lowercased().filter { ("a"..."z").contains($0) || ("0"..."9").contains($0) }
        let handle = String(kept.prefix(Handles.max))
        return handle.count >= Handles.min ? handle : ""
    }
}
