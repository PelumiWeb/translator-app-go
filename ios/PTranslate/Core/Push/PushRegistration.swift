import Foundation

/// Keeps the server told where this phone receives notifications (ADR 0026): each time someone signs in, and each
/// time Apple gives the phone a new address. Signing out needs nothing here: the server forgets the address with
/// the session.
@MainActor
final class PushRegistration {
    /// "apns", or "apns-sandbox": a build run from Xcode is registered with Apple's test service.
    let service: String
    private let register: @MainActor (_ service: String, _ token: String) async throws -> Void
    private let retryDelays: [Duration]

    private var session: String?
    private var address: String?
    /// The session and address last sent, or being sent.
    private var sent: (session: String, address: String)?
    private var sending: Task<Void, Never>?

    /// - Parameters:
    ///   - register: sends the address to the server, for the session signed in now.
    ///   - retryDelays: how long to wait before each further try, when the server cannot be reached.
    init(
        service: String,
        register: @escaping @MainActor (_ service: String, _ token: String) async throws -> Void,
        retryDelays: [Duration] = [.seconds(10), .seconds(60), .seconds(300)]
    ) {
        self.service = service
        self.register = register
        self.retryDelays = retryDelays
    }

    isolated deinit {
        sending?.cancel()
    }

    /// The session's token, or nil while no one is signed in.
    func onSession(_ token: String?) {
        session = token
        sendIfReady()
    }

    /// Apple gave this phone `deviceToken`: when the app starts, or a new one later.
    func onAddress(_ deviceToken: Data) {
        address = deviceToken.map { String(format: "%02x", $0) }.joined()
        sendIfReady()
    }

    private func sendIfReady() {
        guard let session, let address else { return }
        if let sent, sent.session == session, sent.address == address { return }
        sent = (session, address)
        // A new session or address while one is still being sent replaces it.
        sending?.cancel()
        sending = Task { [service, register, retryDelays] in
            for wait in [Duration.zero] + retryDelays {
                do {
                    try await Task.sleep(for: wait)
                    try await register(service, address)
                    return
                } catch is CancellationError {
                    return
                } catch {
                    // Offline, or the server is restarting: try again after the next wait, then at the next launch.
                }
            }
        }
    }
}
