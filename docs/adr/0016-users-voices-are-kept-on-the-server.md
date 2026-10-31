# ADR 0016: Users' voices are kept on the server

Status: accepted, 2026-10-31

## Context

Since milestone 9 each phone records its owner's voice once, by reading a
passage of about 20 seconds. The recording stays on the phone, excluded from
cloud backups, and is sent with each request to speak; the server deletes it
when that job ends. The onboarding screens promise this: the recording stays
on this phone.

In the social app (ADR 0015), the people who hear your words are on other
phones, and may hear them, or a translation of them, when your phone is
off: a text read aloud later, a message spoken again in another language.
The voice has to be available without the phone it was recorded on.

The owner chose to keep voices on the server.

## Decision

**Each user's voice sample is kept on the server, with their consent.**

- It is uploaded once, when the user records it, after a screen that says
  what will happen to it and asks them to agree. The server records when they
  agreed, and to which wording.
- It is used only to speak that user's own words: their messages, and the
  translations of their messages. Never to speak anyone else's words, and it
  is never handed to other users.
- The user can replace it or delete it at any time, and it is deleted with
  their account.
- The voice engine's company receives it with each request to speak, as it
  does now. If the engine chosen at checkpoint 9.4 keeps voices of its own,
  deleting a voice here deletes it there too.
- It is stored apart from other audio, private, and encrypted at rest once the
  server is hosted (milestone 18).

## Alternatives considered

**Keep the voice on the phone, and send it with every message.** The sender
is online when sending a voice note, so the server could make every
translation at that moment from a sample uploaded with the message. It keeps
the old promise, but uploads about 0.6 MB with every message, and it cannot
serve anything that happens later: reading out a text message on request,
a listener who changes language, a message for someone who joins a group
afterwards.

**Keep only the engine's voice ID, not the recording.** Some engines keep a
voice and return an ID. That ties every user to one company: changing engine,
or using a cheaper one for free users, would mean asking everyone to record
again.

## Consequences

- The onboarding wording changes on both phones, and the phones stop being
  the only place the voice lives. The backup exclusion on the phones stays:
  the server copy is the one that counts, not the phone's backup.
- A voice that can be cloned is sensitive personal data, regulated in some
  places as biometric data. Consent, deletion and the privacy policy must say
  exactly what is kept and why (milestone 18).
- Someone could record another person's voice and sign up with it. The
  consent screen asks users to record their own voice, and reporting
  (milestone 16) gives the real owner a way to act.
- One voice per account replaces one voice per phone, which also settles the
  guest problem of milestone 9.
