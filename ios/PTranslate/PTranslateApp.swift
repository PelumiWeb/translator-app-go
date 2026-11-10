import AVFoundation
import SwiftUI

@main
struct PTranslateApp: App {
    private let container: AppContainer
    @State private var translate: TranslateViewModel
    @State private var voiceSetup: VoiceSetupViewModel
    /// An ID from a shinar://add link that opened the app, waiting to be offered.
    @State private var addHandle: String?
    /// The launch screen's mark, still showing until the app is ready to be seen.
    @State private var covering = true
    /// The intro plays once, on the way to the welcome; anyone already signed in goes straight in.
    private let playsIntro: Bool

    init() {
        let container = AppContainer()
        let requestMicrophone = { await AVAudioApplication.requestRecordPermission() }
        self.container = container
        _translate = State(
            initialValue: TranslateViewModel(
                recorder: container.recorder,
                pipeline: container.pipeline,
                translator: container.translator,
                models: container.models,
                route: container.route,
                speed: container.speed,
                onDeviceAvailable: container.onDeviceTranscription,
                thresholds: container.thresholds,
                requestMicrophone: requestMicrophone
            )
        )
        _voiceSetup = State(
            initialValue: VoiceSetupViewModel(
                recorder: container.recorder,
                player: container.player,
                samples: container.voice,
                preference: container.voiceSetup,
                account: container.account,
                requestMicrophone: requestMicrophone
            )
        )
        playsIntro = container.account.session == nil && !UserDefaults.standard.bool(forKey: Self.introSeen)
    }

    private static let introSeen = "introSeen"

    var body: some Scene {
        WindowGroup {
            ZStack {
                AppRoot(container: container, translate: translate, voiceSetup: voiceSetup, addHandle: $addHandle)
                if covering {
                    LaunchCover(playsIntro: playsIntro) {
                        UserDefaults.standard.set(true, forKey: Self.introSeen)
                        withAnimation(.easeInOut(duration: 0.25)) { covering = false }
                    }
                    .transition(.opacity)
                    .zIndex(1)
                }
            }
            .onOpenURL { url in
                if let handle = handle(fromCode: url.absoluteString) { addHandle = handle }
            }
        }
    }
}
