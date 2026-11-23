# Plan

Milestones are thin vertical slices. Each ends with something that can be
shown working.

Each **checkpoint** is one commit, or a few. Since 2026-10-31 the work no
longer stops at each one: Claude carries on until a decision is needed from
the owner, then lists the commits to make, with what each contains, how to
verify it, and a message. The owner reviews and commits them in batches.

Status: `[ ]` not started, `[~]` in progress, `[x]` done, `[-]` dropped.

Milestones are numbered in the order they were planned, not the order they
are built. On 2026-10-31 the app became a product, a social app where people
talk across languages (ADR 0015), and the order changed. On 2026-11-21 it
changed again, for the App Store (below). The order of work, as of
2026-11-21:

1. Milestone 16, its first part: block and report
2. Milestone 21: paying for Shinar, after a free trial
3. Milestone 18, the rest: the App Store
4. Then, in an order to agree after the launch: milestone 12 (voice notes
   shared in from other apps), milestone 20 (group rooms), the rest of
   milestone 16 (discovery), milestone 22 (Android catches up, and Google
   Play), milestone 11 (voice keyboards), milestone 7 and checkpoint 8.2

Milestone 17, calls, is dropped for now. Milestones 13, 14 and 15 are built;
what is left of them is listed under each.

Milestone 10 was done on 2026-10-29; checkpoint 9.4 and milestone 19 (the
design) on 2026-10-31. Milestone 12 was dropped on 2026-10-31, then brought
back the same day, after milestone 14.

On 2026-11-18 the owner asked for four things at once, built ahead of
milestone 12: text messages (milestone 15), recording with pause, stop and
send (checkpoint 14.6), a call button that says calls are coming soon
(milestone 17), and a second account to test with (checkpoint 13.5).

On 2026-11-19 the owner began a soft launch: outside testers on TestFlight,
invited from a waitlist on the website, with marketing handed to Claude
(`docs/marketing/brief-1-soft-launch.md`). Built for it, ahead of the rest of
milestone 18: daily limits (checkpoint 18.5) and the waitlist (18.6).

On 2026-11-21 the owner decided to take the app to the App Store with the
features it has today, and chose:

- **No free use on the App Store.** A new person gets a free trial, with a
  cap of its own, then pays by subscription to keep using Shinar
  (milestone 21).
- **The iPhone first, on its own.** Payment through Apple's In-App Purchase
  only. New phone work is built on the iPhone only until Android catches up
  in milestone 22.
- **Purchases checked by our own server.** No payment service in between,
  such as RevenueCat.
- **No calls for now.** Milestone 17 is dropped, and the call button goes
  with it.
- **Block and report before the App Store** (milestone 16), as Apple
  requires.

The first App Store version needs the first part of milestone 16, all of
milestone 21, and the rest of milestone 18.

A checkpoint with phone work is done on Android, then on iOS; from
2026-11-21 until milestone 22, on iOS only.

---

## [x] Milestone 0: Agree on the design

No code.

- CLAUDE.md, docs/architecture.md, docs/plan.md
- Owner answers the open questions and approves the new dependencies

**Checkpoint 0.1**: the three documents. Owner runs `git init` and commits.

---

## [x] Milestone 1: End to end, ugly

Record on Android, upload, Go enqueues, the fake provider "transcribes", SSE
streams status back, text shows on screen. No on-device inference, no
translation, no styling.

**Demo**: start Postgres and the server, tap Record, speak, tap Stop. The
screen shows `queued`, `processing`, the words arriving one by one, then the
final text. Kill and restart the server mid-job to show nothing is lost.

**Checkpoint 1.1: repo skeleton** (done)
- `.gitignore`, `Makefile` (Postgres 16 through Homebrew)
- `server/` Go module, `cmd/server` with config, `slog`, `/healthz`, graceful
  HTTP shutdown
- Embedded SQL migrations and the runner; `jobs` and `job_events` tables
- Verify: `make db-up && make server-run`, `curl localhost:8080/healthz`

**Checkpoint 1.2: queue** (done)
- Job store: enqueue, claim (`FOR UPDATE SKIP LOCKED`), complete, fail,
  release; every status change records a `job_events` row
- Worker pool with two-stage graceful shutdown
- `Provider` interface and the fake provider
- Local audio storage (moved here from 1.3 so the workers could be wired into
  `main` and run for real)
- Queue tests against Postgres: no double claims under concurrency, a job
  reaches `done`, a provider error marks it `failed`, shutdown lets a job
  finish within the grace period and releases it after
- ADR 0001
- Verify: `make server-test`

**Checkpoint 1.3: HTTP API and SSE** (done)
- `POST /v1/jobs` (multipart, size limit, WAV check, audio into storage,
  wakes a worker)
- `GET /v1/jobs/{id}/events` with persisted events, in-process bus, replay
  from `Last-Event-ID`, heartbeat; streams end on server shutdown
- The store publishes each event after its transaction commits
- SSE handler tests and upload handler tests with `httptest`, no database
- ADR 0002, ADR 0003
- Verify: `make sample-job`, or by hand:
  `curl -F audio=@sample.wav -F source_lang=en localhost:8080/v1/jobs`
  then `curl -N localhost:8080<events_url>`

**Checkpoint 1.4: Android app** (done)
- Android project in `android/`, package `com.example.ptranslate`
- One Compose screen: Record/Stop button, status line, text
- `AudioRecorder` (16 kHz mono PCM), WAV encoding, microphone permission
- `BackendClient` (upload and SSE), `Transcriber` interface and
  `RemoteTranscriber`
- Minimal `SpeechTranslationPipeline` without translation yet
- JVM tests: WAV encoding, `RemoteTranscriber` against `MockWebServer`,
  the ViewModel with fakes
- Verify: `make db-up`, `make server-run`, then `make android-install` with an
  emulator running. Tap Record, speak, tap Stop.

Deliberately left out of M1: retries with backoff, lease expiry sweep,
`Last-Event-ID` on the client, any error UI beyond a line of text.

---

## [x] Milestone 2: On-device transcription

**Demo**: airplane mode on, record, the transcript appears.

**Checkpoint 2.1: native build and JNI** (done)
- whisper.cpp v1.9.4 as a shallow submodule in `third_party/`
- CMake build into one shared library, always optimised, 16 KB page aligned
- JNI bridge and `WhisperContext`, the only class that calls native code
- Device tests that load the library and transcribe whisper.cpp's sample clip
- `make model-download`, `make android-push-model`, `make android-device-test`
- ADR 0004
- Verify: `make model-download android-push-model android-device-test` with an
  emulator running

**Checkpoint 2.2: on-device transcription in the app** (done)
- `WhisperTranscriber` behind `Transcriber`, with confidence from token
  probabilities, on a single-thread dispatcher
- Silence and non-speech are reported as "no speech" instead of being
  transcribed into invented text
- `RoutingTranscriber` and a switch on screen: on this device, or on the server
- Time taken, real-time factor and confidence shown under each transcript
- Model read from the app's files; pushed by hand with
  `make android-push-model` for now (model manager is M4)
- Verify: `make android-install android-push-model`, turn on airplane mode,
  record. On the emulator, enable the host microphone first (Extended
  controls, Microphone, "Virtual microphone uses host audio input"), or the
  recording is silent and the app says so.

---

## [x] Milestone 3: Translation

The first point at which the app does what it is for.

**Demo**: pick source and target language, speak, read the translation.

**Checkpoint 3.1: translator and pipeline** (done)
- `Translator` interface and `MlKitTranslator` (ML Kit Translate 17.0.3)
- Pipeline transcribes, downloads language packs if missing, then translates;
  a failed translation still delivers the transcript
- JVM tests for the pipeline with fakes; device tests that translate English
  to Spanish with ML Kit for real
- The screen is unchanged: it still passes the same source and target, so
  nothing is translated until 3.2
- Verify: `make android-test`, and `make android-device-test` with the
  emulator online

**Checkpoint 3.2: language pickers and the translation on screen** (done)
- "From" and "To" pickers. "From" lists what both Whisper and ML Kit support;
  "To" lists everything ML Kit translates into. The source is picked by hand,
  not detected
- Status shows "Downloading language pack" and "Translating"
- The transcript, then the translation under it
- A new recording clears the last result and keeps the chosen languages
- Verify: `make android-install android-push-model`, pick languages, record

---

## [x] Milestone 4: Model distribution

**Demo**: fresh install with no model. The app downloads it with a progress
bar. Turn the network off halfway and back on: the download resumes. Corrupt
the file on the server: the app rejects it.

**Checkpoint 4.1: server endpoints** (done)
- `internal/models`: a catalogue built at startup from the `.bin` files in
  `MODELS_DIR` (default `data/models`), each hashed with SHA-256
- `GET /v1/models/manifest` and `GET /v1/models/{id}`
- Range and resume through `http.ServeContent`; the SHA-256 is the `ETag`, so
  a resume against a replaced file gets the whole new file
- Tests: manifest matches the bytes served, resume from an offset, changed
  file, impossible range, unknown ids
- Verify: `make server-test`, then with the server running
  `curl localhost:8080/v1/models/manifest`

**Checkpoint 4.2: Android model manager** (done)
- `BackendClient`: fetch the manifest, open a model download from an offset
- `ModelManager`: download with resume, verify SHA-256, rename into place;
  state as a `StateFlow`
- JVM tests against `MockWebServer`: good hash, bad hash, resume, server
  ignoring the range, partial file from another version, dropped connection,
  offline with a model installed
- ADR 0005
- Nothing in the app calls it yet; that is 4.3
- Verify: `make android-test`

**Checkpoint 4.3: download on screen** (done)
- `ModelInstaller` interface; the ViewModel mirrors the model's state
- Screen: "not on this device yet" with a Download button, a progress bar,
  "Checking the download", the failure reason with "Try again", or "ready"
- `WhisperTranscriber` asks for the managed model on each transcription
- The download starts only when the user asks, not on first launch: it is
  57 MB of their data
- `make android-push-model` is now only needed for the device tests
- Verify: `make server-run`, `make android-install`, tap "Download speech
  model"

---

## [x] Milestone 5: Queue hardening

**Demo**: the fake provider fails twice then succeeds; the job retries with
growing delays and completes. Kill a worker mid-job; the job is picked up
again after its lease expires. Drop the SSE connection; the client reconnects
and continues from where it left off.

**Checkpoint 5.1: retries and backoff** (done)
- A failed attempt is retried after an exponential, jittered delay (2 s, 4 s,
  8 s ... up to 1 minute, each moved by up to 20%) until `max_attempts`
- `provider.Permanent` marks failures that retrying cannot fix; those, and
  missing audio, fail at once
- A retry is recorded as a `queued` event with `reason: "retry"`, the error
  and the delay, so a client following the job sees it is still alive
- The audio is kept across attempts and deleted only when the job ends
- `FAKE_FAIL_ATTEMPTS=2` (or `make server-run-flaky`) makes the fake provider
  fail the first two attempts of every job
- Verify: `make server-test`; or `make server-run-flaky`, then
  `make sample-job` in another terminal

**Checkpoint 5.2: lease expiry sweep** (done)
- Heartbeat: a worker renews its lease every third of its length while a job
  runs, so a long job is not mistaken for a dead worker
- Sweeper: jobs whose lease has run out go back to `queued`, or to `failed`
  if they are out of attempts
- Fencing: every write by a worker requires the attempt number it claimed, so
  a worker that stalled and lost its job cannot overwrite the new owner
- `JOB_LEASE` and `SWEEP_INTERVAL` settings
- Tests: recovery from a dead worker, a long job keeps its lease, a stale
  worker is refused on every kind of write and stops quietly
- ADR 0001 addendum
- Verify: `make server-test-race`

**Checkpoint 5.3: `LISTEN/NOTIFY` event bus** (done)
- The store announces each event with `pg_notify` inside the transaction that
  records it
- `Listener`: a dedicated connection that hears announcements and feeds the
  local bus; on a dropped connection it ends open streams, then reconnects
- `-role all|api|worker`, and `make server-run-api` / `make server-run-worker`
- Tests: delivery through Postgres, nothing heard from a rolled-back change,
  recovery after the listener's connection is killed
- ADR 0006
- Verify: `make server-test-race`; or run `make server-run-api` and
  `make server-run-worker` in two terminals, then `make sample-job`

**Checkpoint 5.4: client reconnect and idempotent uploads** (done)
- Server: `Idempotency-Key` on `POST /v1/jobs`. One job per key, enforced by a
  unique index; a repeat returns the existing job and its current status
