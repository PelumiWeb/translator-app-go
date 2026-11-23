import Foundation

/// Where the user stands with the subscription, as the server works it out from Apple's signed records (ADR 0034).
struct Standing: Decodable, Equatable, Sendable {
    enum State: String, Decodable, Sendable {
        /// Never subscribed.
        case none
        case trial, active
        /// A payment failed and Apple is retrying: still allowed to send meanwhile.
        case grace
        /// Ended, cancelled or refunded.
        case expired

        /// A state from a newer server reads as not subscribed: the paywall, not a crash.
        init(from decoder: any Decoder) throws {
            self = State(rawValue: try decoder.singleValueContainer().decode(String.self)) ?? .none
        }
    }

    /// What is left of the free trial's cap.
    struct TrialLeft: Decodable, Equatable, Sendable {
        let voiceMSLeft: Int
        let textsLeft: Int

        enum CodingKeys: String, CodingKey {
            case voiceMSLeft = "voice_ms_left"
            case textsLeft = "texts_left"
        }
    }

    /// False while the server lets everyone send: then there is no paywall.
    let required: Bool
    let state: State
    /// When the trial, the period or the grace period ends.
    let until: Date?
    let renews: Bool
    let trial: TrialLeft?

    init(required: Bool, state: State, until: Date? = nil, renews: Bool = false, trial: TrialLeft? = nil) {
        self.required = required
        self.state = state
        self.until = until
        self.renews = renews
        self.trial = trial
    }

    init(from decoder: any Decoder) throws {
        let container = try decoder.container(keyedBy: CodingKeys.self)
        required = try container.decodeIfPresent(Bool.self, forKey: .required) ?? false
        state = try container.decodeIfPresent(State.self, forKey: .state) ?? .none
        until = try container.decodeIfPresent(Date.self, forKey: .until)
        renews = try container.decodeIfPresent(Bool.self, forKey: .renews) ?? false
        trial = try container.decodeIfPresent(TrialLeft.self, forKey: .trial)
    }

    enum CodingKeys: String, CodingKey {
        case required, state, until, renews, trial
    }

    /// Subscribed, in the trial, or in the grace period.
    var subscribed: Bool { state == .trial || state == .active || state == .grace }

    /// Whether the user may send: always, while the server does not require a subscription.
    var canSend: Bool { !required || subscribed }
}

/// The backend's view of the subscription: where the user stands, and the purchases this phone passes on.
final class BillingClient: Sendable {
    private let rest: Rest

    init(rest: Rest) {
        self.rest = rest
    }

    func standing() async throws -> Standing {
        try await rest.get("/v1/me/subscription")
    }

    /// Hands the server a transaction StoreKit gave this phone, as Apple signed it, and its renewal information when
    /// there is some. The server checks Apple's signature itself. Throws with code "subscription_in_use" when the
    /// subscription belongs to another Shinar account, "not_your_purchase" when it was bought for one.
    func sendPurchase(signedTransaction: String, signedRenewalInfo: String?) async throws -> Standing {
        var body = ["signed_transaction": signedTransaction]
        if let signedRenewalInfo { body["signed_renewal_info"] = signedRenewalInfo }
        return try await rest.send("POST", "/v1/me/subscription/apple", json: body)
    }
}
