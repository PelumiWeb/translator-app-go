import Foundation
import Testing
@testable import PTranslate

/// The same cases as the Android tests.
@MainActor
struct PushRegistrationTests {
    /// Records what is sent, as "session service address", failing while `failures` lasts.
    private final class Server {
        var session: String?
        var failures = 0
        var sent: [String] = []
    }

    private let server = Server()

    private func registration() -> PushRegistration {
        PushRegistration(service: "apns", register: { [server] service, token in
            if server.failures > 0 {
                server.failures -= 1
                throw URLError(.notConnectedToInternet)
            }
            server.sent.append("\(server.session ?? "-") \(service) \(token)")
        }, retryDelays: [.milliseconds(20), .milliseconds(20)])
    }

    private func signIn(_ registration: PushRegistration, _ session: String?) {
        server.session = session
        registration.onSession(session)
    }

    /// Waits, up to a few seconds, until `count` sends have reached the server: each send is a task of its own,
    /// and on a busy machine it may take a while to run.
    private func waitUntilSent(_ count: Int) async throws {
        for _ in 0..<250 where server.sent.count < count {
            try await Task.sleep(for: .milliseconds(20))
        }
    }

    /// Waits until the server has refused every failure it was set to give.
    private func waitUntilFailuresUsed() async throws {
        for _ in 0..<250 where server.failures > 0 {
            try await Task.sleep(for: .milliseconds(20))
        }
    }

    /// Gives a send that should not happen the time to happen.
    private func pause() async throws {
        try await Task.sleep(for: .milliseconds(100))
    }

    @Test func nothingIsSentUntilSomeoneIsSignedInAndThePhoneHasAnAddress() async throws {
        let registration = registration()
        registration.onAddress(Data([0xab, 0x01]))
        try await pause()
        #expect(server.sent.isEmpty)

        signIn(registration, "s1")
        try await waitUntilSent(1)
        #expect(server.sent == ["s1 apns ab01"])
    }

    @Test func aNewSignInAndANewAddressAreEachSent() async throws {
        let registration = registration()
        signIn(registration, "s1")
        registration.onAddress(Data([0x01]))
        try await waitUntilSent(1)

        registration.onAddress(Data([0x02]))
        try await waitUntilSent(2)
        signIn(registration, nil)
        try await pause()
        signIn(registration, "s2")
        try await waitUntilSent(3)

        #expect(server.sent == ["s1 apns 01", "s1 apns 02", "s2 apns 02"])
    }

    @Test func itTriesAgainWhileTheServerCannotBeReachedThenWaitsForTheNextLaunch() async throws {
        let registration = registration()
        server.failures = 1
        signIn(registration, "s1")
        registration.onAddress(Data([0x01]))
        try await waitUntilSent(1)
        #expect(server.sent == ["s1 apns 01"])

        // Three failures use up the first try and both retries.
        server.failures = 3
        registration.onAddress(Data([0x02]))
        try await waitUntilFailuresUsed()
        try await pause()
        #expect(server.failures == 0)
        #expect(server.sent == ["s1 apns 01"])
    }
}
