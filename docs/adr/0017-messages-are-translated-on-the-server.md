# ADR 0017: Messages are translated on the server

Status: accepted, 2026-10-31

## Context

ADR 0003 kept translation on the phone, on both transcription routes: the
server only transcribed, there was one translation path to test, and
translation cost nothing. Android translates with ML Kit (59 languages),
the iPhone with Apple's Translation framework (22 languages, and none in the
simulator, ADR 0012).

In the social app (ADR 0015) a voice note is spoken to the listener in the
sender's voice, and the voice engine runs on the server. Whatever is spoken
must be translated before it reaches the engine.

## Decision

**Messages between people are translated on the server**, into each
listener's language, as a job on the queue before the synthesis job. The
translation service is chosen at milestone 14.

**The interpreter screen keeps translating on the phone.** ADR 0003 still
holds there: it works offline, and costs nothing.

## Alternatives considered

**The sender's phone translates.** It knows the listener's language from
their profile, and the server would only speak the result. But what a
message can be translated into would depend on the sender's phone: an iPhone
cannot translate into Yoruba, an Android phone can. Two people who can talk
from one phone could not from another. A message to several people would be
translated several times on the sender's phone.

**The listener's phone translates.** The translation would then have to go
back to the server to be spoken in the sender's voice, a round trip for every
message, with the same gaps between the phones' translators.

## Consequences

- One translator for every message, whatever phones the two people have,
  covering the languages the chosen service covers.
- Every message pays for a translation, and the service is a new dependency,
  to be approved at milestone 14.
- There are now two translation paths: the phone's, for the interpreter
  screen, and the server's, for messages. They may translate the same
  sentence differently.
- The server sees the text of every message. The privacy policy has to say
  so (milestone 18).
