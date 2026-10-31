import AVFoundation
import SwiftUI
import Translation

@main
struct PTranslateApp: App {
    private let container: AppContainer
    @State private var translate: TranslateViewModel
    @State private var voiceSetup: VoiceSetupViewModel
    /// An ID from a timbre://add link that opened the app, waiting to be offered.
    @State private var addHandle: String?

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
    }

    var body: some Scene {
        WindowGroup {
            AppRoot(container: container, translate: translate, voiceSetup: voiceSetup, addHandle: $addHandle)
                .modifier(LanguagePackDownloads(downloads: container.translationDownloads))
                .onOpenURL { url in
                    if let handle = handle(fromCode: url.absoluteString) { addHandle = handle }
                }
        }
    }
}

/// Lets iOS show its language pack download sheet over whichever screen is showing. It lives here, with the
/// wiring, so that the screens in `UI/` never see a Translation type.
private struct LanguagePackDownloads: ViewModifier {
    let downloads: TranslationDownloads

    func body(content: Content) -> some View {
        content.translationTask(downloads.request) { session in
            await downloads.run(session)
        }
    }
}
