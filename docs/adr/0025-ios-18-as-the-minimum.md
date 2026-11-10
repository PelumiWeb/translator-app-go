# ADR 0025: iOS 18 as the iPhone app's minimum

Status: accepted, 2026-11-10

## Context

The iPhone app required iOS 26 (section 3 of the architecture), for one
reason: Apple's translator could be used without a screen only from iOS 26
(ADR 0012). Anyone on an older iOS had to update before installing, and the
iPhone XS, XS Max and XR, which stop at iOS 18, could not install it at all.
The owner does not want people to have to update iOS to use the app.

With ML Kit as the translator (ADR 0024), nothing in the translation needs
iOS 26 any more. A build of the app with an iOS 18 minimum showed what else
did:

- `Observations`, which watches an `@Observable` value, is iOS 26 only.
- `isolated deinit` on the Whisper actor needs iOS 18.4.
- The Liquid Glass button style is iOS 26 only.

Not visible to the compiler: whisper.cpp is compiled for the A13 and later
(ADR 0004, addendum), and iOS 18 also runs on the A12 of the XS and XR, which
lacks the dot-product instructions the build uses. Whisper would stop the app
there the moment it ran.

## Decision

**iOS 18.0.** It runs on every iPhone iOS 17 runs on, from the XS and XR
(2018), so a lower minimum would only reach people who have kept an older iOS
by choice, at a high cost: `@Observable` (iOS 17) and `Mutex` (iOS 18) are used
throughout, and Apple's newer concurrency features with them.

- `nextChange(of:)` (`Core/NextChange.swift`), built on `withObservationTracking`,
  replaces `Observations` in the container and in one test.
- The Whisper actor frees its model in a plain `deinit`. That is as safe: an
  actor's method keeps the actor alive while it runs, so its last reference
  cannot go while a transcription is using the model.
- `glassButtonStyle()` uses Liquid Glass on iOS 26 and the bordered button,
  the nearest look, on iOS 18.
- The main-actor view models keep their `isolated deinit`: the compiler
  accepts it at iOS 18.0, and the whole test suite passes on the iOS 18.1
  simulator.

**The A12 transcribes on the server.** The Whisper build keeps the A13's
instructions, which every newer iPhone uses for speed. `WhisperSupport` reads
the processor's features (`hw.optional.arm.FEAT_DotProd` and `FEAT_FP16`, the
ones ggml reads). Without them:

- the route is the server, and the on-device route cannot be chosen;
- the model is never downloaded and the speed is never measured;
- `WhisperContext.load` refuses before any of whisper.cpp's code runs, as a
  last guard;
- Advanced settings says, in place of the route, model and speed, that every
  recording is sent to the server because the processor is too old for the
  speech model.

Android has no counterpart: its Whisper build targets older processors.

## Alternatives considered

**Stay at iOS 26.** Nothing to change, but it shuts out everyone who has not
updated, and the XS and XR for good.

**iOS 18 with Apple's translator.** Every translation below iOS 26 would go
through a SwiftUI view that holds a session, which the keyboard of milestone
11 could not rely on, for 22 languages. Replaced by ADR 0024.

**Build Whisper for the A12.** The XS and XR would transcribe on the phone,
but every newer iPhone would lose the fast instructions, for two models from
2018.

**Two Whisper builds, picked at run time.** The A12 would keep on-device
transcription at full speed for the rest, but it doubles the native library
in the app and needs ggml's run-time dispatch, which its iOS build does not
offer.

## Consequences

- The app installs on iOS 18.0 and later, on every iPhone from the XS and XR.
- On iOS 18 the settings button is a plain bordered circle, not glass, and
  the system's own controls look as iOS 18 draws them.
- An XS or XR always uses the server for speech, so it needs a connection and
  the user's sign-in for every recording; translation and speech output are
  unaffected.
- Tests run on the iOS 26.4 simulator (`make ios-test`) and on iOS 18.1
  (`make ios-test IOS_DEST='platform=iOS Simulator,OS=18.1,name=iPhone 16'`,
  with an iPhone 16 simulator made for that runtime in Xcode or with
  `xcrun simctl create`). CI runs the newest only.
- No iPhone XS or XR was available to try the server route on; the check
  itself is tested only where it passes.
