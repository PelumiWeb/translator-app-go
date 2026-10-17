import AVFoundation
import SwiftUI

@main
struct PTranslateApp: App {
    private let container: AppContainer
    @State private var translate: TranslateViewModel
    @State private var voiceSetup: VoiceSetupViewModel

    init() {
        let container = AppContainer()
        let requestMicrophone = { await AVAudioApplication.requestRecordPermission() }
        self.container = container
        _translate = State(
            initialValue: TranslateViewModel(
                recorder: container.recorder,
                pipeline: container.pipeline,
                requestMicrophone: requestMicrophone
            )
        )
        _voiceSetup = State(
            initialValue: VoiceSetupViewModel(
                recorder: container.recorder,
                player: container.player,
                samples: container.voice,
                preference: container.voiceSetup,
                requestMicrophone: requestMicrophone
            )
        )
    }

    var body: some Scene {
        WindowGroup {
            AppRoot(
                voice: container.voice,
                voiceSetupSkipped: container.voiceSetup.skipped,
                translate: translate,
                voiceSetup: voiceSetup
            )
        }
    }
}
