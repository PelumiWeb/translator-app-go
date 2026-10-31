# ADR 0010: Voice-cloning engine

Status: accepted, 2026-10-31

## Context

Since checkpoint 9.3 both phones send the user's voice sample and the
translation to `POST /v1/speech`, and the server speaks it through a
`provider.Synthesizer`. Until now the only one was a fake that answers with
beeps. The plan was to try a free public demo of an open model first; the app
became a product on 2026-10-31 (ADR 0015), so the owner chose a paid service
straight away.

What the product needs from an engine:

- Speak a language other than the one in the sample, in the sample's voice:
  an English speaker's voice, saying Spanish.
- Many languages, since anyone may talk to anyone.
- Every user has their own voice. An engine that keeps voices on its side,
  with a cap on how many, caps the number of users who can have one.
- Cheap per message: every voice note pays for a synthesis.

Prices and limits below come from third-party summaries read on 2026-10-31,
which disagree with each other in places.

## Decision

**Fish Audio** (docs.fish.audio), its `POST /v1/tts`, with the voice sample
sent inline with each request.

- Each request carries the sample and its transcript. The transcript is
  known: the passage the phones ask users to read
  (`provider.OnboardingPassage`). Fish Audio keeps no voice of ours, so there
  is no cap on users with voices, and changing engine never means asking
  anyone to record again.
- About $15 per million characters, a fraction of a cent per voice note, and
  80+ languages, speaking any of them in the sample's voice.
- Until the owner adds API credit, the server uses the free model,
  `s2.1-pro-free` (`FISH_AUDIO_MODEL` in `server/.env`). Deleting that line
  switches to the default paid model.
- The audio is asked for as raw 16 kHz PCM and given a WAV header on the
  server: the format the phones already play.

**No Go dependency.** The call is `net/http`. Fish Audio takes an inline
sample only as MessagePack, not JSON, so a small encoder (`fish/msgpack.go`,
tested against the specification's bytes) writes the one request we send.

**Configuration.** The key is `FISH_AUDIO_API_KEY` in `server/.env`, which
git ignores and the Makefile loads. A key on its own chooses Fish Audio;
`VOICE_ENGINE=fake` brings back the beeps, and no engine at all makes the
server refuse speech with 501, so the phones use their own voices.

## Alternatives considered

**ElevenLabs.** The best-known quality, and cross-lingual. It keeps each
voice on its side and caps how many per plan (one source: 10 on its $6
Starter plan, up to 2,200 on its $990 Business plan), and costs several times
more per character. The voice cap is the problem for a social app.

**Cartesia.** From $5 a month, 42 to 44 languages. Keeps voices on its side;
its limits were unclear.

**MiniMax.** $60 to $100 per million characters, plus $1.50 for each user's
voice, 40 languages.

**A public demo of an open model** (the original plan). Free, but slow,
shared, liable to change, and recordings go to a public demo: not for a
product.

**Hosting an open model ourselves.** No per-character cost, but a GPU server
to run, and the licences of the best open models forbid commercial use.

## Consequences

- Users' recordings, and the texts they translate, go to Fish Audio with
  every request. The onboarding screens say so, on both phones. Whether Fish
  Audio keeps or trains on them must be checked in their terms before going
  live (milestone 18).
- Each synthesis takes a few seconds (2.7 s of speech took 4.4 s in the
  first live test), so the phone waits after the text appears.
- `make voice-check` makes one real request and plays it, to check the key
  and the engine.
- The engine is still chosen per request, so a cheaper or better one can be
  added later, for example for free and paying users.