- Android: the upload is retried on a connection failure or a 5xx, always
  with the same key
- Android: a stream that ends before the job does is reconnected with
  `Last-Event-ID`; it gives up after five tries in a row that bring nothing new
- The screen shows "Connection lost. Reconnecting" and keeps the words already
  received
- Tests: one job from sixteen simultaneous uploads with one key; reconnect,
  replayed events, a cut connection, giving up, and upload retries against
  `MockWebServer`
- Verify: `make server-test-race` and `make android-test`; or start a
  transcription on the server route and restart the server during it

---

## [x] Milestone 6: Fallback policy

**Demo**: three cases side by side. No model: goes to the cloud. Benchmark
marked slow: goes to the cloud. Mumbled audio: runs on device, confidence is
low, goes to the cloud, and the screen says which path produced the result.

**Checkpoint 6.1: the policy** (done)
- `TranscriptionRoute.AUTO` in `RoutingTranscriber`: device first, cloud when
  there is no model, the device is too slow, or the device's result is poor
- The cloud is a fallback, not a requirement: an unreachable cloud leaves the
  device's result in place, marked as such
- A doubtful device result is shown as a draft while the cloud works
- `Transcript.note` records how a result was routed
- `SilentAudioException` and `NoSpeechException`, so silence is never
  uploaded while "sound but no words" is given to the cloud
- Thresholds in one object, `FallbackThresholds`; provisional, see ADR 0007
- 18 JVM tests with scripted fake transcribers
- ADR 0007
- The screen still offers only device or server; `AUTO` reaches it in 6.2
- Verify: `make android-test`

**Checkpoint 6.2: benchmark and screen** (done)
- `DeviceBenchmark`: once a model is installed, times it on a bundled 11
  second clip of speech, with model loading left out, and stores the result
  per model
- Three-way choice on screen (automatic, this device, server), each with one
  line saying what it does with the recording; automatic is the default
- The device's speed on screen, with "Measure again"
- Each result says why it was routed as it was, when that needs saying
- `wavToPcm`, for reading the bundled clip
- Verified on the emulator: no model goes to the server; a device marked
  slow goes to the server; both say why. The low-confidence case needs real
  speech and is covered by the JVM tests only
- Verify: `make android-install`, download the model, record on "Automatic"

---

## [ ] Milestone 7: Real provider and presentation

**Demo**: the portfolio walkthrough. A real recording, in a language the small
model handles badly, falls back to a real cloud transcription.

- One real `Provider` implementation (choice needs approval; ADR 0008)
- UI pass: the app stops being ugly (replaced by milestone 19, 2026-10-31)
- README with a demo recording, an architecture diagram, and how to run it
- CI: Go tests with a Postgres service, Android unit tests and build

Checkpoints: (7.1) provider, (7.2) UI, (7.3) README and CI.

**Checkpoint 7.1: the real provider** (built 2026-11-11, at the owner's
request: testers on TestFlight met the fake transcript)
- The owner chose Groq, with OpenAI's Whisper large-v3, over whisper.cpp on
  the server (no better than the phone's own model on a 1 GB machine) and
  Azure AI Speech (no Yoruba, Hausa or Igbo). ADR 0009
- `internal/provider/groq`: the recording, the language and the model to
  Groq's OpenAI-compatible API; segments Whisper's own rules call silence
  left out; "no speech was recognised" when nothing is left; rate limits and
  outages retried, everything else failed at once. No Go module
- A `GROQ_API_KEY` on its own chooses Groq; `TRANSCRIBE_ENGINE=fake` keeps
  the fake (`make server-run-flaky` does). `make transcribe-check`
- Both privacy pages say Groq writes the words down when the phone cannot
- Tests: the request, the model, which failures retry, and what is not
  speech (seven cases); a live test behind the key

---

## [~] Milestone 8: Speak the translation

Added on 2026-10-07. The aim of the product is speech-to-speech
interpretation: a person says something in one language and hears it in
another, in their own voice. Milestones 8 and 9 get there in two steps.

Done before milestone 7 at the owner's request (2026-10-09): it needs no new
dependency and makes the app speech-to-speech.

**Demo**: speak, and the phone says the translation aloud.

**Checkpoint 8.1: the phone speaks** (done)
- `SpeechSynthesizer` interface, and `AndroidSpeechSynthesizer` on the
  platform's `TextToSpeech`: a stock voice, no new dependency
- The pipeline speaks the translation after emitting it; `speak = false` for
  callers that only want text
- Failing to speak does not fail the run: the text stays, with the reason
- A voice that is still downloading is retried before the user is told
- Nothing is spoken when both languages are the same
- JVM tests with a fake synthesizer; device tests on the real engine
- ADR 0008
- Verify: `make android-install`, record with a target language other than
  the source, and listen

**Fix after 8.1** (2026-10-09)
- A recording came back as "I, I, I, ..." repeated about a hundred times and
  was translated and read aloud. Whisper's retry is back on, limited to two;
  a check on the text itself now rejects repetition loops before they are
  shown, translated or spoken; on the automatic route they go to the server

**Checkpoint 8.2: controls** (deferred)
- Stop speaking, speak again, and a switch to keep the app silent
- Deferred on 2026-10-09: the owner asked for the speaker's own voice
  (milestone 9) next

---

## [~] Milestone 9: Speak it in the user's own voice

Next, at the owner's request (2026-10-09), ahead of milestone 7 and
checkpoint 8.2.

Paused after checkpoint 9.2 on 2026-10-14 for milestone 10. It resumes at 9.3
once the iOS app has caught up, and from there each checkpoint is done on both
phones.

The spoken translation sounds like the person who owns the phone. This is for
every user of the app, each in their own voice.

Approach, decided on 2026-10-09 (replacing the 2026-10-07 idea of using each
recording as its own sample): the user is onboarded once by reading a short
passage aloud, about 20 seconds. That sample stays on the phone and is sent
with each request to speak. The server stores no voices, and there are still
no accounts.

**Demo**: on first launch, read the passage. From then on, every translation
is spoken in that voice.

**Checkpoint 9.1: synthesis jobs on the server** (done)
- A second kind of job on the same queue (`kind = 'synthesize'`): a voice
  sample and a text go in, audio comes out, with the same events, retries,
  leases and recovery as transcription
- `POST /v1/speech` (voice sample, text, language; idempotency key), and
  `GET /v1/jobs/{id}/audio` for the result, with range support
- `provider.Synthesizer` interface, and a fake that answers with one beep per
  word: audibly not a voice, but real audio of the right length
- The voice sample is deleted when its job ends; generated audio is deleted an
  hour after it was made
- Tests at every layer, and a run with `curl` against a live server
- Verify: `make server-test-race`; or `make server-run`, then
  `make sample-speech`

**Checkpoint 9.2: onboarding on Android** (done)
- A "Record your voice" screen, shown on first launch: what the recording is
  for and what happens to it, a passage to read (about 20 seconds), a timer
  and progress while recording, then listen, record again, or keep
- A recording under 10 seconds, or a silent one, cannot be kept, and says why
- "Not now" uses the phone's voice and does not ask again; the main screen
  shows whether a voice is recorded, with a button to record or change it
- A saved voice can be replaced or deleted from the phone
- `VoiceSampleStore`: one WAV file in the app's private files
- `AudioPlayer`, used here to listen back and in 9.3 to play the server's
  speech
- JVM tests for the rules, the store and the screen's logic
- Nothing is uploaded yet: the sample is only recorded and kept
- Verify: `make android-install`, then tap "Record" beside "Your voice"

**Fix after 9.2** (2026-10-17)
- The onboarding screen says the recording stays on the phone, but Android's
  backup rules were still the template, so the voice sample went into the
  user's cloud backup (until the 57 MB model pushed the app past the 25 MB
  backup limit, which stopped all of its backups). The voice sample, the model
  and the `device` preferences are now excluded from cloud backup and from
  transfer to a new phone. iOS excludes its sample from the start (10.2)

**Checkpoint 9.3: the app speaks through the server** (Android, then iOS)
(done)
- `RemoteSpeechSynthesizer` on both phones: uploads the voice sample and the
  translation to `POST /v1/speech`, follows the job, downloads the audio and
  plays it. Each step retries and reconnects as transcription does
- `FallbackSpeechSynthesizer`: the user's voice through the server, or the
  phone's own voice when no voice is recorded, there is no network, or the
  server fails. The translation is always heard
- The upload-with-retries and follow-the-job logic moved out of
  `RemoteTranscriber` into `core/net` (`Core/Net` on iOS), shared by both
  remote stages; the transcriber's tests still guard it
- Fixed on both phones: a speech job's `done` event carries no text, which
  the clients' event parser required
- iOS: `SpeechSynthesizer.stop()` is `async`, since a voice may be playing on
  the main actor; if a run is cancelled, a clip from the server that is
  already playing is heard to its end
- Tests: 8 cases for the remote synthesizer against a stub server and 4 for
  the fallback, the same on both phones. Android 138 pass; iOS 160 pass,
  1 skipped. Both clients were also checked once against the real server:
  the fake engine's audio came back and decoded (0.92 s, a beep per word)
- With the fake engine the "own voice" is beeps until 9.4
- Verify: `make android-test` and `make ios-test`. Then `make db-up` and
  `VOICE_ENGINE=fake make server-run`; on the emulator or an iPhone, record
  your voice, then record a sentence with a target language other than the
  source: the translation is heard as beeps. Stop the server and record
  again: the phone's voice says it instead

**Fix after 9.3** (2026-10-31)
- On the owner's iPhone the translation was heard as beeps, the fake engine,
  which reads as broken. A server now has no voice engine unless started
  with `VOICE_ENGINE=fake`; without one it refuses requests to speak with
  `501` (`no_voice_engine`), and both phones treat 501 as final, so the
  phone's own voice speaks at once instead of after retries. A worker with
  no engine fails a speech job at once
- Tests: the refusal and the worker's failure on the server, and one case on
  each phone. Android 139 pass; iOS 161 pass, 1 skipped

**Checkpoint 9.4: a real voice engine** (the server, then the wording on both
phones) (done)
- Changed on 2026-10-31: a paid hosted engine straight away, instead of the
  free public demo planned first, since the app is now a product (ADR 0015)
- Fish Audio, chosen by the owner on 2026-10-31 (ADR 0010): `fish.Synthesizer`
  sends the voice sample, its transcript (`provider.OnboardingPassage`, the
  passage read at onboarding) and the text, and gets 16 kHz PCM back, which
  the server wraps as WAV. A small MessagePack encoder of our own, since
  Fish Audio takes an inline sample only in MessagePack: no Go dependency
- The key is `FISH_AUDIO_API_KEY` in `server/.env` (git-ignored), loaded by
  the Makefile's server targets; a key on its own chooses Fish Audio. The
  free model `s2.1-pro-free` is used until API credit is added
- Failures: 429 and 5xx are retried; others (a bad key, no credit, a bad
  request) fail the job at once, and the phone's voice speaks instead
- The onboarding screen on both phones now says the recording goes on to
  Fish Audio, the service that makes the speech
- Tests: the encoder against the MessagePack specification, the request and
  the WAV it produces, each kind of failure, and a live test that runs only
  with a key (`make voice-check`). Checked live: 2.7 s of Spanish in the
  sample's voice in 4.4 s, and the whole path through the server
- Verify: `make voice-check`; then `make server-run` and, on the iPhone,
  record a sentence with a voice recorded: it is spoken in your voice
- Heard by the owner on their iPhone on 2026-10-31: their own voice, in
  another language

**Fix after 9.4** (2026-10-31)
- The owner noticed seconds of silence while their voice was being made,
  with "Speaking" already on screen. `SpeechSynthesizer.speak` now takes an
  `onStart`, called when the speech becomes audible; the pipeline reports
  `PreparingSpeech` before it and `Speaking` at that moment. Both screens
  say "Preparing speech", with a spinner beside the status during every wait
  of a run, and none while the translation is heard
- On Android the pipeline stays a plain `flow`, with the speech running
  alongside: a `channelFlow` would drop the transcript when translation
  fails, which the tests caught
- Tests: the new stage in the pipeline and on both screens, and both real
  synthesizers reporting their start. Android 140 pass; iOS 162 pass,
  1 skipped

Later, not planned in detail:
- The owner's long-run idea: a free engine for free users and a paid one for
  paying users. Accounts arrive with milestone 13 and billing with milestone
  17; the engine is chosen per request, so this stays possible
- A guest mode: with one voice per phone, a guest's words come out in the
  owner's voice. Less pressing once each person has an account

---

