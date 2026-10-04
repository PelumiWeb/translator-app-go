# ADR 0003: Translation stays on the device on both paths

Status: accepted, 2026-10-04

## Context

Speech can be transcribed in two places: on the device with whisper.cpp, or by
the backend when the device cannot (no model, too slow, low confidence). The
transcript then has to be translated into the receiver's language.

The original spec did not say where translation happens when transcription
falls back to the backend. Either the backend translates as well, or it returns
a transcript and the device translates as it always does.

## Decision

The backend only transcribes. `POST /v1/jobs` takes audio and a source
language, and the `done` event carries the transcript and its language.
Translation is always done on the device by ML Kit, behind the `Translator`
interface, whichever path produced the transcript.

## Alternatives considered

**Translate on the backend during fallback.** The backend would need a
translation provider of its own (a second paid API or a hosted model), a target
language on every job, and a second result field. There would be two
translation implementations whose output differs for the same sentence,
depending on which path a recording happened to take.

**Translate on the backend always.** Removes on-device translation, which
works offline and is part of what this project sets out to demonstrate.

## Consequences

- One translation code path to build and test.
- The backend API stays small and has no notion of a target language.
- The fallback path still needs the ML Kit language pack on the device. A
  device with no pack and no network cannot translate; that is true of the
  on-device path as well.
- Translation quality is ML Kit's on both paths: about 59 languages, and
  pivoting through English when neither side is English. If that proves too
  weak, a cloud `Translator` can be added behind the same interface without
  changing the backend's job API.
