# Shinar (voice translation)

Record speech on a phone, transcribe it, translate it into the listener's
language, and speak it in the speaker's own voice. Native mobile apps (Android
and iOS), on-device inference, and Go backend services.

Since 2026-10-31 it is a product, not a portfolio piece: a social app that
breaks the language barrier, where people talk to anyone in the world without
sharing a language. Each person
signs up with their name and their voice; what they say reaches the other side
in the listener's language and the speaker's voice, in voice notes and text
messages. On the App Store it is paid: a free trial with a cap, then a
subscription, and no free use (2026-11-21). Calls are dropped for now.
`docs/plan.md` has the order (milestones 13 onwards) and
`docs/architecture.md` section 9 the direction.

The Android app came first. The iOS app was added on 2026-10-14: a native
Swift app on the same Go server, built to the same design. Milestone 10
brought it level with Android (2026-10-29), and from then every phone feature
was built on both. On 2026-11-21 the owner chose to take the iPhone to the
App Store first: new phone work is built on the iPhone only until Android
catches up in milestone 22. The owner has a paid Apple Developer account
(2026-10-31).

The app is called Shinar (2026-11-01). Its identifier on both phones is
`com.pelumi.shinar`; the code keeps its old names (the Kotlin package
`com.example.ptranslate`, the Xcode target `PTranslate`). The name is written
once per phone, behind a constant, until its trademark check is done
(ADR 0021).

Read `docs/architecture.md` for the design and `docs/plan.md` for the current
milestone before starting any work.

## Who the owner is

Senior React Native engineer, comfortable in Kotlin/Compose, newer to Go.

- Favour idiomatic, readable code over cleverness, in every language.
- When a Go decision is not obvious, explain it in a one or two line comment at
  the point of the decision (why this, not what it does).
- Kotlin needs no such hand-holding; comment only what is surprising.
- Swift needs no hand-holding either (the owner is comfortable in it); comment
  only what is surprising.

## Working agreement

1. Work in the order of `docs/plan.md`, without stopping at each checkpoint
   (since 2026-10-31). Do everything that needs no decision from the owner;
   stop and ask only when a decision is genuinely theirs.
2. The owner makes the commits, in batches. Never run `git commit`,
   `git push`, or anything that rewrites history; staging with `git add` is
   fine. At the end of a batch, list the commits to make, in order: the files
   in each, the feature it is, how to verify it, and a message. Keep each
   commit buildable on its own, so a problem can be traced to one commit.
3. Ask before adding any dependency that is not in the approved list in
   `docs/architecture.md` (section "Dependencies"). That includes Gradle
   plugins, Go modules, Swift packages, CocoaPods, Homebrew formulas and git
   submodules.
4. If something in the spec or the plan looks wrong, say so before building on
   it. Do not quietly work around it.
5. A significant decision gets an ADR in `docs/adr/NNNN-short-title.md`, written
   in the milestone where the decision lands.
6. Report results as they are. If tests fail or a step was skipped, say so.

## Structure

```
/android   Kotlin, Jetpack Compose, single Gradle module (:app)
/ios       Swift, SwiftUI, one Xcode project (PTranslate), from milestone 10
/server    Go service: HTTP API, job queue, workers
/web       the website: Next.js, exported as plain files (ADR 0028), hosted on Vercel (ADR 0031)
/docs      architecture.md, plan.md, adr/, design/ (briefs for Claude Design), marketing/ (briefs for marketing)
/third_party/whisper.cpp   git submodule, pinned to a release tag
/deploy    the hosted server: one-time set-up, service and Caddy files, the ship script, the owner's commands for the waitlist, reports and accounts (ADR 0023, 0030, 0035)
Makefile   common commands for all three
```

Android package layout (inside `:app`):

```
core/audio       microphone capture and playback, 16 kHz mono PCM, WAV
core/stt         Transcriber interface and its implementations
core/translate   Translator interface and its implementations
core/model       model manifest, download with resume, SHA-256 check
core/net         backend client: upload, SSE
core/speech      SpeechSynthesizer interface and its implementations
core/voice       the user's recorded voice sample: storage and rules
core/pipeline    transcribe -> translate -> speak, the reusable entry point
core/account     the signed-in user: session, profile, sign-up progress
core/rooms       rooms and voice messages: models, live feed, playback
core/push        push notifications: registering the phone, showing them
ui/              Compose screens and ViewModels
```