## [x] Milestone 10: The iOS app catches up

Added on 2026-10-14 at the owner's request, and next. The app should exist on
iPhone as well, and from here on be built on both phones together. A native
Swift app on the same Go server; architecture section 3 has the design. The
server does not change in this milestone.

The checkpoints follow the order Android took, merged where the iOS app can
start from where Android ended up (for example, the upload already retries
and the stream already reconnects in 10.1). One exception: recording the
user's voice (Android's 9.2) was moved from last to second on 2026-10-14, at
the owner's request. It needs only the recorder, and it is the first screen a
new user sees.

**Demo**: on an iPhone, everything the Android app does at checkpoint 9.2.
Record, transcribe on the phone or on the server, chosen automatically;
translate; speak the translation; record your voice.

**Checkpoint 10.1: project, and the server route** (≈ 1.4 and 5.4) (done)
- The owner creates the Xcode project, as with Android at 1.4: File, New,
  Project, iOS App; product name `PTranslate`; organisation identifier
  `com.example`; interface SwiftUI; language Swift; testing Swift Testing;
  storage none; saved in `ios/`, with "Create Git repository" unticked
  because `ios/` is inside this repository. Then the minimum iOS version
  is set to 26.0, Swift 6 language mode, and nonisolated default actor
  isolation. The bundle id stays the owner's `pelumi.PTranslate`, and the UI
  test target Xcode adds is removed
- A shared scheme, so `xcodebuild` works without Xcode having opened the
  project
- `Info.plist`: microphone and local-network descriptions, plain HTTP to local
  addresses only, and `BackendURL` from the `BACKEND_URL` build setting
- `.gitignore` for Xcode's per-user and build files
- One screen: Record/Stop, a status line, the text
- `AudioRecorder` on `AVAudioEngine`, converted to 16 kHz mono 16-bit, capped
  at 30 seconds; WAV encoding; the microphone permission
- `BackendClient` on `URLSession`: upload with an idempotency key and retries;
  the event stream, reconnecting with `Last-Event-ID`
- `RemoteTranscriber`, and a minimal `SpeechTranslationPipeline`
- Tests: WAV encoding; the event-stream parser; `RemoteTranscriber` against a
  `URLProtocol` stub, the same 15 cases as Android; the view model with fakes.
  30 in all
- `make ios-build`, `make ios-test`
- ADR 0011
- Verify: `make server-run`; run the app in the simulator from Xcode; record,
  and read the transcript

**Checkpoint 10.2: record your voice** (≈ 9.2) (done)
- The "Record your voice" screen on first launch: what the recording is for
  and what happens to it, the same passage as Android, a timer and progress
  while recording, then listen, record again, or keep
- The same rules: under 10 seconds, or silent, cannot be kept, and says why
- "Not now" does not ask again; the main screen shows whether a voice is
  recorded, with a button to record or change it; a saved voice can be
  replaced or deleted
- `VoiceSampleStore`: one WAV file in Application Support, excluded from
  iCloud backup so that "the recording stays on this phone" is true
- `AudioPlayer` on `AVAudioEngine`; `PCMAudio` gains its loudness check and
  WAV reading
- Tests: the voice rules, the store (and that it is kept out of backups),
  the screen's logic, and the rest of Android's WAV cases. 54 in all
- Verify: run the app in the simulator; it opens on "Record your voice"

**Checkpoint 10.3: whisper.cpp on iOS** (≈ 2.1) (done)
- A script builds the pinned submodule with CMake and Ninja into
  `whisper.xcframework`, for iPhone and the simulator, git-ignored;
  `make ios-whisper`, which `ios-build` and `ios-test` run first. On the CPU
  (Metal off) and compiled for the A13, iOS 26's oldest chip
- `WhisperContext`, the only Swift type that calls whisper's C functions: an
  actor on a dispatch queue of its own, with Android's decoding settings
- Simulator tests that load the model and transcribe whisper.cpp's sample
  clip, refuse a missing or wrong file, and check the CPU instructions used.
  59 tests in all
- ADR 0004 addendum: the build, CPU rather than GPU, and the A13 target
- Verify: `make model-download ios-test`

**Checkpoint 10.4: on-device transcription and the model download**
(≈ 2.2, 4.2, 4.3 and the fix after 8.1) (done)
- `ModelManager`: manifest, download with resume (`Range` and `If-Range`),
  SHA-256, rename into place; kept out of iCloud backup. On the main actor,
  since its state drives the screen; the download and the checksum run off it
- `WhisperTranscriber`: confidence; silence and "no speech"; the
  repetition-loop check, with zlib as on Android so both phones measure the
  same ratio
- `RoutingTranscriber` with two routes, this device and the server; the
  server stays the default until the automatic route arrives at 10.6
- On screen: the route switch, the download button, progress, "Checking the
  download", failure with "Try again"; under each result, where it came from,
  how long it took and the confidence
- Tests: Android's model download cases, transcript checks, checksum vectors
  and view model cases; the transcriber on the simulator with the real model
  (speech, silence, hiss, no model). 94 in all. Also checked once against the
  real server: the 57 MB model downloaded and verified in 9 seconds and
  transcribed the sample clip
- Verify: `make server-run`, choose "This device", download the model in the
  app, then stop the server and record

**Checkpoint 10.5: translation** (≈ 3.1 and 3.2) (done)
- Apple's Translation framework, chosen on 2026-10-17 (architecture section
  3.4). Found on 2026-10-29: it does not translate in the simulator, which
  lists its languages but reports every pair as unsupported. The app says so
  there, and real translation is checked on the iPhone
- `Translator` and `AppleTranslator`; the pipeline translates after
  transcribing, and a failed translation keeps the transcript
- iOS downloads a language pack only from a screen, after asking the user in
  its own sheet: `TranslationDownloads` in `Core/` waits while the app's root
  view runs the request through SwiftUI's `translationTask`. Declining fails
  the run and keeps the transcript, as a failed download does on Android
- One entry per language, 22 in all, as the variant iOS treats as the
  default; Apple's Norwegian, `nb`, is `no`, as on Android and in Whisper
- `WhisperLanguages`, as on Android: "From" lists what both Whisper and the
  translator support, "To" everything the translator translates into
- "From" and "To" pickers; the transcript, then the translation;
  "Downloading language pack", "Translating"
- Tests: Android's translation cases for the pipeline and the view model, the
  Whisper language list, matching Apple's languages, and the real framework's
  list. A test that really translates English to Spanish runs only on an
  iPhone and is skipped on the simulator. 111 pass, 1 skipped
- ADR 0012
- Verify: `make ios-test`. In the simulator, pick two languages and record:
  the transcript appears, then "Apple's translation does not work in the
  simulator". On the iPhone the translation appears instead, after iOS asks
  to download the language the first time

**Checkpoint 10.6: the automatic route** (≈ 6.1 and 6.2) (done)
- `RoutingTranscriber` with the automatic route, and the same
  `FallbackThresholds` values as Android (confidence 0.6, real-time factor
  1.0); each transcript carries a `RoutingNote` saying how it was routed
- `DeviceBenchmark` on the same bundled 11 second clip (whisper.cpp's
  `jfk.wav`, as on Android), stored per model in `UserDefaults`. The container
  watches the installed model and measures whenever one appears
- Three-way choice on screen, automatic by default; the device's speed, with
  "Measure again"; under each result, why it was routed as it was, when that
  needs saying
- Tests: Android's 20 routing cases, its 6 benchmark cases, and its 4 view
  model cases for routing and speed. 140 pass, 1 skipped
- The simulator measures the Mac, not a phone (0.10x real time on this one);
  a real speed needs the iPhone
- Verify: `make ios-test`. Then `make server-run`, run the app in the
  simulator, download the model on "Automatic": the speed appears by itself.
  Record: the result comes from this device with no explanation. Choose
  "Server" and back, or stop the server, to see the other cases

**Checkpoint 10.7: speak the translation** (≈ 8.1) (done)
- `SpeechSynthesizer`, and `SystemSpeechSynthesizer` on `AVSpeechSynthesizer`:
  the voice iOS uses for the language's default region, which is the one the
  user picked in Settings
- The same rules as 8.1: the text arrives first; failing to speak keeps it,
  with the reason; nothing is spoken when both languages are the same;
  `speak: false` ends at the text, as a keyboard would want
- Not carried over: retrying a voice that is still downloading. Android's
  engine fetches a voice on first use and fails that request; iOS installs
  its voices with the system and has no such state (ADR 0008 addendum)
- Choosing among Apple's regional variants is now one rule on `Language`,
  shared by the translator and the synthesizer
- Tests: Android's speaking cases for the pipeline and the view model, and
  the real synthesizer on the simulator: it speaks, `stop()` cuts it short, a
  language with no voice is reported, the default region's voice is chosen.
  148 pass, 1 skipped
- In the simulator the app never reaches this step: Apple's translation does
  not run there (10.5), and nothing is spoken when the languages are the
  same. Hearing it in the app needs the iPhone
- Verify: `make ios-test`, which speaks briefly. On the iPhone, record with a
  target language other than the source, and listen

Done on 2026-10-29. Translation and speech, which the simulator cannot run,
are still to be seen working on an iPhone. First installed on the owner's
iPhone 14 Pro on 2026-10-31, with `make ios-device`.

---

## [ ] Milestone 11: Voice keyboards

Added on 2026-10-14 at the owner's request, after a screenshot of a voice
keyboard dictating into WhatsApp on an iPhone.

A keyboard on both phones that records the user and types the original words,
in the language spoken, into whatever app is open. Confirmed by the owner on
2026-10-17, with room for typing the translation as a later option: the
keyboard goes through the same pipeline as the app, which already stops after
the text when asked not to speak, so adding translation is a setting, not a
redesign. Only the original words are built now. It uses the same recording
and transcription as the app (on the phone or on the server, by the same
routing). Translation and speech stay in the app: a keyboard can only type.

**Demo**: in WhatsApp, switch to our keyboard, speak, and the words appear in
the message field. On both phones.

Checkpoints, to be detailed when the milestone starts:

- 11.1 Android: `core/` moves into its own Gradle module; an
  `InputMethodService` with a record button that types the transcript
- 11.2 iPhone, a throwaway test on the owner's iPhone. iOS does not let a
  keyboard use the microphone, so the main app records in the background and
  passes the words to the keyboard through a shared App Group (possible since
  the owner joined the Apple Developer Program, 2026-10-31). Apple documents
  none of this. This checkpoint decides whether 11.3 happens
- 11.3 iPhone: `Core/` moves into a shared framework; the keyboard extension
- ADR 0013

---

## [ ] Milestone 12: Voice notes, interpret and reply

Dropped on 2026-10-31, when the social app brought voice notes into the app
itself (milestone 14). Brought back the same day by the owner, to come after
milestone 14: people will still receive voice notes in other apps. The
feasibility notes are under open question 8, now settled.

Added on 2026-10-29 at the owner's request. A user receives a voice note in
another app, such as WhatsApp, and reads or hears it in their own language.
They answer by speaking, and the answer goes back to the sender in the user's
own voice and the sender's language. On both phones.

The owner first pictured this through the keyboard. A keyboard cannot do
either half, on either phone: it sees only the text box it types into, so it
cannot read another app's voice notes, and it cannot insert audio (an Android
keyboard can insert images into apps that accept them, never audio; an iOS
keyboard only text). Sharing can do both. The proposed route, to confirm when
the milestone starts: the voice note comes in through the share sheet, and the
reply goes out the same way. The keyboard of milestone 11 is unchanged; its
later option to type the translation would be the text way to reply.

Most of it is built by then. The pipeline takes audio, not the microphone, so
a decoded voice note goes through the same stages and the same routing; the
reply in the user's voice is milestone 9. It needs 9.3 on both phones, and 9.4
for the reply to sound like the user rather than the fake engine's beeps or
the phone's stock voice.

```
WhatsApp voice note ──share──> PTranslate
    decode -> 16 kHz mono PCM
    transcribe, language detected       -> their words
    translate into the user's language  -> on screen
    the user records a reply
    transcribe -> translate into the note's language
    speak in the user's voice (POST /v1/speech) -> audio file
PTranslate ──share──> WhatsApp
```

**Demo**: in WhatsApp, share a voice note in another language to PTranslate
and read it in yours. Tap Reply, speak, and share the answer back to the same
chat, where it plays in your voice and their language. On both phones.

Checkpoints, to be detailed when the milestone starts:

- 12.1 A throwaway test on both real phones: share a WhatsApp voice note in,
  decode it, share a generated audio file back, and see how it arrives. This
  decides the iOS route and the audio formats
