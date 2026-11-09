# ADR 0020: Rooms, message delivery, and Google Cloud Translation

Status: accepted, 2026-10-31; addendum 2026-11-09 (Azure AI Translator)

## Context

Milestone 14 lets two people talk in voice notes. ADR 0017 decided that
messages are translated on the server and left the service to choose. On
2026-10-31 the owner chose Google Cloud Translation, from three options:
Google, Claude through the Anthropic API, or a stand-in until later. The
owner also chose that a room opens at once, with no request to accept.

A voice note has to become, for each listener, their language spoken in the
sender's voice. Each step can fail or be slow: the translation service, the
voice engine (Fish Audio, ADR 0010), the network. The phones must show where
a note is (the design's "translating", "preparing your voice", "delivered")
and update without the person refreshing.

## Decision

**A room of two is found by its pair of users.** `rooms.pair` holds the two
user IDs in order, unique, so the same two people always share one room, and
two people adding each other at once still make one. A room opens at once.
Until its first message only the person who opened it sees it in their list.

**The phone writes down what was said.** It sends the recording with its
words and its length. The server's own transcription is still a stand-in
(milestone 7), and the phones already transcribe well.

**One delivery job per message and listener language** runs on the existing
queue (ADR 0001), with its leases, retries and fencing. It translates, saves
the translation, then speaks it in the sender's kept voice (ADR 0016) and
saves the audio. Because the translation is saved first, a retry after the
voice engine fails does not translate again. ADR 0017 imagined a translation
job followed by a synthesis job; one job with saved progress does the same
without a hand-off between two jobs.

What each listener gets is a **rendition** (`renditions`, one per message and
language):

- listeners who speak the language the note was said in get the sender's own
  recording at once, with no job;
- when the sender keeps no voice, the server has no voice engine, or the voice
  fails for good after the translation was made, the rendition is ready with
  the words only, and the listener's phone reads them in one of its voices
  (`voice = phone`), as the phones already fall back today;
- when the translation fails for good, the rendition is `failed`, the sender's
  words stay on their card, and they can ask for it to be tried again.

**Google Cloud Translation, basic edition (v2 REST),** called with `net/http`
and an API key sent in a header, so the key never sits in a logged URL. No Go
module. `GOOGLE_TRANSLATE_KEY` turns it on; `TRANSLATE_ENGINE=fake` marks text
with its language instead, for development without a key.

**Live updates are hints, not history.** Every change to a room is announced
with Postgres `NOTIFY` on a channel of its own, in the transaction that makes
it, naming the room, the message and the members. Each API process passes it
to the streams of those members (`GET /v1/me/events`), one stream per phone.
Nothing is stored or replayed: the tables are the state, so a phone fetches
what it shows when the stream says it is ready, and again when told something
changed. Job events (ADR 0002) needed replay because a job's progress existed
only as its events; a room's does not.

**Deleting an account deletes what it said**: its messages, their recordings,
and the copies spoken in its voice. The room stays for the other person, who
can no longer send to it.

## Alternatives considered

**Claude, through the Anthropic API.** Translations that read the way people
talk, but slower per message than a translation API and a second paid
provider. The owner chose Google.

**A stand-in translator for now.** It would have left rooms untestable
between two languages.

**A translation job, then a synthesis job** (as ADR 0017 put it). Two jobs per
language, and a hand-off that can itself fail.

**Replaying room events, like job events.** A stored, numbered history per
user, kept in order across concurrent writers, to save one fetch after a
reconnect. Not worth its locking.

**The listener's phone translates** (rejected in ADR 0017): the translation
would then have to go back to the server to be spoken in the sender's voice.

## Consequences

- A Google Cloud API key, restricted to the Translation API, in `server/.env`.
  The first 500,000 characters a month are free, then $20 per million; a
  30-second note is about 400 characters, per listener language.
- Google sees the text of every message it translates. The privacy page in
  the apps says so, and the privacy policy must (milestone 18).
- The server holds every message's recording, words and spoken copies until
  the sender deletes their account. Deleting only the voice stops new copies
  being made in it; copies already made stay with their messages.
- A worker that dies holding a delivery's last attempt leaves its rendition
  "translating": the lease sweep fails the job without telling the rooms
  package. Rare, and the sender can see it never arrived; to fix with the
  sweep.
- Group rooms (milestone 20) fit as they are: one rendition and one job per
  language in the room, not per member.

## Addendum, 2026-11-09: Azure AI Translator, for now

Google Cloud Translation could not be turned on: the Google payments
profile holds an expired card, and Google refuses new cards until it is
fixed. The owner chose Microsoft's Azure AI Translator in its place, on the
free F0 tier: 2 million characters a month, about 130 languages including
Yoruba, Igbo and Hausa, and quality on a par with Google. On F0, Azure stops
translating for the rest of the month when the allowance is used up rather
than charging; the paid tier is about $10 per million characters.

The server keeps both engines. A key on its own chooses its engine
(`AZURE_TRANSLATOR_KEY` with `AZURE_TRANSLATOR_REGION`, or
`GOOGLE_TRANSLATE_KEY`); with both, `TRANSLATE_ENGINE` says which. Azure's
key goes in a header, like Google's. Six of the phones' language codes are
spelled differently by Azure and are mapped (Chinese, Norwegian and Nynorsk,
Serbian, Mongolian, Tagalog); nine of the phones' languages Azure does not
have (Latin, Breton, Occitan, Tajik, Yiddish, Sanskrit, Hawaiian, Javanese,
Sundanese), and a message into or out of one fails to translate, as it would
with any service that lacks it. The free month running out fails the same
way, to be retried the next month.

Microsoft, not Google, now sees the text of each message it translates; the
privacy page and policy must say so.
