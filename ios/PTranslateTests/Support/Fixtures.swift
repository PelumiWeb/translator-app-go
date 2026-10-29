import Foundation

/// Files the tests use from the repository instead of copying them into the test bundle: a simulator can read the
/// Mac's files.
enum Fixtures {
    /// The repository root: this file is at ios/PTranslateTests/Support/ in it.
    static let repository = URL(filePath: #filePath)
        .deletingLastPathComponent()
        .deletingLastPathComponent()
        .deletingLastPathComponent()
        .deletingLastPathComponent()

    /// The Whisper model, where `make model-download` puts it for the server.
    static let model = repository.appending(path: "server/data/models/ggml-base-q5_1.bin")

    /// whisper.cpp's sample: eleven seconds of English.
    static let clip = repository.appending(path: "third_party/whisper.cpp/samples/jfk.wav")

    static let modelDownloaded = FileManager.default.fileExists(atPath: model.path(percentEncoded: false))

    static let noModel = "no model in server/data/models; run `make model-download`"
}
