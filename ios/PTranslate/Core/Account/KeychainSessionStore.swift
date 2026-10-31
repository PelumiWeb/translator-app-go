import Foundation
import Security

/// Keeps the session's token in the Keychain, readable only on this phone once it has been unlocked (ADR 0018),
/// and the profile, which is not secret, in user defaults, so the app can open without waiting for the network.
final class KeychainSessionStore: SessionStore, @unchecked Sendable {
    // UserDefaults is documented as thread-safe; the Keychain calls are too. Hence @unchecked.
    private let defaults: UserDefaults
    private let service: String

    init(defaults: UserDefaults, service: String = "session") {
        self.defaults = defaults
        self.service = service
    }

    func load() -> Session? {
        guard
            let token = readToken(),
            let data = defaults.data(forKey: Self.profileKey),
            let profile = try? JSONDecoder().decode(Profile.self, from: data)
        else { return nil }
        return Session(token: token, profile: profile)
    }

    func save(_ session: Session) {
        writeToken(session.token)
        defaults.set(try? JSONEncoder().encode(session.profile), forKey: Self.profileKey)
    }

    func clear() {
        SecItemDelete(query() as CFDictionary)
        defaults.removeObject(forKey: Self.profileKey)
    }

    private func query() -> [String: Any] {
        [kSecClass as String: kSecClassGenericPassword, kSecAttrService as String: service, kSecAttrAccount as String: "token"]
    }

    private func readToken() -> String? {
        var item: CFTypeRef?
        var search = query()
        search[kSecReturnData as String] = true
        search[kSecMatchLimit as String] = kSecMatchLimitOne
        guard SecItemCopyMatching(search as CFDictionary, &item) == errSecSuccess, let data = item as? Data else { return nil }
        return String(data: data, encoding: .utf8)
    }

    private func writeToken(_ token: String) {
        SecItemDelete(query() as CFDictionary)
        var item = query()
        item[kSecValueData as String] = Data(token.utf8)
        item[kSecAttrAccessible as String] = kSecAttrAccessibleAfterFirstUnlockThisDeviceOnly
        SecItemAdd(item as CFDictionary, nil)
    }

    private static let profileKey = "session_profile"
}
