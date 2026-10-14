# ADR 0011: A native iOS app on the same server

Status: accepted, 2026-10-14

## Context

The app existed only on Android. On 2026-10-14 the owner asked for it on
iPhone as well, and for both phones to be built together from then on. The
portfolio is meant to show native mobile work, on-device inference and a Go
backend; the backend and its API were already the part with no platform in
it.

## Decision

**A second, native app, in Swift and SwiftUI, on the same Go server.** It
shares no code with the Android app. It shares the server's API, the model
files the server distributes, the whisper.cpp submodule, and the design in
`docs/architecture.md`, which describes both apps. The server does not change.

**Built to the same design, idiomatic for its platform.** The same stages,
seams, rules and thresholds; where the logic is the same, the same test cases.
The Kotlin types have Swift counterparts with the same names
(`SpeechTranslationPipeline`, `Transcriber`, `RemoteTranscriber`,
`BackendClient`, `AppContainer`), but each is written the way Swift is
written: a protocol, not an interface; `PCMAudio`, not `PcmAudio`; an
`AsyncThrowingStream` where Android has a `Flow`.

**One app target, folders for `Core/` and `UI/`.** As on Android, `Core/`
stays free of screens so the keyboard of milestone 11 can reuse it, and moves
into a framework when that keyboard is built, not before.

**Swift 6, with the main actor written down where it applies.** Xcode 26 makes
new app targets main-actor by default. That default is turned off here:
`Core/` would otherwise assume the main thread, and could not move into a
framework without changes. SwiftUI views are main-actor already, and view
models say so with `@MainActor`.

**Streams own their work.** `AsyncThrowingStream.producing` runs the producer
in a task the stream owns, cancelled when its reader stops. This is the Swift
counterpart of a cold `Flow`: the work never outlives the stream, and no task
is left that nothing owns.

**No dependencies.** Everything is in the iOS SDK. The event stream is parsed
by hand (`ServerSentEventParser`, about 60 lines), since `URLSession` has no
event-stream client and `URLSession.AsyncBytes.lines` drops the empty lines
that end each event. Tests use Swift Testing, and a `URLProtocol` stub stands
in for `MockWebServer`.

**The owner creates the project in Xcode**, as with Android. No project
generator. The bundle id is the owner's (`pelumi.PTranslate`), since
installing on a real iPhone needs an identifier registered to their team.

## Alternatives considered

**Kotlin Multiplatform**, sharing `core/` with Android. The parts that could
be shared are the smaller part: the pipeline, the routing rules, the API
client. Recording, translation, speech and loading whisper.cpp are platform
APIs on both phones and would be written twice anyway. It would also add a
toolchain and make the iOS side less clearly native, which is what the
portfolio should show.

**React Native**, which the owner knows well. It would mean rewriting the
Android app too, or keeping three code bases, and on-device inference would
still be native modules on each side.

**XcodeGen or Tuist** to generate the project from a text file, avoiding hand
edits to `project.pbxproj`. Xcode 16 and later keep folders in sync with the
disk, so adding files no longer touches the project file, which removes most
of the reason.

## Consequences

- Every phone feature is built twice. The plan does each phone checkpoint on
  Android, then on iOS, with a commit after each.
- Every rule exists twice and can drift. The "Both phones" convention in
  CLAUDE.md asks that a rule changed on one phone is changed on the other in
  the same milestone, and porting the Android tests is how drift is caught.
- The server proves its API is a real contract: a second client in another
  language works against it unchanged. Checked at 10.1 with a run from the
  simulator against the real server.
- The simulator runs on the Mac, so its speed says nothing about a phone. A
  real iPhone is needed for the benchmark (10.5) and the keyboard (11.2).
