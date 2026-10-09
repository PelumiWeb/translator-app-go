# ADR 0008: Speak the translation with the phone's own voices

Status: accepted, 2026-10-09

## Context

The aim of the product is speech-to-speech interpretation: someone speaks in
one language and the listener hears it in another, eventually in the speaker's
own voice. Until this milestone the app stopped at translated text on screen.

Getting to the speaker's own voice needs a voice-cloning model, which is too
heavy for a phone and will run on the backend (milestone 9). This ADR covers
the step before it: any voice at all.

## Decision

A third stage joins the pipeline behind an interface, as the two before it:

```kotlin
interface SpeechSynthesizer {
    suspend fun speak(text: String, language: Language)
    fun stop()
}
```

`AndroidSpeechSynthesizer` implements it with the platform's `TextToSpeech`
API. `speak` suspends until the utterance has finished, or was stopped, so the
pipeline can treat speaking as one more step and callers can tell when it is
over.

**The pipeline speaks the translation after emitting it.** The text reaches
the screen first. A caller that only wants text, such as a keyboard, passes
`speak = false` and the flow ends at the translation.

**Failing to speak is not a failed run.** The transcript and the translation
are already delivered, so the pipeline reports `SpeechFailed` with a reason
and completes normally. The screen keeps the translation and shows the reason.

**Nothing is spoken when the two languages are the same.** Repeating someone's
words back to them in their own language is not interpreting.

**A voice that is not on the phone yet is retried.** The first time a language
is spoken, the engine has no voice for it. It begins downloading one and fails
that request, in practice with a network timeout. The synthesizer asks again,
up to three times four seconds apart, before telling the user in words that
the voice is not there yet. On the development emulator the first French
request failed and the retry succeeded.

## Alternatives considered

**A cloud text-to-speech API.** Better voices in more languages. It makes the
last step of every interpretation depend on the network and cost money, for a
stage that milestone 9 replaces anyway.

**Bundling an open speech model** (Piper, or sherpa-onnx voices). Offline, and
the same on every phone. Each voice is tens of megabytes per language on top of
the Whisper model and the translation packs, and it is a second native
inference stack to build and maintain for an interim step.

**Speaking from the screen layer instead of the pipeline.** The ViewModel
could call the synthesizer when it receives the translation. Then every front
end would have to remember to, and "transcribe, translate, speak" would not be
one thing that can be tested as a whole.

## Consequences

- The app is speech-to-speech end to end, offline once the voices are on the
  phone, with no new dependency.
- The voice is whatever the phone's engine provides, so quality and language
  coverage vary by device. A language with no voice at all gets the text and a
  message saying so.
- The speech engine belongs to another app on the phone. Since Android 11 the
  manifest must declare an interest in it (`<queries>`), or it is invisible
  and speaking fails as if none were installed.
- Three kinds of downloaded data are now involved, each managed differently:
  the Whisper model (by this app, from the backend), the translation packs (by
  ML Kit), and the voices (by the phone's speech engine). The first use of a
  new language can wait on two of them.
- The voice is not the speaker's. That is milestone 9, which adds a second
  `SpeechSynthesizer` and keeps this one as the offline fallback.