Native code is in `android/app/src/main/cpp`: the CMake build and the JNI
bridge.

iOS folder layout (inside the app target), the same packages as Android:

```
Core/Audio       microphone capture and playback, 16 kHz mono PCM, WAV
Core/STT         Transcriber protocol and its implementations
Core/Translate   Translator protocol and its implementations
Core/Model       model manifest, download with resume, SHA-256 check
Core/Net         backend client: upload, SSE
Core/Speech      SpeechSynthesizer protocol and its implementations
Core/Voice       the user's recorded voice sample: storage and rules
Core/Pipeline    transcribe -> translate -> speak, the reusable entry point
Core/Account     the signed-in user: session, profile, sign-up progress
Core/Rooms       rooms and voice messages: models, live feed, playback
Core/Push        push notifications: registering the phone, what they do
UI/              SwiftUI views and their view models
```

whisper.cpp is built into an xcframework by a script; Swift calls its C API
directly, so there is no bridge code. ML Kit is Google's pre-built files,
fetched and repackaged by another script into the local Swift package
`ios/MLKit` (ADR 0024). The app needs iOS 18 (ADR 0025).

Server layout:

```
cmd/server           main: wiring, config, graceful shutdown
internal/api         HTTP handlers, SSE
internal/queue       job store (Postgres), worker pool, event listener and bus
internal/provider    Provider, Synthesizer and Translator interfaces, fakes, Whisper on Groq (groq/), Fish Audio (fish/), Google Cloud Translation (google/), Azure AI Translator (azure/)
internal/accounts    users, identities, sessions, handles, kept voices
internal/auth        Apple and Google identity token checks
internal/rooms       rooms, messages, renditions; blocks and reports; the deliverer, its push notifications, and the change hub
internal/push        sending notifications through APNs and FCM
internal/models      manifest and model file serving
internal/storage     audio blob storage
internal/waitlist    the website's waitlist for TestFlight invitations
internal/billing     subscriptions: Apple's signed records checked, what each person may send, the App Store Server API
migrations/          plain SQL, embedded in the binary
```

## Conventions

### What Shinar says, everywhere

- The theme is breaking the language barrier (ADR 0033): the apps, the
  website, the marketing and the store listings. New copy says it plainly,
  then shows it concretely (Rosa speaks Spanish, her grandson English), never
  in abstractions such as "at scale" or "seamless".
- The promise, "Break the language barrier. Be heard in your own voice.",
  and its sub-line are the same words on both Welcome screens and the
  website's hero (`web/lib/site.ts`); change them together.

### Both phones

- The two apps are built to one design (`docs/architecture.md`, sections 2
  and 3). The same stages, seams, rules and thresholds; where the logic is the
  same, the same test cases. A change to a rule on one phone is made on the
  other in the same milestone.
- The exception, from 2026-11-21 until milestone 22: the work for the App
  Store (milestones 16, 18 and 21) is built on the iPhone only. Keep the
  server working for the Android app as it is, and list in milestone 22
  whatever Android will need to catch up.
- Each phone is idiomatic for its platform. Do not write Kotlin in Swift or
  Swift in Kotlin to make the two look alike.

### Android

- `core/` must stay reusable by a future IME. It may not import Compose,
  `Activity`, `ViewModel`, or anything from `ui/`. If it needs a `Context`, it
  takes the application context through its constructor.
- Dependencies are passed through constructors. One `AppContainer` builds the
  object graph. No DI framework.
- Coroutines and `Flow` for async work. No `GlobalScope`. Blocking and native
  calls run on an injected dispatcher so tests can replace it.
- `Transcriber`, `Translator` and `SpeechSynthesizer` are the seams. UI and
  pipeline code depend on the interfaces, never on whisper.cpp, ML Kit or
  Android text-to-speech types.
- JNI surface stays small: one Kotlin class owns the native handle and is the
  only caller of `external` functions.

### iOS

