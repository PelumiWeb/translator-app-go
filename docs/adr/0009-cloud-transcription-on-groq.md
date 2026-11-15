# ADR 0009: Cloud transcription with Whisper large-v3 on Groq

Status: accepted, 2026-11-11

## Context

The phones turn speech into text themselves with whisper.cpp and the small
`base` model, and send the recording to the server when they cannot: an
iPhone XS or XR, whose A12 lacks the instructions the Whisper build needs
(ADR 0025); any phone before its model has downloaded; a phone too slow, or
unsure of what it heard (ADR 0007). Since milestone 1 the server answered
those recordings with a fake, "this is a fake transcript from the fake
provider", waiting for this choice (milestone 7, open question 1 of the
plan). With the server hosted and the app on TestFlight (ADRs 0022, 0023),
testers met the fake.

What the choice needs: the app's users are first of all in Nigeria, so
Yoruba and Hausa matter; it should be better than the phone's own model,
since it is the fallback when the phone is unsure; and the owner wants
nothing that costs money while the app is being tested.

## Decision

**Groq, with OpenAI's Whisper large-v3.** `internal/provider/groq` sends the
recording to Groq's OpenAI-compatible `POST /openai/v1/audio/transcriptions`
with `net/http` and `mime/multipart`, so no Go module is added. It asks for
the full `whisper-large-v3`, not the faster turbo model, whose fewer decoder
layers cost most in the languages with the least training data. `GROQ_MODEL`
can change it.

- **The language is given**, as the bare ISO 639-1 code Whisper takes ("pt"
  for "pt-BR"). The phones already know it, and Groq says it improves
  accuracy and latency.
- **What is not speech is not passed on.** Groq answers `verbose_json`,
  which carries each segment's no-speech probability, average log
  probability and compression ratio. The server applies the reference
  Whisper's own rules and thresholds, which the phones apply on device: a
  segment that is probably silence (no-speech probability over 0.6) and
  uncertain (average log probability under -1) is left out; text that
  compresses better than 2.4 repeats itself too much to be speech. A
  recording with no speech left, or only labels such as "[Music]", fails
  the job with "no speech was recognised", as the phone would.
- **Failures:** a rate limit (429) or Groq's own trouble (5xx) is retried
  by the queue; anything else (unreadable audio, a language Whisper does not
  have, a bad key) fails the job at once.
- **Configuration**, like the other engines: a `GROQ_API_KEY` on its own
  chooses Groq; `TRANSCRIBE_ENGINE=fake` keeps the fake, which
  `make server-run-flaky` uses to show retries. Without a key the fake
  stays, so local development needs no key. The server logs the engine it
  chose as it starts.
- `make transcribe-check` sends whisper.cpp's sample clip to Groq once.

## Alternatives considered

**whisper.cpp on our own server.** Free and private, with no new account.
But the Azure machine (ADR 0023) has two cores and 1 GB of memory, enough
for the same `base` model the phones run and not for a better one, so it
would be no better than the phone it stands in for.

**Azure AI Speech**, on the Azure account already open: 5 free hours a
month. Its speech-to-text list has no Yoruba, Hausa or Igbo (checked in
Microsoft's language support table, 2026-11-11), so it fails the first
requirement.

**OpenAI's own transcription API, Google Cloud Speech-to-Text.** Paid from
the first minute, and Google cannot be billed at the moment (ADR 0020,
addendum).

## Consequences

- Recordings the phones cannot transcribe reach Groq. Both phones' privacy
  pages say so.
- Groq's limits for these models are about 20 requests a minute, 2,000 a day
  and 8 hours of audio a day. Its page did not make clear whether those are
  the free plan's; past them, jobs are retried and then fail. A paid plan,
  per second of audio, is the way past them.
- Whisper large-v3 is better than `base` but still weak in Yoruba and Hausa.
  Igbo is not among Whisper's languages at all, on the phone or on Groq.
- The server answers with the whole text at once: Groq does not stream, so
  phones on this route see no words appear as they are recognised.
