import SwiftUI

/// Chooses between the app's two screens. With only two, a boolean does the job a navigation stack would.
struct AppRoot: View {
    let voice: any VoiceSampleStore
    let translate: TranslateViewModel
    let voiceSetup: VoiceSetupViewModel

    @State private var settingUpVoice: Bool

    init(
        voice: any VoiceSampleStore,
        voiceSetupSkipped: Bool,
        translate: TranslateViewModel,
        voiceSetup: VoiceSetupViewModel
    ) {
        self.voice = voice
        self.translate = translate
        self.voiceSetup = voiceSetup
        // On first launch the user is taken to record their voice. They are not taken there again once they have
        // recorded one, or said "not now".
        _settingUpVoice = State(initialValue: voice.state == .notRecorded && !voiceSetupSkipped)
    }

    var body: some View {
        if settingUpVoice {
            VoiceSetupScreen(viewModel: voiceSetup, onDone: { settingUpVoice = false })
        } else {
            TranslateScreen(
                viewModel: translate,
                savedVoice: voice.state.duration,
                onVoiceTapped: { settingUpVoice = true }
            )
        }
    }
}
