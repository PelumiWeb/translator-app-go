import Foundation
import Observation

/// The version of the consent wording on the voice screen. The server keeps it with the voice, so it is known later
/// what each person agreed to. Change it whenever the wording changes.
let voiceConsentVersion = "2026-10-31"

/// How far the user has come through signing up on this phone.
enum OnboardingStep: String, Sendable {
    case name, handle, language, voice, done
}

/// Remembers `OnboardingStep` across launches.
protocol OnboardingProgress: AnyObject {
    var step: OnboardingStep { get set }
}

final class UserDefaultsOnboardingProgress: OnboardingProgress {
    private let defaults: UserDefaults

    init(defaults: UserDefaults) {
        self.defaults = defaults
    }

    var step: OnboardingStep {
        get { defaults.string(forKey: "onboarding_step").flatMap(OnboardingStep.init(rawValue:)) ?? .name }
        set { defaults.set(newValue.rawValue, forKey: "onboarding_step") }
    }
}

/// The signed-in user, if any, kept across launches, and what they can do with their account. The single source of
/// truth for "who is signed in": a session that the server says has ended is forgotten here.
@MainActor
@Observable
final class Account {
    private(set) var session: Session?

    /// Where the user is in signing up; `.done` once they can use the app.
    private(set) var step: OnboardingStep

    @ObservationIgnored private let client: AccountClient
    @ObservationIgnored private let store: any SessionStore
    @ObservationIgnored private let progress: any OnboardingProgress

    init(client: AccountClient, store: any SessionStore, progress: any OnboardingProgress) {
        self.client = client
        self.store = store
        self.progress = progress
        session = store.load()
        step = progress.step
    }

    var token: String? { session?.token }
    var profile: Profile? { session?.profile }

    /// Signs in with the identity token Apple gave the phone; `nonce` is the raw nonce whose hash Apple put in it.
    /// Apple tells the app the person's name only the first time, so it is passed on when there is one. A new
    /// account starts signing up from its name; one already set up on another phone goes straight in.
    func signInWithApple(identityToken: String, nonce: String, name: String?) async throws {
        let signIn = try await client.signInWithApple(identityToken: identityToken, nonce: nonce, name: name)
        keep(Session(token: signIn.token, profile: signIn.profile))
        move(to: signIn.newUser || !signIn.profile.isComplete ? .name : .done)
    }

    /// Fetches the profile again, as it is on the server.
    func refresh() async throws {
        update(try await client.me())
    }

    /// Changes the fields given; nil leaves one as it is. Throws with code "handle_taken" for an ID someone has.
    func updateProfile(displayName: String? = nil, handle: String? = nil, language: String? = nil) async throws {
        update(try await client.updateProfile(displayName: displayName, handle: handle, language: language))
    }

    func checkHandle(_ handle: String) async throws -> HandleCheck {
        try await client.checkHandle(handle)
    }

    func findUser(_ handle: String) async throws -> Profile {
        try await client.findUser(handle)
    }

    func keptVoice() async throws -> KeptVoice? {
        try await client.voice()
    }

    func keptVoiceAudio() async throws -> Data {
        try await client.voiceAudio()
    }

    func keepVoice(_ wav: Data) async throws -> KeptVoice {
        try await client.putVoice(wav, consentVersion: voiceConsentVersion)
    }

    func deleteVoice() async throws {
        try await client.deleteVoice()
    }

    /// Records the next step of signing up as reached.
    func move(to step: OnboardingStep) {
        progress.step = step
        self.step = step
    }

    /// Signs out here even if the server cannot be told; the session then ends when the account is deleted.
    func signOut() async {
        try? await client.signOut()
        forget()
    }

    /// Deletes the account on the server, with the voice and everything said, then signs out.
    func deleteAccount() async throws {
        try await client.deleteAccount()
        forget()
    }

    /// Forgets the session on this phone, as when the server says it has ended.
    func forget() {
        store.clear()
        session = nil
        move(to: .name)
    }

    private func keep(_ session: Session) {
        store.save(session)
        self.session = session
    }

    private func update(_ profile: Profile) {
        guard var current = session else { return }
        current.profile = profile
        keep(current)
    }
}
