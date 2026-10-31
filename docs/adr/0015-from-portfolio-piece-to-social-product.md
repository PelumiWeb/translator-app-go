# ADR 0015: From a portfolio piece to a social product

Status: accepted, 2026-10-31

## Context

The app was started as a portfolio piece, meant to show native mobile apps,
on-device inference and Go backend services. By the end of October 2026 it
records speech on Android and iPhone, transcribes it on the phone or the
server, translates it, and speaks the translation in the speaker's own voice
through the server.

On 2026-10-31 the owner proposed more: a social app where anyone can talk to
anyone without sharing a language. People sign up with their name, their
voice and a few details, and can be reached by an ID. Each speaks their own
language; the other side hears it in theirs, in the speaker's voice. Voice
notes first, then text, then calls, which cost the most and would be for
premium users only.

Asked whether this is a product or a portfolio piece, the owner answered:
a product.

## Decision

**The app becomes a product: a social app for talking across languages.**
It is built in this order, each a milestone:

1. A real voice engine (checkpoint 9.4), since everything depends on the
   voice sounding like its owner.
2. Accounts, a profile, and each user's voice kept on the server (13).
3. Voice notes between people (14).
4. Text messages (15).
5. Safety (block, report) and discovery, including finding people at random
   (16).
6. Going live: hosting, policies, the app stores (18).
7. Calls, for premium users, with billing (17).

Voice keyboards (11) and the real transcription provider (7) come after.
Milestone 12, which brought voice notes in from other apps through the share
sheet, is dropped: voice notes now live inside the app.

**Accounts and sign-in become goals.** Sign-in by a code sent to an email
address, Sign in with Apple, or Sign in with Google. Billing stays out until
calls bring a premium plan.

**What has been built stays.** The interpreter screen, the pipeline and its
seams, on-device transcription, the job queue and its events are all reused:
a voice note is the same transcribe, translate, speak, between two people.

## Consequences

- Two earlier decisions are reversed, each with its own ADR: voices are kept
  on the server (ADR 0016), and messages are translated on the server
  (ADR 0017).
- A product carries obligations a portfolio piece does not: a privacy policy
  and terms, the safety features both app stores require of apps where people
  contact each other, and handling sensitive data, a person's voice above
  all.
- Running costs grow with use: each voice note pays for a translation and a
  synthesis. The engine is chosen per request, so free and paying users can
  later get different engines.
- The server has to be hosted publicly before the first outside users
  (milestone 18).
- CLAUDE.md's description and non-goals, and the plan's order, change to
  match.
