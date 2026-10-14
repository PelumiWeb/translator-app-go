import AVFoundation
import SwiftUI

@main
struct PTranslateApp: App {
    @State private var translate: TranslateViewModel

    init() {
        let container = AppContainer()
        _translate = State(
            initialValue: TranslateViewModel(
                recorder: container.recorder,
                pipeline: container.pipeline,
                requestMicrophone: { await AVAudioApplication.requestRecordPermission() }
            )
        )
    }

    var body: some Scene {
        WindowGroup {
            TranslateScreen(viewModel: translate)
        }
    }
}
