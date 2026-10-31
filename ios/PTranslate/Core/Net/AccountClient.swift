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
    /// Apple; the server checks that the token carries that hash.
    func signInWithApple(identityToken: String, nonce: String, name: String?) async throws -> SignIn {
        let response: SignInResponse = try await rest.send(
            "POST", "/v1/sessions/apple", json: SignInRequest(identityToken: identityToken, nonce: nonce, name: name)
        )
        return SignIn(token: response.token, profile: response.user, newUser: response.newUser ?? false)
    }

    func me() async throws -> Profile {
        try await rest.get("/v1/me")
    }

    /// Changes the fields given; nil leaves one as it is.
    func updateProfile(displayName: String? = nil, handle: String? = nil, language: String? = nil) async throws -> Profile {
        try await rest.send("PATCH", "/v1/me", json: ProfileUpdate(displayName: displayName, handle: handle, language: language))
    }

    func checkHandle(_ handle: String) async throws -> HandleCheck {
        try await rest.get("/v1/handles/\(handle)")
    }

    /// Finds someone by their ID. Throws with code "user_not_found" when no one has it.
    func findUser(_ handle: String) async throws -> Profile {
        try await rest.get("/v1/users/\(handle)")
    }

    /// Keeps `wav` as the user's voice, with the version of the consent wording they agreed to.
    func putVoice(_ wav: Data, consentVersion: String) async throws -> KeptVoice {
        var form = MultipartForm()
        form.addField("consent", value: consentVersion)
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

    func deleteAccount() async throws {
        try await rest.send(await rest.request("/v1/me", method: "DELETE"))
    }
}

private struct SignInRequest: Encodable {
    let identityToken: String
    let nonce: String
    let name: String?

    enum CodingKeys: String, CodingKey {
        case nonce, name
        case identityToken = "identity_token"
    }
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

    enum CodingKeys: String, CodingKey {
        case handle, language
        case displayName = "display_name"
    }
}
