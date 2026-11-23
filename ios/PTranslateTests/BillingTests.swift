import Foundation
import StoreKit
import StoreKitTest
import Testing
@testable import PTranslate

/// The subscription: the server's view of it over the wire, how the app reads it, and the plan as StoreKit sells it
/// (ADR 0034).
struct BillingTests {
    private let server = StubServer()
    private let client: BillingClient

    init() {
        client = BillingClient(rest: Rest(baseURL: server.baseURL, session: server.session, token: { "tok" }, sessionEnded: {}))
    }

    @Test func whereTheUserStandsIsReadWithWhatIsLeftOfTheTrial() async throws {
        server.enqueue(.json(status: 200, body: """
            {"required":true,"state":"trial","until":"2026-11-30T09:00:00Z","renews":true,"trial":{"voice_ms_left":540000,"texts_left":48}}
            """))

        let standing = try await client.standing()

        #expect(standing.required && standing.state == .trial && standing.renews)
        #expect(standing.until == Date(timeIntervalSince1970: 1_796_029_200))
        #expect(standing.trial == Standing.TrialLeft(voiceMSLeft: 540_000, textsLeft: 48))
        #expect(standing.subscribed && standing.canSend)
        #expect(server.requests.first?.path == "/v1/me/subscription")
    }

    @Test func aStateFromANewerServerReadsAsNotSubscribed() async throws {
        server.enqueue(.json(status: 200, body: #"{"required":true,"state":"paused"}"#))
        let standing = try await client.standing()
        #expect(standing.state == .none && !standing.canSend)
    }

    @Test func aPurchaseIsHandedOverAsAppleSignedIt() async throws {
        server.enqueue(.json(status: 200, body: #"{"required":false,"state":"trial","renews":true}"#))
        server.enqueue(.json(status: 200, body: #"{"required":false,"state":"active","renews":true}"#))

        _ = try await client.sendPurchase(signedTransaction: "tx.jws", signedRenewalInfo: "renewal.jws")
        _ = try await client.sendPurchase(signedTransaction: "tx2.jws", signedRenewalInfo: nil)

        let first = try #require(server.requests.first)
        #expect(first.method == "POST" && first.path == "/v1/me/subscription/apple")
        #expect(try JSONSerialization.jsonObject(with: first.body) as? [String: String]
            == ["signed_transaction": "tx.jws", "signed_renewal_info": "renewal.jws"])
        #expect(try JSONSerialization.jsonObject(with: try #require(server.requests.last).body) as? [String: String]
            == ["signed_transaction": "tx2.jws"])
    }

    @Test func aSubscriptionOfAnotherAccountIsRefusedWithItsCode() async throws {
        server.enqueue(.json(status: 409, body: #"{"error":{"code":"subscription_in_use","message":"no"}}"#))
        await #expect {
            _ = try await client.sendPurchase(signedTransaction: "tx.jws", signedRenewalInfo: nil)
        } throws: { error in
            (error as? BackendError)?.code == "subscription_in_use"
        }
    }

    @Test(arguments: [
        (Standing(required: true, state: .none), false),
        (Standing(required: true, state: .expired), false),
        (Standing(required: true, state: .trial), true),
        (Standing(required: true, state: .active), true),
        (Standing(required: true, state: .grace), true),
        (Standing(required: false, state: .none), true), // the server lets everyone send
    ])
    func whoMaySend(standing: Standing, canSend: Bool) {
        #expect(standing.canSend == canSend)
    }

    @Test func thePlanInOneLine() throws {
        let locale = Locale(identifier: "en_GB")
        let until = Date(timeIntervalSince1970: 1_796_029_200) // 30 November 2026
        #expect(planLine(Standing(required: true, state: .none), locale: locale) == "Not subscribed")
        #expect(planLine(Standing(required: true, state: .trial, until: until, renews: true), locale: locale) == "Free week · until 30 Nov")
        #expect(planLine(Standing(required: true, state: .active, until: until, renews: true), locale: locale) == "Subscribed · renews 30 Nov")
        #expect(planLine(Standing(required: true, state: .active, until: until, renews: false), locale: locale) == "Subscribed · ends 30 Nov")
        #expect(planLine(Standing(required: true, state: .grace), locale: locale) == "Payment failed · Apple is retrying")
        #expect(planLine(Standing(required: true, state: .expired), locale: locale) == "Ended")
    }

    @Test func aRefusalForWantOfASubscriptionSaysSo() {
        #expect(RoomViewModel.refusal("subscription_required") == "Start your free week, or subscribe, to send. Your note is kept here to send then.")
        #expect(RoomViewModel.refusal("trial_limit_reached") == "You've sent as much as the free week allows. Your subscription starts when the week ends.")
    }

    @Test(arguments: [
        (nil, Standing(required: true, state: .trial), Subscriptions.Outcome.started(trial: true)),
        (nil, Standing(required: true, state: .active), .started(trial: false)),
        (Subscriptions.Problem.unchecked, Standing(required: true, state: .none), .problem(.unchecked)),
        (Subscriptions.Problem.inUse, nil, .problem(.inUse)),
    ] as [(Subscriptions.Problem?, Standing?, Subscriptions.Outcome)])
    func whatAPurchaseCameTo(problem: Subscriptions.Problem?, standing: Standing?, outcome: Subscriptions.Outcome) {
        #expect(Subscriptions.Outcome.afterPurchase(problem: problem, standing: standing) == outcome)
    }

    @Test func theAlertAfterThePaywallSaysWhatCameOfIt() {
        #expect(Subscriptions.Outcome.started(trial: true).title == "Your free week has started")
        #expect(Subscriptions.Outcome.started(trial: false).title == "You're subscribed")
        #expect(Subscriptions.Outcome.pending.title == "Waiting for approval")
        #expect(Subscriptions.Outcome.failed.title == "That didn't go through")
        #expect(Subscriptions.Outcome.failed.message.contains("nothing was charged"))
        #expect(Subscriptions.Outcome.problem(.unchecked).message == Subscriptions.Problem.unchecked.words)
    }

    /// The StoreKit configuration file sells what App Store Connect sells: one monthly plan, its first week free.
    @Test @MainActor func thePlanIsAMonthWithAFreeWeek() async throws {
        let file = try #require(Bundle(for: TestBundleMarker.self).url(forResource: "Shinar", withExtension: "storekit"))
        let session = try SKTestSession(contentsOf: file)
        session.disableDialogs = true
        defer { session.resetToDefaultState() }

        let products = try await Product.products(for: [Subscriptions.productID])

        let plan = try #require(products.first)
        let subscription = try #require(plan.subscription)
        #expect(plan.displayPrice.contains("4.99"))
        #expect(subscription.subscriptionPeriod == .monthly)
        let offer = try #require(subscription.introductoryOffer)
        #expect(offer.paymentMode == .freeTrial)
        // StoreKit may write the week as 7 days.
        let week = (offer.period.unit == .week && offer.period.value == 1) || (offer.period.unit == .day && offer.period.value == 7)
        #expect(week, "the free trial is \(offer.period)")
    }
}

/// Finds the test bundle, where the StoreKit configuration file is.
private final class TestBundleMarker {}
