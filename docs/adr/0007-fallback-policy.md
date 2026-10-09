# ADR 0007: When to transcribe on the device and when in the cloud

Status: accepted, 2026-10-09. The thresholds are provisional; see "What is not
known yet".

## Context

The app can transcribe in two places. On the device it is private, free,
immediate and works offline, but the model is small and some phones are slow.
In the cloud it is more accurate and independent of the phone, but needs a
network, costs money per request with a real provider, and sends the user's
voice off the device.

Until now a switch on screen chose between them. The user should not have to
know which is better for this recording on this phone.

## Decision

A third route, `AUTO`, prefers the device and falls back to the cloud. The two
manual routes stay, for demonstrations and for testing each path alone.

`AUTO` goes to the cloud **without trying the device** when:

| Condition | How it is known |
| --- | --- |
| No model is installed | the model manager's state is not `Ready` |
| The device is too slow | its measured real-time factor is above 1.0 |

`AUTO` goes to the cloud **after trying the device** when:

| Condition | How it is known |
| --- | --- |
| The result is not confident | mean token probability below 0.6 |
| Sound but no words | the model's no-speech probability above 0.6, or an empty result |
| Repeated nonsense | the text compresses by more than 2.4 times (a repetition loop) |
| The model failed to run | the transcriber raised an error |

A recording that is silent (below about -50 dB) is rejected before any model
runs and is never uploaded. No transcriber can do better with it.

**The cloud is a fallback, not a requirement.** Whenever the cloud cannot be
reached and the device has, or can produce, a result, that result is used and
marked as such:

- A low-confidence device result is kept if the upload fails.
- A device judged too slow is used after all if the cloud is unreachable.

Only when there is no device result at all does a cloud failure become an
error, and then the device's own verdict ("no speech was recognised") is
reported in preference to the network error.

**The doubtful result is shown while the cloud works.** A low-confidence
transcript is emitted as a partial, so the user sees text within the device's
latency and it is replaced when the cloud answers.

**Every result says how it was routed.** `Transcript.note` records why the
first choice was not used, and the screen shows it.

The numbers live in one object, `FallbackThresholds`, and the whole policy in
one class, `RoutingTranscriber`, behind the same `Transcriber` interface as the
two engines. Nothing else in the app knows a decision was made.

**Measuring speed.** Once a model is installed, the app transcribes a bundled
11 second clip of clear speech and stores time taken divided by clip length,
per model. Loading the model is done first and not counted. The figure is
shown on screen with a button to measure again.

One thing this number hides: Whisper processes audio in fixed 30 second
windows, so most of its cost does not shrink with a shorter recording. A phone
that takes 4 seconds for the 11 second clip takes nearly as long for a 3
second phrase. The real-time factor is therefore measured on a clip of a
realistic length, and a limit of 1.0 on it means "about ten seconds for a
typical utterance at worst", not "never slower than the speech".

## Alternatives considered

**Always both, take the better one.** Best accuracy, and no thresholds to
tune. It uploads every recording, which gives up the privacy and cost
advantages that are the reason to have on-device inference at all.

**Cloud first, device as the offline fallback.** Simpler, and what most
products do. It makes the on-device model a degraded mode instead of the
default, and every request costs money.

**Decide by language.** Whisper's small models are much weaker outside English,
so non-English speech could go straight to the cloud. This is probably right
and is not done yet: it needs per-language accuracy measurements, and
confidence already catches much of it after the fact.

**A learned router.** A small classifier predicting whether the device will do
well on this audio. Not justified without data, and unexplainable to a user.

## Consequences

- With `AUTO`, a recording the device handles well never leaves the phone.
- A low-confidence recording costs the user two waits: the device, then the
  cloud. Showing the draft softens that; it does not remove it.
- Audio may be uploaded without the user choosing it each time. The app must
  say so plainly where `AUTO` is selected.
- The policy is tested without a device, a model or a network: both engines
  are replaced by scripted fakes.

## What is not known yet

The two thresholds were chosen without measurements.

- **0.6 for confidence** is a guess. Mean token probability is a weak signal:
  Whisper can be confidently wrong, and hesitant on a perfectly good
  transcript of unusual words. This is no longer hypothetical: on 2026-10-09 a
  recording came back as one word repeated about a hundred times with a
  confidence of 0.87. A separate check on the text now catches that case, and
  it is a warning against leaning on confidence for anything else. The right value is the one that best separates
  good from bad transcripts on real recordings in the languages that matter,
  and it may differ by language.
- **1.0 for the real-time factor** means "no slower than the speech itself".
  Whether users tolerate that depends on clip length; for a three second
  phrase, three seconds is fine, and for thirty it is not.

Both need recordings and timings from real phones. The only device available
during development was an emulator on a machine under heavy memory pressure,
whose timings for the same clip ranged from 4 to over 100 seconds, so nothing
measured there was used.