- `Core/` must stay reusable by the keyboard extension (milestone 11). It may
  not import SwiftUI, use UIKit views or view controllers, or reference
  anything in `UI/`.
- Dependencies are passed through initialisers. One `AppContainer` builds the
  object graph. No DI framework.
- Swift 6 language mode. `async`/`await` and `AsyncThrowingStream` for async
  work (the counterpart of `Flow`). No unstructured task that nothing owns
  (the counterpart of `GlobalScope`). Blocking and native work stays off the
  main actor.
- `Transcriber`, `Translator` and `SpeechSynthesizer` are the seams, as
  protocols. UI and pipeline code never see whisper.cpp, ML Kit or
  `AVSpeechSynthesizer` types.
- One Swift type owns the whisper context and is the only caller of whisper's
  C functions.
- Tests use Swift Testing, built into Xcode. Network tests stub `URLSession`
  with a `URLProtocol`; no third-party test libraries.

### Go

- Standard library first: `net/http` with the Go 1.22+ `ServeMux` patterns
  (`"GET /v1/jobs/{id}/events"`) and `log/slog`. Postgres through a pgx pool.
- Interfaces are declared by the package that consumes them and kept small.
- `context.Context` is the first parameter of anything that does I/O.
- Errors are wrapped with `fmt.Errorf("doing x: %w", err)` and handled once:
  either return the error or log it, not both.
- No package-level mutable state. `main` builds everything and passes it down.
- Tests are table-driven where there are several cases, and use only the
  standard `testing` package.

### Website

- Next.js with TypeScript, exported as plain files: nothing on it needs a
  server. CSS Modules; no CSS framework. CSS keyframes for what loops or
  plays on its own, Motion (`framer-motion`, ADR 0028 addendum) for what
  changes with state: a card between screens, an answer opening. Use its
  `m` components (the provider loads the features lazily) and Pearl's
  easing (`web/lib/motion.ts`).
- Pearl, dark only, from the same tokens as the apps. The name lives only in
  `web/lib/site.ts`.
- It shows only what the app does today (brief round 4, section 8).
- The privacy page says only what the apps and the server do, and names the
  services they use (ADR 0032). A change to what is kept, or to who helps,
  changes it in the same milestone, with the apps' privacy pages.
- Its one form, the waitlist, posts to `/v1/waitlist` on the name that served
  the page (ADR 0030); Vercel passes it on to the server (`web/vercel.json`,
  ADR 0031). No CORS, no server address in the page.

### API

- JSON, snake_case field names, versioned under `/v1`.
- Errors: `{"error": {"code": "...", "message": "..."}}` with a matching status.

## Commands

After cloning, fetch whisper.cpp: `git submodule update --init`. Push
notifications on Android need `android/app/google-services.json` from the
Firebase console (git-ignored: the repository is public); without it the
app builds with them off.

Postgres is a local Homebrew install (`postgresql@16`), not Docker: the dev
machine is short on disk and RAM. For the same reason, run the Android
emulator or the iOS simulator, never both at once. `make help` lists every
target. The Android project lives in `android/` (application ID
`com.pelumi.shinar`, code package `com.example.ptranslate`); open that folder
in Android Studio. The iOS project
lives in `ios/`; open `ios/PTranslate.xcodeproj` in Xcode.