- 12.2 Interpret an incoming note (Android, then iOS): receiving the share;
  decoding to PCM; the source language detected by Whisper, on the phone and
  on the server (`source_lang=auto`), with the picker kept to correct it;
  notes longer than 30 seconds; the screen
- 12.3 Reply in the user's voice (Android, then iOS): record, translate into
  the note's language, generate the speech as a file (the server's voice, or
  the phone's own voice as the fallback), convert it to .m4a, listen, share
- ADR 0014

Known before starting:

- WhatsApp voice notes are Opus audio in an Ogg file. Android decodes that
  with `MediaExtractor` and `MediaCodec`. Whether iOS's AVFoundation opens Ogg
  is not known; if it does not, libopus and libogg would be a new dependency
- On iOS the app either registers the audio file type, so it is offered in
  the share sheet, or adds a Share Extension. The extension needs an App Group
  (open question 3); registering the file type does not
- The reply will probably arrive in WhatsApp as an audio file, not as a voice
  note with a waveform; 12.1 shows which
- `speak` plays aloud; the reply needs speech saved to a file instead. The
  server already returns a WAV, and the phones' own voices can write a file
  too (`synthesizeToFile`, `AVSpeechSynthesizer.write`)
- There are no accounts, so the app cannot know the sender's preferred
  language: the reply goes in the language the note was spoken in, and the
  user can change it
- The incoming note is someone else's voice and words. Transcribed on the
  phone, it stays there; the cloud fallback uploads it, and the screen must
  say so
- Recordings are capped at 30 seconds and uploads at 10 MB (about 5 minutes
  of WAV); voice notes are often longer than 30 seconds

---

## [~] Milestone 13: Accounts, profile, and your voice on the server

Added on 2026-10-31, when the app became a product (ADR 0015). The first
step of the social app: each person has an account, a profile others can
find, and their voice kept on the server, so that others can hear their
words in it (ADR 0016). Architecture section 9 has the direction.

**Demo**: on both phones, sign up with Apple, Google or a code sent by email;
enter your name, choose your ID and your language; record your voice and
agree to it being kept. Sign in on a second phone: the same profile and
voice are there. Delete your voice, then your account: both are gone from
the server.

**Checkpoint 13.1: server accounts, Sign in with Apple and Google** (done, 2026-10-31, but codes by email)
- Migration 0004: `users`, `identities` (`provider`, `subject`) and
  `sessions` (the SHA-256 of each token, never the token)
- `internal/auth`: Apple and Google identity tokens checked with the
  standard library: RS256 only, the provider's keys fetched and kept for a
  day, issuer, audience, expiry, subject, nonce
- `internal/accounts`: sign in (the account is made at the first sign-in, and
  two first sign-ins at once make one account), sessions, the profile, unique
  handles, deleting an account
- `POST /v1/sessions/apple`, `POST /v1/sessions/google`,
  `DELETE /v1/sessions/current`, `GET`/`PATCH`/`DELETE /v1/me`;
  `APPLE_AUDIENCE` (the bundle ID), `GOOGLE_CLIENT_ID` (off when empty)
- ADR 0018
- Waiting on the owner: a Google Cloud project with its OAuth clients, for
  Google sign-in; a domain and a mail service, for codes by email
- Verify: `make server-test`

**Checkpoint 13.2: each user's voice on the server** (done, 2026-10-31)
- Migration 0005: `voices`, one per user, with the consent version and when
  it was given; the audio in its own directory, `VOICES_DIR`
- `PUT`/`GET`/`DELETE /v1/me/voice`, `GET /v1/me/voice/audio`; replacing a
  voice deletes the old file, two uploads at once leave no file behind,
  deleting the account deletes the voice
- `POST /v1/speech` without a `voice` part, from a signed-in user, speaks in
  their kept voice; the job gets its own copy, so the kept one survives it
- Verify: `make server-test`

**Design round 2** (arrived 2026-10-31)
- `docs/design/brief-2-social.md` went to Claude Design; the handoff is in
  `docs/design/pearl/` (README, round 2 section) and `docs/design/pearl/v1/`:
  welcome, sign in, name, ID, language, voice consent, rooms, add someone (ID
  and QR), profile, every state of a voice note, errors and notices
