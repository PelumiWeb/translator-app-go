import StoreKit
import SwiftUI

/// The paywall: Apple's own subscription view for the one plan, which shows the price, the free week and how it
/// renews in the person's currency, with Restore and the terms, as App Review asks (Guideline 3.1.2). Shinar's words
/// go above it. Not designed yet: built from Pearl's type and colours.
struct PaywallSheet: View {
    let subscriptions: Subscriptions

    @Environment(\.dismiss) private var dismiss

    var body: some View {
        SubscriptionStoreView(productIDs: [Subscriptions.productID]) {
            PaywallHeader(problem: subscriptions.problem)
        }
        .subscriptionStoreControlStyle(.buttons)
        .storeButton(.visible, for: .restorePurchases)
        .storeButton(.visible, for: .policies)
        // The close button: "Not now". Reading and hearing what one is sent needs no subscription.
        .storeButton(.visible, for: .cancellation)
        .subscriptionStorePolicyDestination(url: Brand.website.appending(path: "terms"), for: .termsOfService)
        .subscriptionStorePolicyDestination(url: Brand.website.appending(path: "privacy"), for: .privacyPolicy)
        .inAppPurchaseOptions { [subscriptions] _ in subscriptions.purchaseOptions }
        .onInAppPurchaseCompletion { _, result in
            switch result {
            case .success(let purchase): await subscriptions.purchased(purchase)
            case .failure: subscriptions.purchaseFailed()
            }
            // Closed once something came of it, good or bad; the screen underneath says which. Backing out of
            // Apple's sheet leaves no outcome, and the paywall open.
            if subscriptions.outcome != nil { dismiss() }
        }
        .onChange(of: subscriptions.standing?.subscribed) { _, subscribed in
            // Restored, or bought on another device meanwhile.
            if subscribed == true { dismiss() }
        }
        .containerBackground(Palette.bg, for: .subscriptionStore)
        .tint(Palette.you)
        .preferredColorScheme(.dark)
    }
}

private struct PaywallHeader: View {
    let problem: Subscriptions.Problem?

    var body: some View {
        VStack(spacing: 14) {
            Text(Brand.name).font(.pearlScreenTitle).foregroundStyle(Palette.fg)
            Text("Be heard in your own voice").font(.pearlLead).foregroundStyle(Palette.them)
            VStack(spacing: 8) {
                Text("Send voice notes and messages to anyone. They hear them in their language, in your voice.")
                // The cap is the server's (TRIAL_VOICE_MINUTES and TRIAL_TEXT_MESSAGES); change the two together.
                Text("Your free week includes 10 minutes of voice notes and 50 messages.")
                Text("Reading and hearing what you're sent is always free.")
            }
            .font(.pearlMessage).foregroundStyle(Palette.fg2)
            .multilineTextAlignment(.center)
            if let problem {
                Notice(text: problem.words)
            }
        }
        .padding(.horizontal, 24)
        .padding(.top, 32)
    }
}

extension Subscriptions.Problem {
    var words: String {
        switch self {
        case .inUse: "This Apple account's subscription belongs to another \(Brand.name) account. Sign in to that one, or use another Apple account."
        case .notYours: "That purchase was made for another \(Brand.name) account."
        case .unchecked: "Your purchase couldn't be checked with Apple just now. It will be tried again."
        case .offline: "No connection. Your purchase is safe, and will reach \(Brand.name) when you're back online."
        }
    }
}

extension View {
    /// The paywall as a sheet, and once it closes after a purchase, an alert saying what came of it.
    func paywall(isPresented: Binding<Bool>, subscriptions: Subscriptions) -> some View {
        modifier(PaywallPresentation(isPresented: isPresented, subscriptions: subscriptions))
    }
}

private struct PaywallPresentation: ViewModifier {
    @Binding var isPresented: Bool
    let subscriptions: Subscriptions
    @State private var outcome: Subscriptions.Outcome?

    func body(content: Content) -> some View {
        content
            // The alert waits for the sheet to be gone: SwiftUI shows one presentation at a time.
            .sheet(isPresented: $isPresented, onDismiss: { outcome = subscriptions.takeOutcome() }) {
                PaywallSheet(subscriptions: subscriptions)
            }
            .alert(outcome?.title ?? "", isPresented: Binding { outcome != nil } set: { if !$0 { outcome = nil } }, presenting: outcome) { _ in
                Button("OK", role: .cancel) {}
            } message: { outcome in
                Text(outcome.message)
            }
    }
}

extension Subscriptions.Outcome {
    var title: String {
        switch self {
        case .started(let trial): trial ? "Your free week has started" : "You're subscribed"
        case .pending: "Waiting for approval"
        case .problem, .failed: "That didn't go through"
        }
    }

    var message: String {
        switch self {
        case .started(let trial):
            trial
                ? "Send voice notes and messages in your voice. Unless you cancel, it becomes the monthly subscription when the week ends."
                : "Thank you. Send voice notes and messages in your voice, to anyone."
        case .pending: "Your purchase is waiting for someone to approve it. It starts as soon as they do."
        case .problem(let problem): problem.words
        case .failed: "The purchase couldn't be completed, and nothing was charged. Please try again."
        }
    }
}

/// The interpreter, for someone with no subscription: what it does, and the way in.
struct LockedInterpreter: View {
    let onSeePlan: () -> Void

    var body: some View {
        VStack(spacing: 18) {
            Spacer()
            Image(systemName: "arrow.left.arrow.right").font(.system(size: 34)).foregroundStyle(Palette.them)
            Text("The interpreter").font(.pearlScreenTitle).foregroundStyle(Palette.fg)
            Text("Two people, one phone, each speaking their own language. It comes with your free week, and with the subscription after it.")
                .font(.pearlMessage).foregroundStyle(Palette.fg2).multilineTextAlignment(.center)
            Button("Start your free week", action: onSeePlan).buttonStyle(PrimaryButtonStyle())
            Spacer()
        }
        .padding(.horizontal, 32)
        .frame(maxWidth: .infinity, maxHeight: .infinity)
        .background(Palette.bg)
    }
}

/// The plan, as Profile shows it in one line.
func planLine(_ standing: Standing, now: Date = .now, locale: Locale = .current) -> String {
    let day = { (date: Date) in date.formatted(Date.FormatStyle(locale: locale).day().month(.abbreviated)) }
    switch standing.state {
    case .none: return "Not subscribed"
    case .trial: return standing.until.map { "Free week · until \(day($0))" } ?? "Free week"
    case .active:
        guard let until = standing.until else { return "Subscribed" }
        return standing.renews ? "Subscribed · renews \(day(until))" : "Subscribed · ends \(day(until))"
    case .grace: return "Payment failed · Apple is retrying"
    case .expired: return "Ended"
    }
}
