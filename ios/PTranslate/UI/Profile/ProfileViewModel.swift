import Foundation
import Observation

/// The profile: who the user is, their voice, and their account.
@MainActor
@Observable
final class ProfileViewModel {
    /// The voice kept on the server, once known.
    private(set) var keptVoice: KeptVoice?
    private(set) var playingVoice = false
    private(set) var busy = false
    private(set) var problem: String?

    var profile: Profile? { account.profile }
    /// The length of the voice on this phone, if there is one.
    var voiceLength: Duration? { voice.state.duration }
    var hasVoice: Bool { keptVoice != nil || voiceLength != nil }

    @ObservationIgnored private let account: Account
    @ObservationIgnored private let voice: any VoiceSampleStore
    @ObservationIgnored private let player: any AudioPlayer
    @ObservationIgnored private var work: Task<Void, Never>?

    init(account: Account, voice: any VoiceSampleStore, player: any AudioPlayer) {
        self.account = account
        self.voice = voice
        self.player = player
    }

    isolated deinit {
        work?.cancel()
        player.stop()
    }

    /// Fetches the profile and the kept voice again. Run it from `.task`.
    func refresh() async {
        try? await account.refresh()
        if let kept = try? await account.keptVoice() { keptVoice = kept }
    }

    /// Plays the user's voice; on a phone that has not got it yet, fetches it from the server first.
    func playVoiceTapped() {
        if playingVoice {
            player.stop()
            return
        }
        work = Task { [self] in
            playingVoice = true
            problem = nil
            defer { playingVoice = false }
            do {
                let wav: Data
                if let local = try await voice.loadWAV() {
                    wav = local
                } else {
                    wav = try await account.keptVoiceAudio()
                    try await voice.save(try PCMAudio(wav: wav))
                }
                try await player.play(try PCMAudio(wav: wav))
            } catch is CancellationError {
            } catch {
                problem = "Your voice couldn't be played just now."
            }
        }
    }

    /// Deletes the voice from the server and the phone. The user's messages then use the phone's voice.
    func deleteVoiceConfirmed() {
        perform("Your voice couldn't be deleted just now. Please try again.") { [self] in
            try await account.deleteVoice()
            try await voice.delete()
            keptVoice = nil
        }
    }

    func signOutTapped() {
        perform("Signing out didn't work just now. Please try again.") { [account] in await account.signOut() }
    }

    /// Deletes the account and everything with it: the profile, the voice, the messages.
    func deleteAccountConfirmed() {
        perform("Your account couldn't be deleted just now. Please try again.") { [account] in try await account.deleteAccount() }
    }

    private func perform(_ failure: String, _ action: @escaping @MainActor () async throws -> Void) {
        guard !busy else { return }
        busy = true
        problem = nil
        work = Task { [self] in
            defer { busy = false }
            do {
                try await action()
            } catch {
                problem = failure
            }
        }
    }
}