- Built as designed, with these departures, each for a reason:
  - the city in "Madrid · hears Español" is left out: sign-up never asks for
    one (the owner's choice, 2026-10-31); rows say "hears Español"
  - the iPhone offers Sign in with Apple only and Android Google only, as ADR
    0018 says; the frame showing both on one phone is not followed
  - sign-up has 4 steps, not 5: the speech model step is not designed
  - "in her voice" becomes "in their voice": the app does not know pronouns
  - the welcome's "Hear Rosa say hello" card is left out until there is a
    recording to play
  - Terms and Privacy Policy are plain text until there are pages to link to

**Design round 3: the name Shinar, the logo and the splash** (arrived and
built 2026-11-01)
- The app is called Shinar. The handoff in `docs/design/pearl/` (README,
  "Logo and splash"; `assets/`, `brand/`) adds the mark, the app icon, the
  splash screen and a first-launch intro, the mark beside the name on the
  welcome, and the welcome's two voices in the logo's order: mint (them) on
  the left, warm white (you) on the right
- The owner chose the identifier `com.pelumi.shinar` on both phones. The
  name is written once per phone, the link in QR codes is `shinar://add/`,
  and the code keeps its names. ADR 0021
- Both phones: the icon (Android's adaptive, with a one-colour version for
  themed icons), the launch screen (the mark on petrol), then a cover that
  starts from that frame: the first time, on the way to the welcome, the
  1.5-second intro (a tap skips it; with Reduce Motion only the name fades
  in); otherwise a 250 ms fade into the app
- Built as designed, with these departures:
  - Android 8 to 11 get the splash from a window background, not the
    core-splashscreen library the handoff suggests: the same picture with
    no new dependency. Android 12 and later use their own splash screen
  - the intro's mark stays where the splash put it and the name appears
    under it, so nothing jumps between the splash and the intro; the
    handoff gives no size for the name there, so it is 40 points
  - the iPhone icon's dark and tinted versions are left to iOS, which makes
    them from the icon; the handoff designs only the one
- Verify: `make android-test`, `make ios-test`; on a phone, delete the old
  PTranslate app, install, and open it signed out to see the intro

**Checkpoint 13.3: Android, sign-up and profile** (built, 2026-10-31)
- Sign in with Google through Credential Manager; the session token sealed
  with an Android Keystore key; name, ID (checked as typed, with suggestions
  when taken), language, voice with consent; profile with play, record again
  and delete voice, edit name, ID and language, privacy, Advanced, sign out,
  delete account. Signing out leaves no voice on the phone
- `GET /v1/handles/{handle}` and `GET /v1/users/{handle}` on the server
- Waiting on the owner: the Google Cloud project's web client ID, in
  `android/local.properties` as `googleWebClientId=` and in `server/.env` as
  `GOOGLE_CLIENT_ID=`, and an Android OAuth client in the same project for
  `com.pelumi.shinar` with the debug key's SHA-1; until then the Google
  button says it is not set up
- Verify: `make android-test`; on a phone, `make android-install`

**Checkpoint 13.4: iOS, sign-up and profile** (built, 2026-10-31)
- The same on iPhone with Sign in with Apple (the entitlement is in the
  project; `make ios-device` lets Xcode add the capability to the app ID),
  the token in the Keychain
- Verify: `make ios-test`; on the iPhone, `make ios-device`

**Checkpoint 13.5: test accounts** (built, 2026-11-18, at the owner's request)
- The owner needs a second account to test rooms with; on one kind of phone
  there is one account per Apple or Google sign-in
- Server: `POST /v1/sessions/email` with an address and a code. For now only
  test accounts have codes, fixed in `TEST_ACCOUNTS` (`address:code`, codes of
  at least 12 letters or digits); any other address is refused like a wrong
  code. ADR 0018, addendum
- Both phones: holding "Email sign-in is coming soon." on the sign-in screen
  opens a form for the address and code; then sign-up goes on as usual
- Two test accounts are in `server/.env` (`test1@shinar.test`,
  `test2@shinar.test`), for the server on this Mac
- Owner: for TestFlight builds, add the same `TEST_ACCOUNTS` line to
  `/etc/shinar/env` on the Azure machine and restart the server, after
  shipping this server
- Before the App Store (milestone 18): describe the hidden entry in the
  review notes, or replace it with the visible email sign-in
- Verify: `make server-test`, `make android-test`, `make ios-test`; on a phone,
  hold the line, sign in as `test1@shinar.test`

Still to come: codes by email (a domain and a mail service); revoking Apple
tokens when an account is deleted (milestone 18).

---

## [~] Milestone 14: Rooms of two

Renamed on 2026-10-31, when the owner asked for the part where people
connect: "rooms for just two users and later a group user, but let's start
with just a room of two". A room is a conversation. In this milestone it has
two people; group rooms are milestone 20. It is the conversation screen of
the design (`docs/design/pearl/02-conversation.png`).

**Demo**: two phones, one set to English and one to Spanish. One adds the
other by ID (or QR code) and a room opens for the two. It opens at once: the
first voice note reaches the other person with no request to accept (the
owner's choice, 2026-10-31); blocking and reporting come in milestone 16. A voice message in
English plays on the other phone in Spanish, in the sender's voice, with the
original words a tap away. The reply comes back the other way. With both in
the room, each message appears as soon as it is ready.

**Checkpoints 14.1 and 14.2: server rooms, delivery and live changes** (done, 2026-10-31)
- Migration 0006: rooms of two (found by their pair), members with when they
  last read, messages, and renditions, one per message and listener language
- A `deliver` job per message and language on the existing queue: translate
  with Google Cloud Translation (the owner's choice, 2026-10-31), save the
  translation, speak it in the sender's kept voice; listeners of the same
  language get the recording itself; no kept voice means the listener's
  phone reads the words. ADR 0020
- `POST`/`GET /v1/rooms`, messages, read, retry, audio per language, and
  `GET /v1/me/events`, a live stream of changes per user over `NOTIFY`
- `TRANSLATE_ENGINE=fake` for working without a key; `GOOGLE_TRANSLATE_KEY`
  for the real one, waiting on the owner
- 2026-11-09: Google could not be billed (an expired card stuck on the
  payments profile), so the owner chose Azure AI Translator on its free tier,
  beside Google rather than instead of it (ADR 0020, addendum). The owner's
  key is in `server/.env`; `make translate-check` gave good English to
  Yoruba, Hausa, Spanish and Chinese, and Yoruba to English
- Owner, by about 2026-12-09: the Azure free account lasts 30 days. Upgrade
  it to Pay as you go before then, or Azure disables the subscription, and
  with it both the translator and the hosted server (ADR 0023), which share
  it. Upgrading keeps the free amounts: the Translator stays on the free F0
  tier, so it still costs nothing
- The privacy page on both phones names the translator (Microsoft's Azure
  AI Translator since 2026-11-09); switching engines means changing it too
- Verify: `make server-test`

**Checkpoints 14.3 and 14.4: rooms on both phones** (built, 2026-10-31)
- Your rooms with unread dots, and the empty state with your ID to share;
  add someone by ID or by QR code (show yours, scan theirs, or open a
  timbre://add link); a room with every state of a note: writing down,
  sending, translating, preparing your voice, delivered, failed with retry,
  the phone's voice; theirs new, playing (the words light up), played, with
  the original a tap away; tap to record, tap to send, cancel, 30 seconds
  with a countdown from 25; hints when a recording was too short, silent or
  unclear; the microphone banner; "Reconnecting" while the stream is down
- Not yet: the dots that follow the microphone's level; the empty room and a
  listener's note still on its way are not in the design and are kept plain
- Verify: two phones, one English and one Spanish, the server running with
  `TRANSLATE_ENGINE=fake` (or a Google key)

**Checkpoint 14.5: push notifications, for a phone not in the room** (built, 2026-11-15)
- Decided on 2026-11-15: Apple's push service (APNs) directly for the
  iPhone, with the key from the developer account, and Firebase Cloud
  Messaging for Android (`firebase-messaging` and the `google-services`
  plugin, approved the same day). Not FCM on the iPhone too: ringing an
  iPhone for a call (milestone 17) needs a VoIP push, which only APNs
  delivers. ADR 0026
- Server: migration 0007 (a push address per session, a `push` job kind);
  `PUT /v1/sessions/current/push`; a `push` job added in the transaction
  that makes a note ready for a language, sent by the workers to every
  phone of its listeners; addresses the services call gone are forgotten;
  `internal/push` signs its own tokens with the standard library;
  `make push-check`
- Both phones: register the address whenever someone is signed in; the
  notification reads "Rosa" / "Sent a voice note · 0:14" (the brief's
  wording, not the words of the note); not shown while its room is on the
  screen; tapping it opens the room; opening a room clears its
  notifications; permission is the system's own question, the first time
  the rooms are shown; the privacy page says what the notification carries
- Not yet: the screen before the permission question (brief 3.11, not
  designed); a badge with the unread count
- Owner: the APNs key (Key ID to Claude, the file in `~/keys/shinar/`) and
  Push Notifications on the App ID. The Firebase key is in place and passes
  `make push-check`
- Verify: `make server-test`; `make push-check`; two phones signed in, one
  app in the background: a note from the other shows a notification, and
  tapping it opens the room

**Fix after 14.4: dates in a room** (2026-11-23, at the owner's request)
- The cards showed a time, and only on the user's own messages, so older
  conversations had no dates. A heading now goes over the first message of
  each day: "Today", "Yesterday", the weekday within the week, else the date,
  with the year when it is not this one. iPhone only for now; Android in
  milestone 22
- Verify: `make ios-test`; a room with messages from more than one day

**Checkpoint 14.6: recording with pause, stop and send** (built, 2026-11-18,
at the owner's request: "record should have pause, stop and send; it
currently just has stop, which also sends")
- While recording: cancel, pause (the microphone is off; resume carries on),
  stop, and send. Stop ends the recording without sending: the note can be
  played back, then deleted or sent. Only time spent recording counts toward
  the 30 seconds; at the limit the note stops and waits to be sent, where
  before it was sent by itself
- `Recording.pause()` and `resume()` on both phones (Android stops the
  `AudioRecord` on its capture thread; iOS pauses the `AVAudioEngine`);
  `NoteRecorder` holds the composer's note, with the same tests on both
- Not designed: the handoff's composer is "tap to start, tap to send". The
  controls are built from Pearl's parts (the outlined round buttons, the
  send circle in its halo, the dots); a design pass can follow
- The "too short" hint now reads "Tap the mic, say something, then send.",
  since tapping again no longer sends
- Verify: `make android-test`, `make ios-test`; in a room, record, pause,
  resume, stop, listen, then send

---

## [~] Milestone 15: Text messages

**Demo**: type in English; the other side reads it in Spanish, and can tap
it to hear it in the sender's voice.

Asked for by the owner on 2026-11-18, "for more comfort", ahead of milestone
12.

**Checkpoint 15.1: server** (done, 2026-11-18)
- Migration 0008: `messages.kind` (`voice` or `text`); a text message has no
  recording. Sent as JSON to the same endpoint, with the same idempotency key
- The delivery job translates it and makes it ready with the words alone, and
  the phones are told: "Sent a message", not the words. Its voice is made only
  when someone asks, `POST /v1/messages/{id}/speech?lang=`, by a second job,
  once per language, then kept; not pushed again. ADR 0027
- Verify: `make server-test`

**Checkpoint 15.2: both phones** (built, 2026-11-18)
- The composer: a message field beside the mic; with words in it, the mic
  becomes send. A text waits on its card ("Sending", failed with retry) like a
  voice note
- Their card: the translation as soon as it is made, the original a tap
  away, and a listen button: the first tap asks for the sender's voice, the
  button breathes while it is made, then it plays, the words lighting up. My
  card: "Delivered", and "Hear it in Spanish"
- The rooms list shows a text's translation, or "Message" until it is made;
  an empty room says "Say something to Rosa, or write it"; the privacy page
  covers typed messages and the new notification
- Android: the room keeps the composer above the keyboard. The app's insets
  are now consumed once, at the top, which also takes away a gap the height
  of the navigation bar above the keyboard on the sign-up steps
- Not designed: text messages are in the brief's "later" list. Built from
  Pearl's voice note cards and field
- Not checked yet: on two phones, against the server
- Verify: `make android-test`, `make ios-test`; two phones, one English and
  one Spanish: type, read, listen

---

## [~] Milestone 16: Safety and discovery

Block and report, and a way to act on reports; then finding people beyond
those whose ID you know, including at random. Block and report come first:
both app stores require them in apps where people contact each other, and
contact at random invites abuse without them.

Split on 2026-11-21: block and report now, for the App Store, on the iPhone
(milestone 22 brings them to Android); discovery after the launch.

Apple's Guideline 1.2 asks four things of apps where people contact each
other: a way to filter objectionable material, reporting with a timely
answer, blocking, and published contact details. The contact details are on
the help page already (ADR 0032).

**Checkpoint 16.1: block and report on the server** (built 2026-11-23)
- Migration 0011: `blocks` (who blocked whom, and when); `reports` (who
  reported whom, the reported person as they were, the reason, a note, a copy
  of the message, when, and when and how it was closed); `users.suspended_at`;
  a fifth kind of job, `report`, with `jobs.report_id`
- Blocking: in a room of two, a block either way stops the room. Neither can
  send into it, nor have a failed note tried again, until the blocker
  unblocks; a resend of a note kept before the block is answered as before.
  `POST /v1/blocks`, `DELETE /v1/blocks/{user_id}`, `GET /v1/me/blocks`
- The person blocked is not told, a change from the plan's "a room says by
  whom": their sends get `403 cannot_send` and no reason, their room looks
  as before, and only the blocker's stream hears of the block. The blocker's
  room says `you_blocked`, and their sends get `409 you_blocked`. Opening a
  room with the blocker is not refused either, since that would tell them;
  nothing can be sent into it
- Reporting a person, or one of their messages, someone the reporter shares a
  room with: `POST /v1/reports`, with a reason (spam, harassment, hate,
  sexual, violence, self_harm, other) and an optional note of up to 500
  characters. It blocks too, unless `block` is false. The message is copied
  into the report as the reporter saw it: its words, its translation into
  their language, and the recording, into a folder of its own
  (`REPORTS_DIR`). The copy outlives the message and its sender's account,
  and goes when the report is closed
- The owner hears of each report at once: a `report` job, added with it, sends
  a push notification to every phone of the account named by
  `REPORTS_NOTIFY_HANDLE`, "New report" / "A voice note · harassment", never
  the words, retried like any notification. The iPhone opens nothing when it
  is tapped, since it names no room
- The owner's commands on the machine: `make reports-list`,
  `make report-close ID=... NOTE=...`, `make account-suspend HANDLE=...`
  (signed out everywhere, cannot sign in, `403 account_suspended`, and
  cannot be found by ID) and `make account-delete HANDLE=...`, which only
  says what would go until run again with `YES=1`, and deletes through the
  same code as deleting an account in the app. `deploy/waitlist.sh` is now
  `deploy/command.sh`, since it runs both kinds
- Filtering (agreed by the owner, 2026-11-21): Azure translates with
  `profanityAction=Marked`, writing `***` for an offensive word; the voice
  made from the translation leaves those out, and a translation with nothing
  left to say goes to the listener's phone. Same-language rooms, with no
  translation, are not filtered
- ADR 0035
- Tests: 26 new, in the rooms store (blocks both ways, resends, who sees the
  block, the list, retries, the stream telling only the blocker; a report
  outliving its sender's account, blocking unless asked not to, the shared
  room, the owner's notice, closing; masked words left out of the voice),
  the API (every answer of the new endpoints, the refusals, a suspended
  sign-in, deleting an account leaving the report's copy), suspension in
  accounts, the queue's new job, and the commands. Also checked: the
  migration on this Mac's database with its existing jobs; lint, and the
  changed packages under the race detector
- Owner, before this server is pushed to `main` (which ships it): on the
  machine, create the folder, which the server cannot create itself since
  `/var/lib/shinar` is root's, with
  `sudo install -d -m 750 -o shinar -g shinar /var/lib/shinar/reports` (or run
  `deploy/setup.sh` again, which is safe); and add
  `REPORTS_DIR=/var/lib/shinar/reports` and `REPORTS_NOTIFY_HANDLE=<your ID>`
  to `/etc/shinar/env`. Without the folder the server stops at start. Then
  stay signed in on your iPhone with notifications on
- Verify: `make server-test`

**Checkpoint 16.2: block and report on the iPhone** (built 2026-11-23)
- In a room, a menu at the top in place of the call button: Report Rosa, and
  Block Rosa (asked once: "Neither of you will be able to send messages here
  until you unblock them. They won't be told.") or Unblock. Press and hold one
  of their messages: Report this voice note, or this message
- The report sheet: the reason (spam or a scam, harassment or bullying, hate,
  sexual content, threats or violence, someone may be at risk, something
  else), an optional note of up to 500 characters, "Also block Rosa", on by
  default, and what is kept and that every report is looked at within a day.
  Then "Thanks for telling us"
- A blocked room says so in place of the composer, with Unblock. Profile,
  under Privacy, lists the people blocked, each with Unblock
- A refused send tells only the blocker why; the person blocked sees the usual
  failed card. A suspended account's sign-in says it has been suspended
- The phone's own voice, reading a translation, leaves out the `***` the
  translator writes for an offensive word, as the server's voice does, with
  the server's test cases
- What is kept: the iPhone's privacy page ("Reports and blocking", and
  Deleting), the website's privacy page, help page and terms, and the
  marketing brief. They describe the build this checkpoint makes, which
  reaches testers through TestFlight when it is pushed
- Not designed: built from Pearl's list rows, menus and banner, as text
  messages were; a design pass can follow
- Tests: the client's new calls (block, unblock, the list, both kinds of
  report and their bodies), a room's blocked flag from a server with and
  without it, the refusals, and the masked words. iOS 209 pass, none
  skipped; the website builds
- Not checked yet: on a phone against a server, which needs 16.1 shipped
- Verify: `make ios-test`; then, with the server on this Mac, the iPhone and
  the simulator signed in as the two test accounts: report a note from the
  other, see the room blocked, send from the blocked side (it fails with no
  reason), unblock from Profile

---

## [-] Milestone 17: Calls, for premium users

Dropped for now on 2026-11-21: the owner wants the App Store launch to focus
on the features the app already has. The call button goes (checkpoint
18.11), and the hands-free mode below waits with the calls. Billing comes
without them, in milestone 21. What follows is kept for when calls come back.

Live calls, each side hearing the other in their own language and the
speaker's voice. Starts with a research checkpoint: recognising, translating
and speaking as a stream, within about two seconds, and what a minute costs.
Premium only, so it brings billing: in-app purchases on both stores.

Measured and estimated on 2026-11-15: with today's batch engines (Groq,
Azure, Fish Audio) a phrase reaches the listener 5 to 8 seconds after it
ends; streamed at every stage, about 2 to 3. It will feel like talking
through an interpreter, in turns, not like a WhatsApp call. About 4 to 5
cents a minute of conversation. It also needs a server that relays live
audio, the phones' call screens (CallKit, ConnectionService) and VoIP push.

**A call button in each room** (built, 2026-11-18, at the owner's request):
a phone icon at the top of the room says "Calls are coming soon", and to send
a voice note or a message until then. No premium wording: billing is not
built.

**Before calls: a hands-free mode in rooms** (asked by the owner on
2026-11-15). Hold to talk, release to send; the other phone plays the note
as it arrives, in the listener's language and the speaker's voice: a
translated walkie-talkie. Built almost entirely from the rooms' existing
pieces, it gives most of the feel of a call for little cost, and shows
whether people want calls before they are built. Within the usage limits.

---

## [ ] Milestone 18: Going live

A public server (hosting, a domain, HTTPS, storage for audio), a privacy
policy and terms that cover voices, and listings on the App Store and Google
Play. Before the first outside users, and after milestone 16, since the
stores require block and report.

**Checkpoint 18.1: iPhone builds to TestFlight** (built 2026-11-02, at the
owner's request, ahead of the rest of this milestone)
- A GitHub Actions workflow on every push to `main` that changes the iPhone
  app, or by hand: the iOS tests, then a signed App Store build uploaded to
  TestFlight, for testers the owner invites through TestFlight groups. The
  owner chose GitHub Actions over Xcode Cloud. ADR 0022
- `make ios-testflight` runs the same upload from a Mac
- Signed with a distribution certificate and profile kept as secrets; an
  App Store Connect API key for the upload. The server address is the
  repository variable `BACKEND_URL`
- Info.plist answers the encryption question; `PrivacyInfo.xcprivacy` gives
  the reasons for `UserDefaults` and file attributes
- Checked here: the workflow parses, and an unsigned Release archive builds
  with the address passed in, carrying the privacy manifest. Not yet run on
  GitHub: it needs the owner's set-up (ADR 0022, "Setting it up")
- Waiting on the owner: the app in App Store Connect, the certificate, the
  profile, the API key, and the GitHub secrets and variable
- Testers reach the server only on the owner's Wi-Fi until it is hosted;
  external TestFlight groups wait for that too

**Checkpoint 18.2: the server on Azure** (built 2026-11-09, at the owner's
request, ahead of the rest of this milestone)
- The owner asked for free hosting, to pay for as it grows, and chose
  Azure's 12-month free services: a B2ats v2 Ubuntu machine and a managed
  Postgres 18 (B1ms) in South Africa North. ADR 0023
- `deploy/`: the one-time set-up (Caddy for HTTPS, the service, folders, the
  speech model, a firewall, a deploy user), the service and Caddy files, the
  settings template, and `ship.sh`. `make server-ship`
- GitHub Actions on every push to `main` that changes the server: lint, the
  tests against Postgres 18 beside them, then ship over SSH
- Transcribing and speaking now need a signed-in user, and both phones send
  their token: a public server would otherwise spend the Fish Audio credit
  for anyone. Server, Android (163) and iOS (185, 1 skipped) tests pass
- Checked here: the scripts parse, the workflow is valid, the server builds
  for Linux
- Live since 2026-11-09 at
  `https://shinar-api.southafricanorth.cloudapp.azure.com`: `setup.sh` and
  the first `make server-ship` ran on the machine, `/healthz` answers over
  HTTPS (Let's Encrypt), the migrations ran on PostgreSQL 18.6, plain HTTP
  redirects, and transcribing and speaking answer 401 without a sign-in.
  The GitHub workflow has not run yet: it needs the secrets
- Fixed 2026-11-11: the first TestFlight builds, archived in Xcode, still
  talked to the Mac (`http://OgundipePelumi.local:8080`) and reached no
  server on testers' phones. Release builds of both apps now talk to the
  hosted server, and `testflight.sh` refuses a build whose server is not
  `https://`. ADR 0022, addendum
- Known limits: the files on the machine's disk are not backed up yet; one
  machine, so a deploy pauses the server for a few seconds

**Checkpoint 18.3: iPhones from iOS 18, and ML Kit on iOS** (built
2026-11-10, at the owner's request, ahead of the rest of this milestone)
- The app needed iOS 26, so anyone who had not updated, and every iPhone XS
  and XR, could not install it. The owner does not want people to have to
  update iOS. It now needs iOS 18.0 (ADR 0025)
- The iPhone translates with ML Kit, as Android does: the same 59 languages,
  packs of about 35 MB downloaded by the app, no iOS sheet, no screen needed.
  Apple's translator, `TranslationDownloads` and the root view's download
  hook are gone (ADR 0024, replacing 0012). The owner approved the
  dependencies on 2026-11-10
- `make ios-mlkit` fetches Google's three ML Kit files from Google, checks
  their SHA-256, and repackages them for Xcode with a simulator copy; the
  local package `ios/MLKit` links them with GTMSessionFetcher,
  GoogleDataTransport, GoogleUtilities and ZipArchive (Swift packages) and
  GoogleToolboxForMac 4.2.1 (built from source). No CocoaPods
- For iOS 18: `nextChange` in place of `Observations`, a plain `deinit` for
  the Whisper actor, and the bordered button where iOS 26 has Liquid Glass
- iPhone XS and XR: their A12 lacks instructions the Whisper build uses, so
  they never load Whisper and send every recording to the server
  (`WhisperSupport`); Advanced settings says so
- Checked here: iOS tests pass on the iOS 26.4 simulator and on the iOS 18.1
  simulator (187 each, none skipped; ML Kit translates for real in the
  simulator). An unsigned App Store archive builds with no embedded
  frameworks, one dSYM and every privacy manifest; the app is 34 MB, about
  31 MB of it ML Kit
- Not checked yet: ML Kit on a real iPhone, the TestFlight upload with the
  new libraries, and any iPhone XS or XR
- For the store listings (later in this milestone): ML Kit reports usage and
  diagnostics to Google, not linked to the user, on both phones

**Checkpoint 18.4: the website** (built 2026-11-18, at the owner's request,
ahead of the rest of this milestone)
- The owner asked for a motion piece of a voice note's journey, from one
  person to another, and the same idea on a website. The brief for both, for
  Claude Design: `docs/design/brief-4-motion.md`
- The owner chose Next.js. `web/`, exported as plain files into `web/out/`;
  no server runs it. Dependencies: Next.js, React, and TypeScript for
  development; no CSS framework or animation library (Motion came in
  checkpoint 18.10). ADR 0028
- The home page: the promise, and a loop of the core turn (her note in
  Spanish, warm white, becomes his in English, mint, in her voice); the
  journey in seven stages while scrolling, each drawn as the screen it
  happens on with the app's own states and words; what is kept of your
  voice, and why; "Coming to iPhone and Android". Pearl, dark only; built
  from Pearl's parts, without a design of its own yet
- Reduce Motion: every animation stops at its end state
- "Hear Rosa say hello" appears when its two audio files are in
  `web/public/sample/` (`web/README.md`); there is no real audio yet
- `make web-dev`, `make web-build`, `make web-preview`
- Owner: real audio for the sample, from someone who agrees; the domain and
  the host; what the page asks visitors to do before the app is on the
  stores. When Claude Design answers brief round 4, the page follows it
- Verify: `make web-preview`, then open http://localhost:3100 at a phone's
  width and a desktop's; scroll the journey down and back up

**Checkpoint 18.5: daily limits** (built 2026-11-19, at the owner's request,
before the soft launch)
- The owner chose 10 minutes of voice notes and 100 text messages a day for
  each person, and to stay on the engines' free tiers. ADR 0029
- Server: a message past the limit is refused with `429`
  (`voice_limit_reached`, `text_limit_reached`) and nothing of it is kept.
  Counted from the person's messages of the last 24 hours, in the
  transaction that adds the new one, with their row locked so two at once
  take turns; a resend with its key is never refused. Migration 0009 (an
  index). `DAILY_VOICE_MINUTES`, `DAILY_TEXT_MESSAGES`
- Both phones: the room says why ("You've sent as many voice notes as one
  day allows. You can send more later, or write a message."), and the note
  stays failed on its card, to send later. The rule is
  `RoomViewModel.refusal`, with the same cases on both phones
- Not yet: the interpreter's server work (Groq, `POST /v1/speech`), which
  jobs cannot tie to a person; showing what is left (not designed); a
  monthly budget for the whole server, needed before any paid tier
- Tests: the store (each limit, a note that would pass it, the other kind and
  the other person still allowed, resends, the rolling day, eight at once
  under the race detector) and the API's answers; one case on each phone
- Verify: `make server-test`; or `DAILY_TEXT_MESSAGES=2 make server-run`,
  and on a phone send three messages: the third says why it was not sent

**Checkpoint 18.6: the waitlist** (built 2026-11-19, at the owner's request)
- The owner chose a waitlist on our server, for TestFlight invitations, iPhone
  only for now; Android sign-ups are kept. ADR 0030
- Server: `POST /v1/waitlist` (email, phone, optional first name), open to
  anyone, the same answer for a new or a known address, a hidden field for
  bots. Migration 0010. `-waitlist-invite N`, `-waitlist-count`,
  `-waitlist-list`, `-waitlist-remove`, run on the machine by
  `make waitlist-invite N=20` (writes `invitations.csv` for App Store
  Connect), `make waitlist-count`, `make waitlist-list` (everyone, invites
  nobody) and `make waitlist-remove EMAIL=...`
- Website: "Try it first" in the hero leads to the form at the end of the
  page: iPhone or Android, email, first name, and what is done with the
  address. `make web-preview` passes `/v1/` to the server on this Mac
- Hosting: the site is on Vercel (ADR 0031), a project with its Root
  Directory `web`; `web/vercel.json` passes `/v1/waitlist` on to the server.
  Caddy on the machine serves only the API, as before
- Checked here: the endpoint and the three commands against the server on
  this Mac, and a sign-up sent through the built site. The page was not
  looked at in a browser
- Owner, before sharing the link: an address for removal requests
  (`CONTACT_EMAIL` in `web/lib/site.ts`); ship the server; the Vercel
  project (Root Directory `web`), then a sign-up through it; an external
  TestFlight group, whose first build goes through Apple's beta review; and
  a domain, when wanted
- Verify: `make server-test`; `make server-run` and `make web-preview`, then
  join the list at http://localhost:3100 and `-waitlist-count` shows it

**Checkpoint 18.7: help, privacy and terms; questions on the home page**
(built 2026-11-19, at the owner's request)
- The tester FAQ from the marketing work, with who can try it and what it
  costs, on the home page (`#faq`); `/support`, `/privacy` and `/terms`,
  linked from every page's footer. ADR 0032
- The owner decided: 16 and over; the pages name "Shinar" and the contact
  address, no person or company for now
- The waitlist's hidden field is renamed so no browser autofills it
- Checked here: the build, and every page and link through `make web-preview`.
  Not looked at in a browser
- Owner, in App Store Connect: TestFlight, Test Information: the privacy
  page as Privacy Policy URL, the home page as Marketing URL, a feedback
  email; Beta App Review Information: a contact, and two test accounts
  (ADR 0018, addendum) so the reviewer can send a note between them. For the
  App Store later: the help page as Support URL, the privacy page, the age
  rating (16+), the App Privacy answers
- Still needed for the App Store: block and report in the app (milestone 16);
  the governing law and who runs Shinar, in the terms; a way to delete an
  account by ID on the server, for requests by email
- Verify: `make web-preview`, then http://localhost:3100/#faq, `/support`,
  `/privacy` and `/terms`

**Checkpoint 18.8: the language barrier as the theme** (built 2026-11-19, at
the owner's request)
- The promise becomes "Break the language barrier. Be heard in your own
  voice.", its sub-line "Talk to anyone. You speak your language. They hear
  theirs, in your voice.": on the Welcome screen of both phones and the
  website's hero and title. ADR 0033
- The website: the journey "crosses the language barrier" between two people
  who don't share a language; a first question, "What does Shinar do?"; the
  waitlist asks people to join with someone they don't share a language
  with; the footer's name story ends with Babel and the language barrier
- The marketing brief, design brief round 4, Pearl's Welcome screen and
  CLAUDE.md carry the theme and the new promise; the tone rule no longer
  steers away from the phrase, only from abstractions around it
- Checked here: the website's build; the Swift file parses. The phones were
  not built or looked at (one string each)
- Verify: `make web-preview`, then http://localhost:3100; `make ios-build`
  and `make android-build`, and the Welcome screen on each

**Checkpoint 18.9: the website after looking at two others** (built
2026-11-20, at the owner's request)
- The owner pointed at two apps that translate speech (voicelink.tech,
  1voice.ai) and a go-to-market report on the site (explee.com), to improve
  the website, its motion and the marketing brief. The report sits behind a
  browser check and was not read; the brief says where it goes
- The home page gains: a header that stays at the top, with "How it works",
  "Questions" and "Try it first", on every page; under the hero's buttons,
  "See how it works" and "Free while we test. On iPhone, by invitation."; the
  Welcome screen's two voices, large, behind the loop; "Made for people a
  language apart" (three relationships, only Rosa's naming languages); "Three
  ways to talk" (voice notes, text messages, the interpreter, each drawn from
  the app's parts); how joining goes, in three steps, above the form, on the
  deepest layer
- Motion, from Pearl's own vocabulary: the mark's splash intro plays once in
  the home page's header; the hero's words rise as the page opens, and the
  sections rise 8 and fade in as they come into view (the wordmark's
  entrance). Reduce Motion: nothing moves or hides; nothing is hidden without
  JavaScript either
- The journey works out its step from where the steps are, so a jump down
  the page and back lands on the right one
- A link preview picture, `/og.png`, drawn at build time from the page's
  words; the site's address is `SITE_URL` in `web/lib/site.ts`
- The marketing brief: section 11, the apps around Shinar, what sets it
  apart and what not to borrow; lines for posts; the site's new parts
- Text messages are on the page, as in the brief, though checkpoint 15.2 has
  not yet been checked on two phones; if that check fails, the card comes off
- Checked here: the build; the page at a phone's width and a desktop's, under
  Reduce Motion, and the privacy page, in headless Chrome screenshots; the
  picture. Not looked at in a real browser
- Verify: `make web-preview`, then http://localhost:3100 at a phone's width
  and a desktop's; scroll down and back up; open http://localhost:3100/og.png

**Checkpoint 18.10: the website's motion, with Motion** (built 2026-11-21, at
the owner's request)
- The owner pointed at motion.dev, motion-primitives.com,
  interfacecraft.dev and wisprflow.ai/notetaker to make the website better,
  and chose to add Motion, the animation library, though ADR 0028 had left
  one out. Not motion-primitives: it needs Tailwind. ADR 0028, addendum
- Motion is `framer-motion` 14.10.1, its React package's first name: the
  newer `motion/react` imports all of it at once, which kept the build from
  leaving out what the site doesn't use (57 KB more at first load, not
  23 KB, gzipped). Its layout features load after the page is shown
- The journey, on a wide screen: one phone stays beside the steps and only
  its screen changes, rising with the scroll going down and falling going
  back; Rosa's note is one card from screen to screen, moving and resizing
  into place, and changing side (warm white to raised petrol) as her words
  become English; the place line cross-fades. A rail beside the steps fills
  to the middle of the window and lights each step's point; a step is active
  once its count has passed the middle, so its words are beside the phone
- The questions slide open and shut, the plus turning into a minus; without
  JavaScript, or under Reduce Motion, they open as before
- The waitlist: the chosen phone's capsule slides to the other choice; the
  Android line eases open; the form gives way to "You're on the list"
- The hero's promise is written down word by word as the page opens, the
  app's "words arriving"; buttons give a little when pressed
- Reduce Motion: Motion keeps only fades; the CSS animations stop as before
- Checked here: the build and its types; in Chrome, on the built site at a
  desktop's width and a narrow one: the card's move and change of colour
  measured frame by frame, the questions opening, closing and turned round
  mid-slide, the capsule sliding, joining with the network call stubbed in
  the page (nothing was sent). Not checked: a real phone, Safari, Reduce
  Motion switched on
- Verify: `make web-preview`, then http://localhost:3100 at a desktop's
  width: scroll the journey down and back up, open and close a question,
  switch iPhone and Android; then at a phone's width

Planned on 2026-11-21, for the first App Store version, after the first part
of milestone 16 and milestone 21. iPhone only.

**Checkpoint 18.11: the rest of what App Review asks for**
- Asking before words leave the phone. Guideline 5.1.2(i), since November
  2025: an app must say which outside AI services get people's data, and ask
  first; a privacy page alone is not enough, and Apple has rejected apps for
  it. One step in sign-up, before anything is sent, naming each service and
  what it gets: Groq writes the words down when the phone cannot, Microsoft's
  translator translates them, Fish Audio speaks them in your voice. The
  voice step already asks for Fish Audio. People who signed up before it see
  the step once
- Deleting an account revokes its Sign in with Apple (Guideline 5.1.1(v),
  ADR 0018). The phone sends Apple's authorization code at sign-in; the
  server exchanges it for a refresh token, keeps it, and revokes it when the
  account is deleted. Needs a Sign in with Apple key (the owner, in the
  developer account)
- The call button goes on Android too, since milestone 17 is dropped for
  now: a few lines, so Android does not wait for milestone 22. The iPhone's
  went in 16.2, where the room's menu took its place
- The terms: Shinar is run by the owner's company, a private company limited
  by shares registered in Nigeria, under Nigerian law (the owner's choice,
  2026-11-22; its name to come), named as the App Store names the seller;
  and the subscription: the price, the trial, renewal, and cancelling and refunds
  through Apple. The privacy pages: what Apple tells us of a purchase (the
  product and its dates, never card details)
- The two test accounts stay behind the hidden entry (13.5); the review
  notes say how to reach it
- Verify: `make server-test`, `make ios-test`; on the iPhone, sign up afresh,
  then delete the account: Shinar leaves the Apple account's list of apps
  using Sign in with Apple

**Checkpoint 18.12: paid engines, within a monthly budget**
- Paying people cannot be cut off by a free allowance that everyone shares.
  The owner moves each engine to its paid tier: Fish Audio's paid model with
  credit (open question 2), Azure AI Translator on S1, Groq's paid tier
- The monthly budget per engine that ADR 0029 asks for before any paid tier.
  The server counts what it sends to each engine in a month. At 80% the
  owner is told, by a push notification to their account. Past the budget
  the server stops calling that engine: speech falls back to the phone's
  voice, and translations wait until the owner raises the budget. A setting
  per engine
- Verify: `make server-test`

**Checkpoint 18.13: audio off the machine's disk**
- Recordings, kept voices and the speech made from them are on the machine's
  disk, with no backup (ADR 0023). Before paying people trust it with their
  voices, they move to Cloudflare R2 (the owner's choice, 2026-11-21). Its
  S3 API is called with `net/http`, the requests signed with the standard
  library: no Go module. One bucket in Western Europe, between Lagos and
  Madrid. A command moves the files already kept, and the phones download
  straight from R2 through short-lived signed links, so downloads cost
  nothing and do not pass through the machine
- What it costs, at 30 minutes of voice notes a month per sender,
  compressed: nothing within the free 10 GB; after a year, about $2.50 a
  month for 1,000 senders, $30 for 10,000 and $320 for 100,000 ($0.015 per
  GB a month, downloads free). Under a cent a month per subscriber
- The owner: a Cloudflare account, R2 switched on (it asks for a payment
  method), the bucket, and an API token for that bucket alone, into
  `/etc/shinar/env`
- Audio is kept compressed (AAC), about 8 times smaller than WAV. The iPhone
  encodes with its own encoder before uploading, and Fish Audio is asked for
  compressed audio. The server accepts both, for Android's builds until
  milestone 22
- Verify: `make server-test`; on the iPhone, an old note and a new one both
  play

**Checkpoint 18.14: the listing and the submission**
- The name's trademark check (ADR 0021), before the name is on the store
- The owner, in App Store Connect: the name, subtitle, description (with the
  Terms of Use link Apple asks of subscription apps), keywords, screenshots,
  the help page as Support URL, the privacy page, the App Privacy answers,
  the age rating (16+), and the review notes (the two test accounts and how
  to reach their sign-in; reviewers subscribe in the sandbox). The
  subscriptions are submitted with this version
- Claude: the App Privacy answers, from what the app and the server do; the
  description and the review notes, to the marketing brief; and a run on two
  iPhones against the hosted server of everything a reviewer will try:
  sign-up, the voice, a room, voice notes and messages, notifications, block
  and report, buying, the trial's cap, restoring, deleting the account.
  Checkpoint 15.2 gets its check on two phones then
- The website and the marketing brief: the price and the trial, said
  plainly; "Free while we test" goes; the App Store badge once it is live
- The soft launch's testers get the build with the paywall first, on
  TestFlight, where buying costs nothing; their welcome note says so
- Verify: Apple's review

Asked on 2026-11-02 and put off by the owner: telling people when a new
version is out. Two ways were weighed: our server answering with the latest
and minimum build for each phone (works on TestFlight and both phones, a
minimum can force an update), or Apple's public App Store lookup (no server
work, but iPhone only, App Store releases only, and no minimum).

Asked on 2026-11-15, to come before the app is opened to the public: usage
limits, so the paid engines (Fish Audio, Azure AI Translator, Groq) cannot
run up a bill. The owner's examples: a voice note of at most a minute, and
an hour of conversation a day. Also: people would pay for access, with
limits. Built in part on 2026-11-19, as checkpoint 18.5: 10 minutes of voice
notes and 100 messages a day, on the free tiers (ADR 0029). What was
weighed then:
- The limits must be kept by the server, not only shown by the phones: a
  signed-in user can call the API without the app. The phones show what is
  left and stop recording at the limit; the server refuses past it.
- What is counted per user and day: seconds of audio transcribed on the
  server, characters translated, characters spoken in a kept voice. Plus a
  monthly budget for the whole server per engine, past which it stops
  calling that engine (messages then use the phone's voice), so a bug or an
  abuser cannot spend more than the budget.
- Recordings stop at 30 seconds today, one Whisper window. A minute needs
  the on-device transcriber to take two windows.
- Paying for access is billing, which CLAUDE.md keeps out until calls
  (milestone 17). Moving it earlier is the owner's decision, and needs the
  App Store's and Google Play's in-app purchase rules looked at first.

---

## [x] Milestone 19: The design

Added on 2026-10-31: the owner hands the app's design to Claude Design, to
make it look great before more features are built. The brief is
`docs/design/brief.md`: the product, its users, both platforms, every state
of a translation with real timings, the screens of version 1, and what to
ask for.

**Checkpoint 19.1: the design** (done)
- The owner chose "Lagoon / Pearl" (working name Timbre) from Claude Design
  on 2026-10-31. The handoff is in `docs/design/pearl/`: tokens, components,
  and three screens (recording your voice, a conversation, the interpreter)

**Checkpoints 19.2 and 19.3: tokens, components, and today's screens**
(Android, then iOS) (done)
- The palette, IBM Plex Sans bundled and scaled with the phone's text size,
  spacing and radii (ADR 0019). Always dark, the one mode designed
- Fixed the same day: the first build followed the phone's appearance and
  showed the proposed light palette on the owner's phone, which is not the
  design. The apps are now dark whatever the phone's setting
- Components: the record control with its rings (calmer while the voice is
  made, still while it speaks, a fading halo with Reduce Motion), the
  language chips with swap, a searchable language sheet, the dot waveform,
  buttons and notices
- Recording your voice: a teleprompter that moves through the passage over
  the 20 seconds, the timer, then the recording's waveform to listen to,
  record again, or keep
- The interpreter: the two languages, what was said and its translation, and
  a headline and line under it for every stage of a run, including
  "Preparing your voice…", and "Speaking in the phone's voice" when the
  user's own could not be made. Tap to stop it speaking. Haptics when
  recording starts and stops
- Settings, with the developer controls under Advanced: where speech is
  turned into text, the speech model, the phone's speed, the last result
- Underneath, on both phones: the speech reports whose voice it is in;
  `pipeline.stopSpeaking`; the view models report a typed stage and the
  screens own the wording; `levels` on recorded audio for the waveform
- Tests: swapping languages, stopping speech, the voice reported, the
  waveform's levels, and the stage in place of status text. Android 145
  pass; iOS 167 pass, 1 skipped. iOS checked in the simulator, including
  with the phone set to light. Android is built and tested, not yet looked at
  on a screen
- Not yet: rings that follow the microphone's level; a designed light mode;
  the Plex families for Arabic, Devanagari, Thai and CJK
- Verify: `make ios-device` and walk through recording your voice and a
  translation; on Android, `make android-install`

The new screens (sign-in, rooms, and so on) are built to the design in their
own milestones.

Approved by the owner on 2026-10-31 ("The design is good").

---

## [ ] Milestone 20: Group rooms

Added on 2026-10-31. Rooms with more than two people. Each message is
translated and spoken once per language in the room, so a room of five who
speak three languages makes three versions of each. Safety (milestone 16)
covers group rooms as well as rooms of two.

---

## [~] Milestone 21: Paying for Shinar, after a free trial

Added on 2026-11-21. On the App Store nobody uses Shinar free: a new person
tries it free for 7 days, within a cap, then subscribes to keep sending.
iPhone only, through Apple's In-App Purchase, every purchase checked by our
own server. Android's turn is milestone 22. No calls: what is sold is what
the app does today.

People pay with their Apple account. Apple requires In-App Purchase for
anything used inside the app (Guideline 3.1.1), so Apple Pay, which is for
physical goods, does not apply. Under the Small Business Program, Apple keeps
15%.

**Decided by the owner on 2026-11-21:**

- **One subscription, $4.99 a month.** Apple sets the price in other
  countries from it, and each can be changed. No yearly plan for now.
- **Apple's own free trial, 7 days,** as the subscription's introductory
  offer. The person confirms with their Apple account on the paywall;
  nothing is charged during the 7 days, and the trial becomes the monthly
  plan unless they cancel. Apple gives each Apple account one trial, so a
  new Shinar account does not bring a second one. Apple's signed record says
  when a period is the free trial, so the server applies the trial's cap
  then. A trial run by our server, without payment details, would let anyone
  start again with a new account.
- **Only senders pay.** Sending a voice note or a message, and using the
  interpreter, needs the trial or the subscription. Reading and hearing what
  you are sent does not, and neither does asking for a text message's voice:
  the sender's subscription paid for it. Rosa can sign up and hear her
  grandson without paying, and subscribes when she wants to answer.

**The trial's cap** (confirmed by the owner, 2026-11-23; settings): 10
minutes of voice notes and 50 messages over the 7 days, at most about 25 cents of engines per
trial. Subscribers keep ADR 0029's daily limits, 10 minutes and 100
messages.

**What a minute costs** (2026-11-21, at the engines' paid rates): about 2
cents a minute of voice note between two languages. Fish Audio is about 1.2
cents ($15 per million bytes), the translation about 0.8 ($10 per million
characters), and Groq at most 0.2 ($0.111 an hour), only when the phone
cannot write the words down. A $4.99 subscription brings $4.24 after Apple's
15%, which pays for about 7 minutes a day. At today's limit of 10 minutes a
day one subscriber can cost about $6 a month. The average will be far
below that, but the limits should be set with it in mind.

**Checkpoint 21.1: Apple's set-up** (the owner, in App Store Connect; the
code does not wait for it)
- First, the seller (the owner's choice, 2026-11-22): Shinar is published by
  the owner's company, so other apps can sit beside it. The company is
  registered with CAC as a private company limited by shares (a business
  name is not enough: Apple takes only legal entities as organisations). It
  needs a bank account in its name, a website and work email on its own
  domain, and a D-U-N-S number; then Apple converts the individual
  membership to an organisation one, keeping the app. The agreement, tax,
  bank and trader status below are done as the company, after the
  conversion. Sandbox purchases on a real iPhone wait for the agreement; the
  StoreKit configuration file does not
- Business: the Paid Applications Agreement, with the bank account, the tax
  forms and contact details; then the App Store Small Business Program,
  before the first sale
- Business, Compliance: the EU trader status (Digital Services Act). Spain
  is in the EU. A trader's address, phone and email are shown on the EU
  store
- The subscription: one group, "Shinar", holding one subscription with the
  product ID `com.pelumi.shinar.monthly` (an ID can never be used again,
  even after it is deleted), 1 month, $4.99; its name and description; a
  free trial of 1 week as its introductory offer, in every country; and a
  screenshot of the paywall for review. Billing Grace Period on, so a failed
  card does not cut someone off at once. The first subscription is reviewed
  with an app version
- An In-App Purchase key (Users and Access, Integrations): its Key ID, the
  Issuer ID, and the `.p8` file into `~/keys/shinar/`, named in
  `/etc/shinar/env`
- The server notification URLs, version 2, for production and for the
  sandbox:
  `https://shinar-api.southafricanorth.cloudapp.azure.com/v1/billing/apple/notifications`
- Sandbox Apple accounts, for testing purchases
- Claude: a StoreKit configuration file in the Xcode project with the same
  products, so purchases work in the simulator and the tests before any of
  the above exists

**Checkpoint 21.2: the server** (built 2026-11-23)
- Migration 0012: `subscriptions` (one per Apple original transaction: the
  account, the product, sandbox or production, the newest transaction and the
  period it paid for, whether that is the free trial, `revoked_at`, and from
  the renewal information `auto_renew` and the grace period) and
  `store_notifications` (each of Apple's notifications once, as it came)
- `internal/billing` checks Apple's signed records with the standard
  library: the chain to Apple Root CA - G3 (in the program, its SHA-256
  pinned in a test), Apple's marks on the leaf and the intermediate, the
  chain as of the signing date, ES256 only, the bundle ID
- `POST /v1/me/subscription/apple`: the phone's signed transaction, and its
  renewal information when it has it. A purchase naming another account is
  refused (`403 not_your_purchase`); a subscription of another live account
  too (`409 subscription_in_use`); one whose account was deleted moves to
  the account that restores it
- `POST /v1/billing/apple/notifications`: Apple's notifications, version 2,
  with no sign-in. Every kind is handled the same way, by the transaction and
  renewal information inside; records from either side are applied only when
  newer than what is kept
- `GET /v1/me/subscription`, in place of a field in `GET /v1/me`: `required`,
  `state` (none, trial, active, grace, expired), `until`, `renews`, and in the
  trial what is left of its cap
- With `BILLING=on`: sending, and the interpreter's work on the server, need
  the trial or a subscription (`402 subscription_required`, checked before a
  recording is stored). In the trial its cap counts from its start, in place
  of the daily limits (`429 trial_limit_reached`); `TRIAL_VOICE_MINUTES` and
  `TRIAL_TEXT_MESSAGES`, 10 and 50, confirmed by the owner on 2026-11-23. Reading and
  hearing need nothing. Purchases are recorded with the switch off too
- Changed from the plan: the interpreter's work is not counted against a cap,
  as no limit was ever set for it; the monthly budget (18.12) bounds it. Nor
  does the server ask Apple for each subscription's status when a
  notification might be missing: the phone reports what StoreKit gives it at
  every launch and change, and Apple sends notifications again. The App Store
  Server API client exists, signed with the In-App Purchase key, for
  `make appstore-check` and later uses
- ADR 0034
- Tests: the verifier against chains made for tests (each way a record can be
  refused), and once against Apple: `make appstore-check` asked Apple's
  sandbox for a test notification and the verifier accepted Apple's own
  signature, which also shows the In-App Purchase key works before the paid
  agreement. The store (trial to paid to expired, late and older records,
  the grace period, a refund, one subscription one account, Apple's word
  first), the service (whose purchase, notifications applied and kept once),
  the trial's cap in the rooms store, and every answer of the API. Also: the
  migration on this Mac's database; lint, and the race detector
- Owner, when this server ships: nothing to set. `BILLING` stays off; Apple's
  notifications start arriving at the address set in App Store Connect
- Verify: `make server-test`; `make appstore-check`

**Checkpoint 21.3: the iPhone** (built 2026-11-23)
- `Core/Billing/Subscriptions`, on StoreKit 2: where the user stands, from
  the server; every purchase carries the user's ID as `appAccountToken`;
  `Transaction.updates` heard for as long as the app runs (renewals, refunds,
  Ask to Buy, offer codes); on signing in, and after Restore, everything the
  Apple account holds is sent to the server. A transaction is finished only
  once the server has it, or has refused it for good, so one that could not
  be sent goes again on the next launch. `Core/Net/BillingClient` for the two
  endpoints
- The paywall is Apple's own `SubscriptionStoreView` for the one plan, rather
  than one drawn by hand: it shows the price, the free week and how it renews
  in the person's currency, with Restore, the terms and the privacy page, as
  App Review checks (Guideline 3.1.2). Shinar's words above it: what it does,
  the free week's cap, and that reading and hearing are always free. Its close
  button is the "Not now". It closes by itself once a purchase comes to
  something, good or bad (the owner's ask, 2026-11-23), and an alert over the
  screen underneath says what: the free week started, subscribed, waiting for
  approval, or what went wrong. Backing out of Apple's sheet leaves it open
- Where it shows: once, the first time someone reaches the app with no
  subscription; in a room, in place of the composer ("Start your free week to
  send", before anything is recorded); when the server refuses a send for
  want of one; on the Interpreter tab, which is locked without one; and from
  Profile
- Profile, under Plan: the plan in a line ("Free week · until 30 Nov",
  "Subscribed · renews 30 Dec", "Payment failed · Apple is retrying"),
  Manage subscription (Apple's sheet), or Start your free week, and Restore
  purchases. Deleting the account says first that Apple keeps charging until
  the subscription is cancelled, and where
- With the server's `BILLING` off, nothing of this shows: the server says a
  subscription is not required, so TestFlight testers see no paywall and no
  plan
- The website's terms (a Subscription section: renewal, cancelling at least
  24 hours before, refunds through Apple, and that deleting the account does
  not cancel it; no price, which the app shows) and both privacy pages (what
  Apple tells us of a purchase, never card details)
- `PTranslateTests/Shinar.storekit`: the plan as App Store Connect sells it,
  for the tests, and for the simulator when chosen in the scheme (Run,
  Options, StoreKit Configuration). A purchase there is signed by Xcode, not
  Apple, so the server refuses it: the server's check is seen working, the
  purchase is not
- Tests: the client over the wire, unknown states, who may send, the plan's
  wording, the refusals, and the configuration file read by a StoreKit test
  session (a month, its first week free). iOS 219 pass, none skipped
- Not checked yet: a real purchase in Apple's sandbox, which needs the Paid
  Applications Agreement, which waits on the company (21.1); the paywall on a
  screen
- Verify: `make ios-test`. To see it: `BILLING=on make server-run`, the
  simulator with the StoreKit configuration chosen, sign up as a test account:
  the paywall is offered, the room's composer asks for the free week, the
  Interpreter is locked

---

## [ ] Milestone 22: Android catches up, and Google Play

Added on 2026-11-21, when the owner chose to take the iPhone to the App Store
first. Android gets what was built for the iPhone alone from milestone 16
on: block and report (the screens, `you_blocked`, and leaving `***` out of
the phone's voice; the server's side is shared), `account_suspended` at
sign-in, the day headings in a room, asking before words leave the phone, and
the paywall
and subscriptions through Google Play Billing (a dependency to approve), with
the server checking purchases with Google (the Play Developer API, and
notifications through Cloud Pub/Sub). Then Google Play itself: a developer
account ($25), the closed test that new personal accounts must run (at least
12 testers for 14 days) before they can publish, and the listing.

---

## Open questions

Answered on 2026-10-04: dependencies approved; Postgres through Homebrew
instead of Docker; the owner creates the Android project at checkpoint 1.4.

Decided on 2026-10-04: the source language is picked by hand. ML Kit needs an
explicit source language, and detecting it would mean running Whisper's
language detection first; that can be added later behind the same picker.

Decided on 2026-10-09: own-voice output uses a sample recorded once at
onboarding and kept on the phone (replacing the 2026-10-07 idea of using each
recording as its own sample). No voice profiles on the server and no accounts,
so auth stays a non-goal. The trial engine is a public Hugging Face demo.

Decided on 2026-10-17: the iOS app translates with Apple's Translation
framework, for now. The owner would prefer ML Kit on both phones, but ML Kit
for iOS has no build for Apple-silicon simulators (checked in its 8.0.0
binary), and Xcode 26 no longer runs simulators under Rosetta: with it, the
whole app would stop running in the simulator. The keyboard of milestone 11
types the original words; typing the translation may come later.
Revisited on 2026-11-10: ML Kit on both phones after all, from Google's own
files with a simulator copy made by our script (ADR 0024, checkpoint 18.3).

Decided on 2026-10-14: an iOS app, native Swift, built next (milestone 10);
then the rest of milestone 9 on both phones; then voice keyboards on both
(milestone 11). The keyboard types the original words, not a translation.
iOS and the keyboard were non-goals until this date.

Decided on 2026-10-31: the app is a product, a social app where people talk
across languages in their own voices (ADR 0015). Voices are kept on the
server (ADR 0016). Sign-in by email, Sign in with Apple, or Sign in with
Google. The order of work above, with milestone 12 dropped. The owner now
has a paid Apple Developer account, which answers the App Group question of
milestone 11 and makes push notifications possible on iOS.

Still open:

1. Settled 2026-11-11: Groq, with Whisper large-v3 (ADR 0009).
2. When to add API credit on Fish Audio, to move from the free model to the
   paid one (checkpoint 9.4).
3. The profile's details beyond name, ID and language (milestone 13).
4. Codes sent by email need an email-sending service and a domain to send
   from (milestone 13).
5. Settled 2026-10-31: Sign in with Google on Android, through Credential
   Manager (`androidx.credentials`, `googleid`). On the iPhone it waits.
6. Settled 2026-10-31: Google Cloud Translation translates messages on the
   server (ADR 0020). The owner also chose: two QR libraries on Android
   (ZXing core, Google's code scanner), and no city on profiles for now.
7. Hosting, the domain, and storage for audio (milestone 18). Storage
   settled 2026-11-21: Cloudflare R2 (checkpoint 18.13).
8. Settled 2026-10-31: milestone 12 comes back, after milestone 14.
   Feasible: Android decodes WhatsApp's audio itself;
   on the iPhone, one test on a real phone settles whether iOS decodes it and
   whether WhatsApp's share menu offers the app. The reply reaches WhatsApp as
   an audio file, not a voice-note bubble.
9. Settled 2026-11-21: one subscription at $4.99 a month, Apple's 7-day free
   trial, and only senders pay (milestone 21). The trial's cap, 10 minutes of
   voice notes and 50 messages, confirmed 2026-11-23.
10. Settled 2026-11-22: the owner's company runs Shinar, a private company
    limited by shares registered in Nigeria, under Nigerian law (checkpoints
    18.11 and 21.1). Its name is to come.
11. Settled 2026-11-21: Guideline 1.2's filtering is the translator's
    profanity option (checkpoint 16.1).
