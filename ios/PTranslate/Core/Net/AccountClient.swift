import Foundation

/// The result of signing in: the session's token, and whether the account was made just now.
struct SignIn: Sendable {
    let token: String
    let profile: Profile
    let newUser: Bool
}

/// The backend's account endpoints (ADR 0018).
final class AccountClient: Sendable {
    private let rest: Rest

    init(rest: Rest) {
        self.rest = rest
    }

    /// Signs in with the identity token Apple gave the phone. `nonce` is the raw nonce whose SHA-256 was given to
    /// Apple; the server checks that the token carries that hash. `authorizationCode` is traded by the server for what
    /// it needs to end the Apple sign-in when the account is deleted.
    func signInWithApple(identityToken: String, nonce: String, name: String?, authorizationCode: String? = nil) async throws -> SignIn {
        let response: SignInResponse = try await rest.send(
            "POST", "/v1/sessions/apple",
            json: SignInRequest(identityToken: identityToken, nonce: nonce, name: name, authorizationCode: authorizationCode)
        )
        return SignIn(token: response.token, profile: response.user, newUser: response.newUser ?? false)
    }

    /// Signs in to a test account: an email address whose code is fixed on the server, as there is no mail service
    /// yet. Throws with code "invalid_code" when the two do not match.
    func signInWithEmail(_ email: String, code: String) async throws -> SignIn {
        let response: SignInResponse = try await rest.send("POST", "/v1/sessions/email", json: EmailSignInRequest(email: email, code: code))
        return SignIn(token: response.token, profile: response.user, newUser: response.newUser ?? false)
    }

    func me() async throws -> Profile {
        try await rest.get("/v1/me")
    }

    /// Changes the fields given; nil leaves one as it is.
    func updateProfile(
        displayName: String? = nil, handle: String? = nil, language: String? = nil, servicesConsent: String? = nil
    ) async throws -> Profile {
        try await rest.send(
            "PATCH", "/v1/me",
            json: ProfileUpdate(displayName: displayName, handle: handle, language: language, servicesConsent: servicesConsent)
        )
    }

    func checkHandle(_ handle: String) async throws -> HandleCheck {
        try await rest.get("/v1/handles/\(handle)")
    }

    /// Finds someone by their ID. Throws with code "user_not_found" when no one has it.
    func findUser(_ handle: String) async throws -> Profile {
        try await rest.get("/v1/users/\(handle)")
    }

    /// Keeps `wav` as the user's voice, with the words read in it and the version of the consent wording they
    /// agreed to.
    func putVoice(_ wav: Data, transcript: String, consentVersion: String) async throws -> KeptVoice {
        var form = MultipartForm()
        form.addField("consent", value: consentVersion)
        form.addField("transcript", value: transcript)
        form.addFile("voice", filename: "voice.wav", contentType: "audio/wav", data: wav)
        var request = await rest.request("/v1/me/voice", method: "PUT")
        request.setValue(form.contentType, forHTTPHeaderField: "Content-Type")
        request.httpBody = form.body()
        return try rest.decode(try await rest.send(request))
    }

    /// The user's kept voice, or nil if they keep none.
    func voice() async throws -> KeptVoice? {
        do {
            return try await rest.get("/v1/me/voice") as KeptVoice
        } catch let error as BackendError where error.code == "no_voice" {
            return nil
        }
    }

    func voiceAudio() async throws -> Data {
        try await rest.send(await rest.request("/v1/me/voice/audio"))
    }

    func deleteVoice() async throws {
        try await rest.send(await rest.request("/v1/me/voice", method: "DELETE"))
    }

    func signOut() async throws {
        try await rest.send(await rest.request("/v1/sessions/current", method: "DELETE"))
    }

    /// Tells the server where this phone receives notifications: its `token` at a push `service` ("apns", or
    /// "apns-sandbox" for a build run from Xcode). It belongs to the session, so signing out forgets it.
    func setPushToken(service: String, token: String) async throws {
        var request = await rest.request("/v1/sessions/current/push", method: "PUT")
        request.setValue("application/json", forHTTPHeaderField: "Content-Type")
        request.httpBody = try JSONEncoder().encode(PushTokenRequest(service: service, token: token))
        try await rest.send(request)
    }

    func deleteAccount() async throws {
        try await rest.send(await rest.request("/v1/me", method: "DELETE"))
    }
}

private struct PushTokenRequest: Encodable {
    let service: String
    let token: String
}

private struct SignInRequest: Encodable {
    let identityToken: String
    let nonce: String
    let name: String?
    let authorizationCode: String?

    enum CodingKeys: String, CodingKey {
        case nonce, name
        case identityToken = "identity_token"
        case authorizationCode = "authorization_code"
    }
}

private struct EmailSignInRequest: Encodable {
    let email: String
    let code: String
}

private struct SignInResponse: Decodable {
    let token: String
    let user: Profile
    let newUser: Bool?

    enum CodingKeys: String, CodingKey {
        case token, user
        case newUser = "new_user"
    }
}

/// The synthesised `Encodable` leaves nil properties out (`encodeIfPresent`), so the server receives only the
/// fields being changed.
private struct ProfileUpdate: Encodable {
    let displayName: String?
    let handle: String?
    let language: String?
    let servicesConsent: String?

    enum CodingKeys: String, CodingKey {
        case handle, language
        case displayName = "display_name"
        case servicesConsent = "services_consent"
    }
}
