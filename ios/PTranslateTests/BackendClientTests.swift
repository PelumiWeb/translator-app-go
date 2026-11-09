import Foundation
import Testing
@testable import PTranslate

/// The same cases as the Android tests.
struct BackendClientTests {
    private let server = StubServer()

    @Test func transcribingAndSpeakingCarryTheSessionToken() async throws {
        server.enqueue(.json(status: 202, body: #"{"id":"j1","status":"queued","events_url":"/v1/jobs/j1/events"}"#))
        server.enqueue(.json(status: 202, body: #"{"id":"j2","status":"queued","events_url":"/v1/jobs/j2/events","audio_url":"/v1/jobs/j2/audio"}"#))
        let client = BackendClient(baseURL: server.baseURL, session: server.session, token: { "tok" })

        _ = try await client.createJob(wav: Data([1, 2, 3]), sourceLanguage: Language("en"), idempotencyKey: "k1")
        _ = try await client.createSpeech(voiceWAV: Data([1, 2, 3]), text: "hola", language: Language("es"), idempotencyKey: "k2")

        #expect(server.requests.map { $0.header("Authorization") } == ["Bearer tok", "Bearer tok"])
    }

    @Test func withNoOneSignedInNothingPretendsToBeAToken() async throws {
        server.enqueue(.json(status: 202, body: #"{"id":"j1","status":"queued","events_url":"/v1/jobs/j1/events"}"#))
        let client = BackendClient(baseURL: server.baseURL, session: server.session, token: { nil })

        _ = try await client.createJob(wav: Data([1, 2, 3]), sourceLanguage: Language("en"), idempotencyKey: "k1")

        #expect(server.requests.first?.header("Authorization") == nil)
    }
}
