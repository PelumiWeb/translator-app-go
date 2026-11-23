# ADR 0034: Paid from the start: a subscription with a free trial, checked by our server

Status: accepted, 2026-11-23

## Context

On 2026-11-21 the owner decided how Shinar goes to the App Store:

- **No free use.** A new person gets a free trial, with a cap of its own,
  then subscribes to keep sending.
- **One subscription, $4.99 a month**, with **Apple's free trial of 7 days**.
- **Only senders pay.** Reading and hearing what one is sent costs nothing
  more: the sender's subscription paid for it when it was sent. So the
  grandmother who is sent a note can hear it before she pays.
- **The iPhone first**, through Apple's In-App Purchase, which Apple
  requires for anything used inside an app (Guideline 3.1.1). Google Play
  comes with milestone 22.
- **Our own server checks purchases**, with no payment service such as
  RevenueCat in between.
- **No calls** (milestone 17 is dropped for now): what is sold is what the
  app does today.

On 2026-11-22 the owner chose to publish through a company of their own,
once it exists, so the Apple account will become an organisation account.
That changes who signs Apple's agreements, not this design.

A voice note between two languages costs about 2 cents a minute in the
engines. A subscriber at $4.99 brings $4.24 after Apple's 15%.

## Decision

**Apple's introductory offer is the trial.** The person starts the
subscription on the paywall, confirms with their Apple account, and pays
nothing for 7 days; it becomes the monthly plan unless they cancel. Apple
gives each Apple account one trial, so a new Shinar account does not bring a
second one, which a trial kept by our server could not prevent.

**The server believes only what Apple signed.** Each record the App Store
makes (a transaction, renewal information, a server notification) is a JWS
whose header carries its certificate chain. `internal/billing` checks, with
the standard library:

- the chain ends at Apple Root CA - G3, kept in the program and pinned by its
  SHA-256 in a test;
- the leaf carries Apple's mark for the App Store's signer, and the
  intermediate Apple's mark for its developer authority, as Apple's own
  libraries check;
- the chain as of the record's signing date, as Apple's libraries do, so a
  record does not stop verifying when its certificate later expires;
- the ES256 signature, and only ES256;
- the bundle ID is the app's.

**Two ways in, both checked the same.** The phone sends each transaction
StoreKit gives it, after a purchase, a restore, and when the app opens
(`POST /v1/me/subscription/apple`). Apple sends server notifications, version
2 (`POST /v1/billing/apple/notifications`), with no sign-in: the signature is
the proof. Every kind of notification is handled alike, by applying the
transaction and renewal information inside, so a kind Apple adds later needs
nothing new. Each notification is kept once, as it came.

**One row per subscription,** as Apple counts them: an original transaction
and its renewals. Records arrive out of order, from two directions, so a
transaction is applied only when it reaches further than the one kept, or is
the same one again (now refunded, say), and renewal information only when it
is newer.

**One subscription, one account.** The phone buys with the user's ID as
`appAccountToken`. A purchase naming another account is refused. A
subscription already tied to another account that exists is refused
(`409 subscription_in_use`). One whose account was deleted, which Apple goes
on billing, moves to the account that restores it.

**Where someone stands** is worked out from their subscriptions at the
moment of asking: `trial` or `active` until the period ends; `grace` while
Apple retries a failed payment, until the grace period ends; otherwise
`expired`, or `none` for someone who never subscribed. A refund ends it at
once. The trial, the paid period and the grace period may all send.

**What needs a subscription:** sending a voice note or a message, and the
interpreter's work on the server (`/v1/jobs`, `/v1/speech`), refused with
`402 subscription_required`. Nothing of a refused message is kept, and a
voice note's recording is not even stored. In the trial, the trial's cap
(`TRIAL_VOICE_MINUTES`, `TRIAL_TEXT_MESSAGES`; proposed 10 and 50) is
counted from the trial's start, in place of the daily limits; past it,
`429 trial_limit_reached`. Subscribers keep the daily limits (ADR 0029).

**A switch, and the purchases recorded either way.** `BILLING=on` makes
sending need a subscription. It stays off on the hosted server until the
iPhone build with the paywall reaches testers, so the builds on their phones
keep working. The server takes and records purchases with the switch off, so
a subscription bought on TestFlight is known when it goes on.
`GET /v1/me/subscription` says `"required"`, so the phone shows the paywall
only when it matters.

**Sandbox purchases count,** marked as sandbox: App Review and TestFlight
buy there, at no cost.

## Alternatives considered

**RevenueCat**, or another service in between. It handles both stores and
gives dashboards, and is free up to $2,500 a month in revenue, then 1%. But
it adds an SDK to the phones and a service to trust with every purchase,
while the server must know who has paid anyway, since it enforces the
limits. Checking Apple's records is a few hundred lines of the standard
library, the same kind of code the server already has for Apple's push
service and Sign in with Apple.

**A trial run by our server**, without payment details. Easier to start, and
no card at the door. But anyone could start again with a new account; Apple
allows one trial per Apple account.

**Asking the App Store Server API for each subscription's status** on every
check, or on a timer. It catches a missed notification at once. But the phone
reports what StoreKit gives it every time the app opens and whenever it
changes, and Apple sends each notification again when it is not sure it
arrived; someone sending is, by then, someone with the app open. The client
for the API exists for `make appstore-check` and can take this job later.

**Everyone pays, receivers too.** Simpler to say, but a room needs two
people, and the person first sent a note would hit a paywall before hearing
it. The owner chose senders only.

## Consequences

- Before the first sale: the Paid Applications Agreement, the Small Business
  Program, the EU trader status, and the conversion to the owner's company's
  account (checkpoint 21.1). Sandbox purchases on a real iPhone wait for the
  agreement; the StoreKit configuration file in Xcode does not.
- Checked against Apple, not only against chains made for tests: on
  2026-11-23 `make appstore-check` asked Apple's sandbox for a test
  notification with the In-App Purchase key, and the verifier accepted
  Apple's own signature.
- The interpreter's work on the server is not counted against a cap, as it
  was not before (ADR 0029). The monthly budget per engine (checkpoint 18.12)
  bounds it, with everything else.
- A resend, after a lost response, of a note kept just before the trial
  ended is refused with `402` though the note was kept. It shows on its card
  as failed until the room reloads.
- At 10 minutes a day, a subscriber who uses all of it costs about $6 a month,
  more than they bring. The average will be far below; the limits are
  settings, and the budget is the backstop.
- Android has no paywall yet. With `BILLING=on`, an Android phone can no
  longer send. Android is not offered to anyone until milestone 22.
