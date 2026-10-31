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