| Command | What it does |
| --- | --- |
| `make db-install` | one-time `brew install postgresql@16` |
| `make db-up` / `make db-down` | start / stop Postgres, create the dev and test databases |
| `make db-psql` | open `psql` on the dev database |
| `make server-run` | run the Go server (applies migrations on start) |
| `make server-run-api` / `make server-run-worker` | the API and the workers as two processes, one terminal each |
| `make server-run-flaky` | the same, with a fake provider that fails each job twice, to watch retries |
| `make server-test` | Go tests; queue tests need `make db-up` first |
| `make server-test-race` | the same with the race detector, each test run 3 times |
| `make server-lint` | `go vet` and `gofmt -l` |
| `make server-ship` | build the server for the Azure machine and ship it over SSH; GitHub Actions runs the same on pushes to `main` that change `server/` (ADR 0023) |
| `make waitlist-count` / `make waitlist-list` / `make waitlist-invite N=20` / `make waitlist-remove EMAIL=...` | the hosted server's waitlist: how many wait, everyone on it (invites nobody), the next N iPhone sign-ups into `invitations.csv` for TestFlight, or one address removed (ADR 0030) |
| `make reports-list` / `make report-close ID=... NOTE="..."` | the hosted server's open reports, oldest first, with what was said; or one closed, with what was done, which deletes its copy of the message (ADR 0035) |
| `make account-suspend HANDLE=...` / `make account-delete HANDLE=...` | suspend an account (signed out everywhere, cannot sign in or be found), or delete one on request; delete only says what would go until run again with `YES=1` |
| `make sample-job` | upload a test clip to the running server and stream its events |
| `make voice-check` | one real request to Fish Audio (key from `server/.env`), then plays what it made |
| `make transcribe-check` | one real transcription from Groq (key from `server/.env`) of whisper.cpp's sample clip |
| `make appstore-check` | ask Apple's sandbox for a test notification with the In-App Purchase key named in `server/.env`, and check Apple's signature on it (ADR 0034) |
| `make push-check` | sign in to Apple's and Google's push services with the key files named in `server/.env`, and send to a made-up phone, so nothing arrives (ADR 0026) |
| `make translate-check` | a few real translations from Azure AI Translator (key from `server/.env`), English to Yoruba among them |
| `make sample-speech` | ask the running server to speak a sentence, and play the result; start the server with `VOICE_ENGINE=fake make server-run`, since without a voice engine it refuses |
| `make android-build` | `./gradlew assembleDebug` |
| `make android-test` | JVM unit tests |
| `make android-install` | install the debug build on the connected device and open it |
| `make device-proxy` | `adb reverse tcp:8080 tcp:8080`; run by `android-install` |
| `make emulator-mic` | let the emulator hear the Mac's microphone; it forgets on every start |
| `make android-device-test` | tests that need a device (native code); uninstalls the app afterwards |
| `make model-download` | download the Whisper model into `server/data/models/` (57 MB, git-ignored) |
| `make android-push-model` | copy the model to the device for the device tests; the app itself downloads its model from the server |
| `make ios-whisper` | build whisper.cpp for iOS into `ios/build/` (once after cloning, about a minute; Xcode needs it) |
| `make ios-mlkit` | fetch Google's ML Kit for iOS into `ios/MLKit/Frameworks/` (once after cloning, about 35 MB to download; Xcode needs it) |
| `make ios-build` | build the iOS app for the simulator |
| `make ios-test` | iOS tests on the simulator (`IOS_SIM="..."` picks another one); the Whisper test needs `make model-download` |
| `make ios-device` | build, install and open the app on the connected iPhone, pointed at this Mac's server (start it first); `MAC_HOST=<ip>` if the `.local` name does not resolve |
| `make ios-testflight` | build for the App Store and upload to TestFlight; GitHub Actions runs it on pushes to `main` that change `ios/` (ADR 0022 has the set-up) |
| `make web-dev` | run the website on http://localhost:3100, reloading as files change |
| `make web-build` | build the website into `web/out/`, checking its types |
| `make web-preview` | build the website and serve `web/out/` on http://localhost:3100, as a host would, with `/v1/` passed to the server on this Mac |

The Android app calls `http://localhost:8080`. `adb reverse` forwards that to
this machine, on the emulator and on a phone over USB alike. The iOS
simulator reaches `localhost` directly; a real iPhone uses the Mac's address
on the same Wi-Fi (architecture, section 3.5).

## Non-goals for now

Do not build, scaffold, or add placeholders for any of these:

- calls, and the hands-free room mode planned before them (milestone 17,
  dropped for now on 2026-11-21)
- a free plan on the App Store: there is a capped trial, then a subscription
- Google Play Billing, and anything else Android-only, until milestone 22
- multi-region

Accounts and sign-in stopped being non-goals on 2026-10-31: they are
milestone 13. Billing stopped being one on 2026-11-21: it is milestone 21,
through Apple's In-App Purchase, checked by our own server. Voice keyboards (an Android IME and an iOS keyboard extension)
are milestone 11, after the social milestones; until then, do not build them,
only keep `core/` and `Core/` reusable for them.
