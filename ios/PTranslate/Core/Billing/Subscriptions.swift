import Foundation
import Observation
import StoreKit

/// The subscription on this phone: where the user stands, and every transaction StoreKit gives the app, passed on to
/// the server, which checks Apple's signature and decides what it allows (ADR 0034). A transaction is finished only
/// once the server has it, so one that could not be sent is sent again on the next launch.
@MainActor
@Observable
final class Subscriptions {
    /// The one subscription, a month with a week's free trial, as set up in App Store Connect.
    static let productID = "com.pelumi.shinar.monthly"

    /// Why the last purchase did not count.
    enum Problem: Equatable {
        /// The Apple account's subscription belongs to another Shinar account.
        case inUse
        /// It was bought for another Shinar account.
        case notYours
        /// The server could not check it with Apple.
        case unchecked
        /// The server could not be reached; it is sent again later.
        case offline
    }

    /// What came of a purchase made on the paywall, for the screen to say once the paywall has closed.
    enum Outcome: Equatable, Identifiable {
        /// It went through: in the free trial, or subscribed.
        case started(trial: Bool)
        /// Waiting for someone to approve it, such as a family organiser (Ask to Buy).
        case pending
        /// Bought, but the server did not count it.
        case problem(Problem)
        /// StoreKit could not complete it; nothing was charged.
        case failed

        var id: String { String(describing: self) }

        /// After a purchase went through and was handed to the server: what the user is told.
        static func afterPurchase(problem: Problem?, standing: Standing?) -> Outcome {
            if let problem { return .problem(problem) }
            return .started(trial: standing?.state == .trial)
        }
    }

    /// Nil until the server has said, and when no one is signed in.
    private(set) var standing: Standing?
    private(set) var problem: Problem?
    /// The last purchase's outcome, until the screen has said it (`takeOutcome`).
    private(set) var outcome: Outcome?

    /// The paywall stands between the user and sending: the server requires a subscription, and they have none.
    var locked: Bool { standing.map { !$0.canSend } ?? false }

    @ObservationIgnored private let client: BillingClient
    @ObservationIgnored private let userID: @MainActor () -> String?
    @ObservationIgnored private var updates: Task<Void, Never>?

    /// `userID` is the signed-in user's ID, or nil when no one is signed in.
    init(client: BillingClient, userID: @escaping @MainActor () -> String?) {
        self.client = client
        self.userID = userID
    }

    isolated deinit {
        updates?.cancel()
    }

    /// Listens, for as long as the app runs, for the transactions that happen outside a purchase made in it:
    /// renewals, refunds, Ask to Buy approvals, offer codes, a purchase on another device.
    func listen() {
        guard updates == nil else { return }
        updates = Task { [weak self] in
            for await verification in Transaction.updates {
                await self?.deliver(verification)
            }
        }
    }

    /// The options a purchase carries: the user's ID, which the server checks the purchase against.
    var purchaseOptions: Set<Product.PurchaseOption> {
        guard let id = userID(), let token = UUID(uuidString: id) else { return [] }
        return [.appAccountToken(token)]
    }

    /// A purchase made on the paywall: passed on to the server when it went through. Leaves an outcome to tell the
    /// user, except when they backed out of Apple's sheet, where there is nothing to tell.
    func purchased(_ result: Product.PurchaseResult) async {
        switch result {
        case .success(let verification):
            problem = nil
            await deliver(verification)
            outcome = .afterPurchase(problem: problem, standing: standing)
        case .pending:
            outcome = .pending
        case .userCancelled:
            outcome = nil
        @unknown default:
            outcome = nil
        }
    }

    /// StoreKit could not complete a purchase.
    func purchaseFailed() {
        outcome = .failed
    }

    /// The outcome to tell the user, once: it is gone after.
    func takeOutcome() -> Outcome? {
        defer { outcome = nil }
        return outcome
    }

    /// On signing in, and after Restore: everything this Apple account holds for the app, to the server, then where
    /// the user stands.
    func sync() async {
        guard userID() != nil else {
            signedOut()
            return
        }
        for await verification in Transaction.currentEntitlements {
            await deliver(verification)
        }
        for await verification in Transaction.unfinished {
            await deliver(verification)
        }
        await refresh()
    }

    /// Asks the server where the user stands. On failure, what was known stays.
    func refresh() async {
        guard userID() != nil else {
            signedOut()
            return
        }
        do {
            standing = try await client.standing()
        } catch {}
    }

    func signedOut() {
        standing = nil
        problem = nil
    }

    /// Sends a transaction to the server, whose answer is where the user now stands.
    private func deliver(_ verification: VerificationResult<Transaction>) async {
        // No one to give it to: left unfinished, it is delivered again, and sent on signing in.
        guard userID() != nil else { return }
        // StoreKit's own check is only a first filter: the server checks Apple's signature for itself.
        guard case .verified(let transaction) = verification, transaction.productType == .autoRenewable else { return }
        let renewal = await transaction.subscriptionStatus?.renewalInfo.jwsRepresentation
        do {
            standing = try await client.sendPurchase(signedTransaction: verification.jwsRepresentation, signedRenewalInfo: renewal)
            problem = nil
            await transaction.finish()
        } catch let error as BackendError where !error.retryable && error.status != 0 {
            problem = switch error.code {
            case "subscription_in_use": .inUse
            case "not_your_purchase": .notYours
            default: .unchecked
            }
            // The server's answer will not change by asking again. StoreKit keeps the entitlement either way, and a
            // later sync sends it again.
            await transaction.finish()
        } catch {
            problem = .offline
        }
    }
}
