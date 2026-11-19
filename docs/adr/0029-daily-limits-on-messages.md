# ADR 0029: Daily limits on what each person sends

Status: accepted, 2026-11-19

## Context

On 2026-11-15 the owner asked for usage limits before the app is opened to
outside people, so the engines behind each message cannot run up a bill or
run out. On 2026-11-19 they asked again, ahead of a soft launch to TestFlight
testers, and decided:

- **10 minutes of voice notes and 100 text messages a day**, for each person.
- **Stay on the free tiers** during the soft launch: Azure AI Translator's
  F0 (2 million characters a month for the whole server), Fish Audio's free
  model, and Groq's free limits. Nothing is billed.

Every message that crosses a language costs a translation, and a voice note
also costs a synthesis in the sender's voice, once per listener language
(ADR 0020). On paid tiers that is about 2 cents a minute of voice note. On
the free tiers it costs nothing, but the allowance is shared: 2 million
characters is about 5,000 translated 30-second notes a month, for everyone.
Without a limit, one person, or a bug in a phone, could use it up in a day,
and translation would stop for everyone until the next month.

## Decision

**The server keeps the limits.** A signed-in person can call the API
without the app, so the phones cannot be the ones to enforce it.
`POST /v1/rooms/{id}/messages` refuses a message past the limit with `429`
and `voice_limit_reached` or `text_limit_reached`, and nothing of it is kept:
no row, no recording, no work queued.

**Counted from the messages themselves, over the last 24 hours.** The total
length of a person's voice notes, and the number of their text messages, sent
in the 24 hours before this one. No counter table to keep in step: the
messages are the record (an index, migration 0009, keeps the sum a short
read). A rolling day rather than a calendar one, so the limit means the same
in Lagos, Madrid and Tokyo, and there is no midnight to explain.

**Checked in the transaction that adds the message.** The sender's `users`
row is locked first, so two messages sent at once take turns and cannot both
slip under the limit. The check counts the new message after it is inserted,
so a resend with the same idempotency key returns the kept message and is
never refused.

**The limits are settings,** `DAILY_VOICE_MINUTES` (10) and
`DAILY_TEXT_MESSAGES` (100), so the owner can change them without a release.

**The phones say why.** Both rooms show "You've sent as many voice notes as
one day allows. You can send more later, or write a message." (or the same
for messages), and the note stays on its card as failed, to send again
later. `429` is not retried by either phone.

**No budget of our own for the whole server, while on the free tiers.** The
free tiers stop by themselves when their month is used up, without charging:
Azure stops translating, and Fish Audio's and Groq's limits refuse
requests. The per-person limits keep any one person from using the shared
allowance up.

## Alternatives considered

**A counter table per person and day.** One upsert per message, with the
limit in its `WHERE`. It would also count work that leaves no message. But
it would be a second record of the same facts, it needs a calendar day and
so a time zone, and today every counted thing is a message.

**Limits on the phones only.** Simpler, and the phones need to show it
anyway. Not enough: the API is reachable without the app.

**Characters instead of minutes and messages.** Closer to what the engines
charge for, but nobody thinks in characters. A 30-second note is about 400.

## Consequences

- A person who reaches the limit can still use the other kind (voice notes
  or text), and the interpreter, which translates on the phone.
- **Not limited yet:** the interpreter's work on the server, transcription on
  Groq (`POST /v1/jobs`) and speech in the user's voice (`POST /v1/speech`).
  Jobs do not record who asked for them. Groq's and Fish Audio's free limits
  cap them for now, but one person could use those up for others. To do
  before the app is public.
- Listening to a text message in its sender's voice is not counted: it is
  made at most once per language per message, so the text limit bounds it.
- **Before moving to paid tiers, a monthly budget per engine** for the whole
  server is needed, past which it stops calling that engine, as the plan
  says. On a paid tier nothing else stops the bill.
- The phones do not show what is left, and recording does not stop at the
  limit, only sending is refused. Where to show it is not designed.
- Voice notes stay at 30 seconds. A minute, the owner's earlier example,
  needs the phone's transcriber to handle two Whisper windows.
