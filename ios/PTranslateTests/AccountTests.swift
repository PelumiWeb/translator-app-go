import CryptoKit
import Foundation
import Testing
@testable import PTranslate

/// In memory, for tests: the session a store would keep across launches.
final class MemorySessionStore: SessionStore, @unchecked Sendable {
    var session: Session?

    init(_ session: Session? = nil) {
        self.session = session
    }

    func load() -> Session? { session }
    func save(_ session: Session) { self.session = session }
    func clear() { session = nil }
}

final class MemoryProgress: OnboardingProgress {
    var step: OnboardingStep

    init(_ step: OnboardingStep = .name) {
        self.step = step
    }
}

/// An account talking to `server`, its requests carrying its own token, as the app wires it.
@MainActor
func testAccount(server: StubServer, store: MemorySessionStore = MemorySessionStore(), progress: MemoryProgress = MemoryProgress()) -> Account {
    final class Box { weak var account: Account? }
    let box = Box()
    let rest = Rest(
        baseURL: server.baseURL,
        session: server.session,
        token: { @MainActor in box.account?.token },
        sessionEnded: { @MainActor in box.account?.forget() }
    )
    let account = Account(client: AccountClient(rest: rest), store: store, progress: progress)
    box.account = account
    return account
}

/// The same cases as the Android tests.
@MainActor
struct AccountTests {
    private let server = StubServer()
    private let signedUp = Session(token: "tok", profile: Profile(id: "u1", displayName: "Ada", handle: "ada", language: "en"))

    @Test func aFirstSignInKeepsTheSessionAndStartsFromTheName() async throws {
        server.enqueue(.json(status: 201, body: #"{"token":"t1","user":{"id":"u1","display_name":"Ada","handle":"","language":""},"new_user":true}"#))
        let store = MemorySessionStore()
        let account = testAccount(server: server, store: store)

        try await account.signInWithApple(identityToken: "id-token", nonce: "raw", name: "Ada Lovelace")

        let request = try #require(server.requests.first)
        #expect(request.path == "/v1/sessions/apple")
        let body = try JSONSerialization.jsonObject(with: request.body) as? [String: String]
        #expect(body == ["identity_token": "id-token", "nonce": "raw", "name": "Ada Lovelace"])
        #expect(account.token == "t1")
        #expect(store.session?.token == "t1")
        #expect(account.step == .name)
    }

    @Test func anAccountSetUpElsewhereGoesStraightIn() async throws {
        server.enqueue(.json(status: 200, body: #"{"token":"t2","user":{"id":"u1","display_name":"Ada","handle":"ada","language":"en"},"new_user":false}"#))
        let account = testAccount(server: server)
        try await account.signInWithApple(identityToken: "id-token", nonce: "n", name: nil)
        #expect(account.step == .done)
    }

    @Test func requestsCarryTheTokenAndA401ForgetsTheSession() async throws {
        server.enqueue(.json(status: 401, body: #"{"error":{"code":"unauthenticated","message":"the session has ended"}}"#))
        let store = MemorySessionStore(signedUp)
        let account = testAccount(server: server, store: store, progress: MemoryProgress(.done))

        await #expect(throws: BackendError.self) { try await account.refresh() }

        #expect(server.requests.first?.header("Authorization") == "Bearer tok")
        #expect(account.session == nil)
        #expect(store.session == nil)
        #expect(account.step == .name)
    }

    @Test func aProfileChangeSendsOnlyWhatChanges() async throws {
        server.enqueue(.json(status: 200, body: #"{"id":"u1","display_name":"Ada","handle":"ada.l","language":"en"}"#))
        let account = testAccount(server: server, store: MemorySessionStore(signedUp))

        try await account.updateProfile(handle: "ada.l")

        let request = try #require(server.requests.first)
        #expect(request.method == "PATCH")
        #expect(request.bodyText == #"{"handle":"ada.l"}"#)
        #expect(account.profile?.handle == "ada.l")
    }

    @Test func anIDCheckReadsTheSuggestionsAndAMissingPersonHasACode() async throws {
        server.enqueue(.json(status: 200, body: #"{"handle":"ada","available":false,"suggestions":["ada.l","ada26"]}"#))
        server.enqueue(.json(status: 404, body: #"{"error":{"code":"user_not_found","message":"no one has that ID"}}"#))
        let account = testAccount(server: server, store: MemorySessionStore(signedUp))

        #expect(try await account.checkHandle("ada") == HandleCheck(handle: "ada", available: false, suggestions: ["ada.l", "ada26"]))
        do {
            _ = try await account.findUser("nobody")
            Issue.record("expected the refusal")
        } catch let error as BackendError {
            #expect(error.code == "user_not_found")
            #expect(error.status == 404)
        }
        // A 404 is not the end of the session.
        #expect(account.session != nil)
    }

    @Test func signingOutForgetsTheSessionEvenWhenTheServerCannotBeTold() async {
        server.shutdown()
        let account = testAccount(server: server, store: MemorySessionStore(signedUp), progress: MemoryProgress(.done))
        await account.signOut()
        #expect(account.session == nil)
    }

    @Test func theNonceAppleGetsIsTheSHA256OfTheOneTheServerGets() {
        let nonce = Nonce.random()
        let expected = SHA256.hash(data: Data(nonce.raw.utf8)).map { String(format: "%02x", $0) }.joined()
        #expect(nonce.hashed == expected)
        #expect(nonce.raw.count >= 43)
        #expect(Nonce.random().raw != nonce.raw)
    }

    @Test(arguments: [
        ("ada", nil),
        ("ada.l_26", nil),
        ("ab", "At least 3 characters."),
        (String(repeating: "a", count: 21), "At most 20 characters."),
        ("ada-l", "Only letters, numbers, dots and underscores."),
        ("Ada", "Only letters, numbers, dots and underscores."),
    ] as [(String, String?)])
    func theIDRulesSayWhichOneIsBroken(handle: String, problem: String?) {
        #expect(Handles.problem(with: handle) == problem)
    }

    @Test func whatIsTypedBecomesAnID() {
        #expect(Handles.normalise("  @Ada.L ") == "ada.l")
    }
}
