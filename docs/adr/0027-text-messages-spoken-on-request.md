# ADR 0027: Text messages, spoken on request

Status: accepted, 2026-11-18

## Context

Milestone 15 adds text messages: "type in English; the other side reads it
in Spanish, and can tap it to hear it in the sender's voice." On 2026-11-18
the owner asked for them, so people can write when speaking is not
comfortable.

A voice note is translated, then spoken in the sender's voice, before the
listener gets it (ADR 0020): hearing is the point of a voice note. A text
message is there to be read. Speaking every one in the sender's voice would
make the listener wait for the voice before reading, and would pay Fish Audio
for every message, whether or not anyone listens. Text messages come more
often than voice notes, and the usage limits asked for on 2026-11-15 are not
built yet.

## Decision

**A message has a kind: `voice` or `text`.** A text message is a row in the
same `messages` table, with no recording: its words are the transcript and
its length is 0 (migration 0008). It goes to the same endpoint,
`POST /v1/rooms/{id}/messages`, as JSON (`text`, `lang`) instead of a
multipart form, with the same idempotency key. One table and one endpoint
keep a room one list, paged and pushed the same way.

**Its words are delivered as soon as they are translated.** The same delivery
job translates it, then makes the rendition ready with the words and no
voice. Listeners of the language it was typed in get it at once, with no job.
The listener's phone is told then, as for a voice note: "Sent a message",
never the words.

**Its voice is made when someone asks.** `POST /v1/messages/{id}/speech?lang=`
(the listener, for their language; the sender, for any of the room's) moves
the rendition to `speaking` and adds a second delivery job, which speaks the
saved translation in the sender's kept voice, exactly as for a voice note.
The voice is made once per message and language, then kept; asking again
while it is being made, or after, changes nothing. When the sender keeps no
voice, or the voice fails for good, the rendition becomes one for the
listener's phone to read, as for voice notes. A voice made on request is not
pushed again: the listener has read the message already.

**The phones** show the words first, with a listen button. The first tap asks
for the voice; the button breathes while it is made, and the message plays as
soon as the room's stream says it is ready.

## Alternatives considered

**Speak every text message at once, like a voice note.** No wait on the first
listen, but a Fish Audio charge for every message, and either a wait before
reading or a second state for "words ready, voice coming". Rejected for the
cost while there are no usage limits.

**Only the phone's voice for text messages.** Free, but loses the product's
point, the sender's own voice. It remains the fallback.

**A separate endpoint and table for text.** Two lists to merge on the phones,
two kinds of push, two delete paths, for no gain.

## Consequences

- The first listen to a text message waits a few seconds for the voice; later
  listens, by anyone in the room, do not.
- Fish Audio is paid only for text messages someone listens to, at most once
  per message and language.
- Phones built before this read a text message as a voice note with no
  recording: they show the words and read them with the phone's voice.
- Deleting an account deletes text messages with the rest of what it said;
  they have no recording, only a voice if one was made.
- The usage limits, when they come, count characters translated and spoken on
  request the same way as for voice notes.
