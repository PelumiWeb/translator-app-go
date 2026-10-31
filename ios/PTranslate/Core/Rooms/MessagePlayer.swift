import Foundation

/// Plays a voice message as a listener hears it: the audio the server made in the sender's voice, or their own
/// recording, or when the sender keeps no voice, the translation read by this phone.
@MainActor
final class MessagePlayer {
    private let rooms: RoomsClient
    private let player: any AudioPlayer
    private let phoneVoice: any SpeechSynthesizer

    /// The last few clips, so playing a note again does not fetch it again.
    private var cache: [String: PCMAudio] = [:]
    private var cacheOrder: [String] = []

    init(rooms: RoomsClient, player: any AudioPlayer, phoneVoice: any SpeechSynthesizer) {
        self.rooms = rooms
        self.player = player
        self.phoneVoice = phoneVoice
    }

    /// Plays `rendition` and returns when it has finished or been stopped. `onStart` is called when it becomes
    /// audible, with its length when that is known (the phone's voice does not say).
    func play(_ rendition: Rendition, onStart: (Duration?) -> Void) async throws {
        if let url = rendition.audioURL {
            let audio = try await clip(url)
            onStart(audio.duration)
            try await player.play(audio)
        } else if !rendition.text.isEmpty {
            onStart(nil)
            try await phoneVoice.speak(rendition.text, language: Language(rendition.lang))
        }
    }

    func stop() {
        player.stop()
        Task { [phoneVoice] in await phoneVoice.stop() }
    }

    private func clip(_ url: String) async throws -> PCMAudio {
        if let cached = cache[url] { return cached }
        let audio = try await Self.decode(try await rooms.audio(url))
        cache[url] = audio
        cacheOrder.append(url)
        if cacheOrder.count > 8 { cache[cacheOrder.removeFirst()] = nil }
        return audio
    }

    @concurrent
    private nonisolated static func decode(_ wav: Data) async throws -> PCMAudio {
        try PCMAudio(wav: wav)
    }
}
