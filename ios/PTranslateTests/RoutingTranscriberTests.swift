import Testing
@testable import PTranslate

struct RoutingTranscriberTests {
    private let onDevice = FakeTranscriber(events: [.final(Transcript(text: "from the phone", confidence: 0.9, source: .onDevice))])
    private let cloud = FakeTranscriber(events: [.final(Transcript(text: "from the server", confidence: nil, source: .cloud))])

    private func text(_ transcriber: RoutingTranscriber) async throws -> String? {
        var text: String?
        for try await case .final(let transcript) in transcriber.transcribe(speech(seconds: 1), language: .english) {
            text = transcript.text
        }
        return text
    }

    @Test func sendsEachRecordingWhereTheRouteSaysAtTheTime() async throws {
        let route = RouteSetting(.onDevice)
        let transcriber = RoutingTranscriber(onDevice: onDevice, cloud: cloud, route: route)

        #expect(try await text(transcriber) == "from the phone")

        route.current = .cloud
        #expect(try await text(transcriber) == "from the server")
        #expect(onDevice.calls == 1)
        #expect(cloud.calls == 1)
    }
}
