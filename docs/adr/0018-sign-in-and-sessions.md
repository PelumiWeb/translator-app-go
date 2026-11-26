# ADR 0018: Sign-in and sessions

Status: accepted, 2026-10-31

## Context

In the social app (ADR 0015) the server has to know who is calling: whose
profile to change, whose voice to speak in, which rooms to show. The owner
chose sign-in with Apple, with Google, and with a code sent by email. Sign in
with Apple needs no more than the paid developer account the owner has. Google
needs a Google Cloud project and libraries on Android. Email codes need a
service to send mail from, and a domain to send it from, which the owner does
not have yet.

Until now the server had no accounts: anyone who could reach it could use it.

## Decision

**Users sign in through an identity, are known by an opaque session token,
and the server checks Apple's and Google's tokens itself, with the standard
library.**

- A user can have several identities, each a `(provider, subject)` pair:
  Apple's stable ID for the person, later Google's, or an email address. The
  first sign-in with an identity creates the account; later ones find it.
  Linking a second identity to an account comes when there is a second kind.
- The phone sends the identity token Apple or Google gave it, with a nonce it
  made for this sign-in. The server checks the token completely: signed with
  RS256 by one of the provider's published keys (fetched, kept for a day,
  fetched again at most once a minute when a token names a key it does not
  have), issued by that provider, for this app (the iOS bundle ID for Apple,
  the server's OAuth client ID for Google), not expired, about someone, and
  carrying the SHA-256 of the nonce. Both phones send the provider the hash
  and the server the nonce itself, so one check serves both. The nonce check
  means a token captured on the way cannot be sent again in another sign-in.
- Google sign-in on Android goes through Android's Credential Manager, with
  the libraries approved on 2026-10-31. Google sign-in on iPhone, and Apple
  sign-in on Android, wait: each needs a web sign-in flow, and Apple's needs
  a domain.
- A successful sign-in opens a session: 32 random bytes, written as URL-safe
  text, which the phone keeps and sends as `Authorization: Bearer <token>`.
  The server stores only its SHA-256, so a copy of the database holds no
  working sessions. Each sign-in on each phone is its own session. Sessions
  last until the user signs out or deletes the account; `last_used_at` is
  recorded so idle ones can be ended later if that proves wise.
- Apple tells the phone the person's name only at the first sign-in, and never
  puts it in the token, so the phone sends it alongside; Google's token
  carries it. Email addresses are kept with the identity only when the
  provider has verified them, and never logged; logs carry user IDs only.
- A handle is 3 to 20 lower-case letters, digits, dots and underscores,
  unique, and compared without regard to case.
- The job, speech and model endpoints stay open while the phones do not yet
  sign in, so the current apps keep working. Once both phones sign in, the
  ones that cost money, speech above all, will require a session.

## Alternatives considered

**JSON Web Tokens as sessions.** They can be checked without the database,
but signing out and deleting an account must end sessions at once, which
means a lookup anyway, plus a signing key to keep and rotate. A random token
and one indexed lookup is simpler and enough at this size.

**Using Apple's identity token as the session.** It expires ten minutes after
it is issued, by design.

**A hosted sign-in service** (Firebase Authentication, Auth0, Supabase). It
would bring Google and email codes ready-made, but adds a company every user
depends on, an SDK in each app, and a second place where accounts live. The
part this server needs, checking one kind of signed token, is small.

**A JWT library for Go.** For one algorithm and one issuer, the checks are
about a hundred lines on top of `crypto/rsa`, and they are all tested; a
library would add a dependency to hide them.

## Consequences

- Anyone holding a session token can act as that user until it is ended. The
  phones keep it where the platform keeps secrets: the iOS Keychain, and on
  Android storage encrypted with a key from the Android Keystore.
- Apple asks apps that offer Sign in with Apple to revoke the user's Apple
  tokens when they delete their account. That needs a Sign in with Apple key
  from the developer account and the authorization code from the phone; it is
  needed before the App Store (milestone 18), not before.
- Sign in with Google needs a Google Cloud project with two OAuth clients,
  which the owner creates: a "web" one whose ID the server checks tokens
  against (`GOOGLE_CLIENT_ID`), and an Android one tied to the app's package
  name and signing certificate. Without `GOOGLE_CLIENT_ID` the server refuses
  Google sign-ins with `501`.
- Email codes wait for a domain and a mail service.
- A person who signs in with Apple on the iPhone and with Google on Android
  has two accounts until linking exists. For trying a conversation that is
  useful: two accounts with no second Apple ID.

## Addendum, 2026-11-18: test accounts

The owner asked for a second account to test with. With Sign in with Apple
on the iPhone and Google on Android, one person on one kind of phone has one
account, and the simulator signs in with the same Apple ID as the phone.

Test accounts are email identities (`provider = email`, the address as the
subject) whose code is fixed on the server, in `TEST_ACCOUNTS`
(`address:code` pairs), instead of sent by mail. `POST /v1/sessions/email`
takes the `email` and `code` and opens a session as the other sign-ins do;
the first sign-in makes the account, which then signs up like any other.
Without `TEST_ACCOUNTS` the endpoint answers `501 sign_in_not_configured`; a
wrong pair, or an address that is not a test account, `401 invalid_code`, so
nothing says which addresses exist.

- Codes are at least 12 letters, digits, `-` or `_`, compared in constant
  time. The server has no limit on attempts, so the length is the defence; a
  mistake in `TEST_ACCOUNTS` stops the server from starting. Use addresses at
  `shinar.test`, a name reserved for testing, which no one can own.
- On both phones the entry is hidden: holding "Email sign-in is coming soon."
  on the sign-in screen opens a form for the address and code.
- When codes by email come, the same endpoint serves everyone, with codes
  sent by mail; test addresses keep their fixed codes. The hidden entry then
  becomes the visible email sign-in.
- Before the App Store (milestone 18): App Review may ask for a demo account.
  A test account is one, and its hidden entry must be described in the review
  notes, since Apple's guideline 2.3.1 asks for no undocumented features.

## Addendum, 2026-11-24: revoking Sign in with Apple when an account is deleted

**Context.** Apple asks apps that offer Sign in with Apple to revoke the
user's Apple tokens when they delete their account (Guideline 5.1.1(v)), so
that the person sees the app leave their Apple account's list of apps using
Sign in with Apple. Revoking needs a token Apple gave the app for that
person, which the server did not keep: it checked the identity token and let
it go.

**Decision.**

- At each sign-in with Apple the phone also sends the
  `authorization_code` Apple gave it. The server trades it, once and within
  Apple's five minutes, for a refresh token, with a client secret it signs
  with the Sign in with Apple key (ES256, five minutes long), and keeps that
  token on the identity (migration 0013). A trade that fails does not fail
  the sign-in: the person is signed in either way, and the next sign-in
  brings a new code.
- Deleting an account, in the app or by the owner's command, revokes each
  kept token first. A revocation that fails does not stop the deletion,
  which Apple's guidance allows; it is logged.
- The key is `APPLE_SIGNIN_KEY_FILE` and `APPLE_SIGNIN_KEY_ID`, its team
  `APNS_TEAM_ID`. Without them nothing is traded or revoked.
  `make signin-check` checks the key with Apple: a made-up code must be
  refused as a bad code (`invalid_grant`), which Apple says only once it
  accepts the app's signature.

**Consequences.** An account made before this, and not signed in since, has
no token, and its Sign in with Apple cannot be revoked by the server; the
person can still remove Shinar in their Apple account's settings. The kept
token can do nothing but end the link, so it is not encrypted beyond what the
database is. The privacy pages say it is kept, and why.
