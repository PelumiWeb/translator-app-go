import SwiftUI

/// Chooses between the interpreter and recording the voice, with settings as a sheet over the interpreter. With so
/// few screens, state values do the job a navigation stack would.
struct AppRoot: View {
    let voice: any VoiceSampleStore
    let translate: TranslateViewModel
    let voiceSetup: VoiceSetupViewModel

    @State private var settingUpVoice: Bool
    @State private var showingSettings = false
    @State private var voiceOpenedFromSettings = false

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
        Group {
            if settingUpVoice {
                VoiceSetupScreen(viewModel: voiceSetup, onDone: {
                    settingUpVoice = false
                    // Back to where the user came from.
                    showingSettings = voiceOpenedFromSettings
                    voiceOpenedFromSettings = false
                })
            } else {
                TranslateScreen(
                    viewModel: translate,
                    hasOwnVoice: voice.state.duration != nil,
                    onSettingsTapped: { showingSettings = true }
                )
                .sheet(isPresented: $showingSettings) {
                    SettingsScreen(viewModel: translate, savedVoice: voice.state.duration, onVoiceTapped: {
                        showingSettings = false
                        voiceOpenedFromSettings = true
                        settingUpVoice = true
                    })
                }
            }
        }
        .tint(Palette.them)
    }
}
