# ADR 0012: Translation on iOS

Status: accepted, 2026-10-29 (chosen 2026-10-17, built at checkpoint 10.5)

## Context

Translation runs on the phone, on both paths (ADR 0003). Android uses ML Kit.
The iOS app needed a translator of its own behind the same `Translator` seam,
with the same rules: the transcript is shown before translation starts, a
missing language pack is downloaded with "Downloading language pack" on
screen, and a failed translation keeps the transcript.

The owner prefers ML Kit on both phones, for the same languages everywhere.

## Decision

**Apple's Translation framework**, the translator behind Apple's Translate
app. It is part of the iOS SDK, runs on the phone, costs nothing, and adds no
dependency. iOS stores its language packs and shares them with the Translate
app.

**Translating needs no screen; downloading does.** iOS 26 can translate
between installed languages with `TranslationSession(installedSource:target:)`,
so `AppleTranslator` works from `Core/`, and a keyboard could use it later.
iOS downloads a language only through a session that SwiftUI's
`translationTask` hands to a view, and asks the user first in its own sheet.
`AppleTranslator.prepare` therefore waits on `TranslationDownloads`, a
main-actor object in `Core/` that imports no SwiftUI, while a view modifier at
the app's root runs the request. The pipeline is unchanged from Android's: it
asks whether the pair is ready, shows "Downloading language pack", prepares,
then translates.

**One entry per language.** Apple lists some languages once per region (38
entries for 22 languages on iOS 26.4). Each language is offered once, as the
variant iOS treats as its default, under the tag Android and Whisper use
(`no` for Apple's `nb`). The pickers behave as Android's: "From" is what both
Whisper and the translator support, "To" is everything the translator offers.

**Concurrency.** The framework's types are not Sendable. `AppleTranslator`
makes each of its objects inside a `@concurrent` function that never lets it
out, so none crosses an actor. The one place that cannot do that is the
session SwiftUI hands over on the main actor, which Apple's documentation
calls from there; that file imports Translation with `@preconcurrency`.

## Alternatives considered

**ML Kit on iOS as well.** The same 59 languages and the same behaviour on
both phones. Ruled out because:

- its iOS binary has no slice for Apple-silicon simulators, and its package
  turns those builds off for the whole app; Xcode 26 no longer runs
  simulators under Rosetta, so the app would not run in the simulator at all;
- it is distributed only through CocoaPods, which stops accepting new
  releases on 2 December 2026.

**Translating on the server.** Rejected by ADR 0003: one translation path, on
the phone, on both routes.

## Consequences

- Apple's translation does not run in the simulator either. The iOS 26.4
  simulator lists the languages, then reports every pair as unsupported and
  cannot download any (found 2026-10-29, after the choice was made). With
  ML Kit nothing would run there; with Apple's framework everything else
  does, the app says that translation does not, and translation is checked on
  an iPhone. One test translates for real and is skipped on the simulator.
- The iPhone offers 22 languages to translate into, against Android's 59.
  The list is read from the framework at runtime, so it grows with iOS.
- The two phones may translate the same sentence differently.
- The first translation into a new language shows iOS's own sheet, not one of
  ours. If the user declines, the run fails with the transcript kept.
- If Google ships ML Kit with an Apple-silicon simulator build and outside
  CocoaPods, this can be revisited behind the same `Translator` seam.
