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
talk across languages (ADR 0015), and the order changed. The order of work,
as of 2026-10-31:

1. Checkpoint 9.4: a real voice engine
2. Milestone 13: accounts, profile, and your voice on the server
3. Milestone 14: voice notes between people
4. Milestone 15: text messages
5. Milestone 16: safety and discovery
6. Milestone 18: going live
7. Milestone 17: calls, for premium users
8. Milestone 11: voice keyboards
9. Milestone 7 and checkpoint 8.2, in an order still to agree

Milestone 10 was done on 2026-10-29. Milestone 12 was dropped on 2026-10-31:
voice notes now live inside the app (milestone 14).

A checkpoint with phone work is done on Android, then on iOS.

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
- UI pass: the app stops being ugly
- README with a demo recording, an architecture diagram, and how to run it
- CI: Go tests with a Postgres service, Android unit tests and build

Checkpoints: (7.1) provider, (7.2) UI, (7.3) README and CI.

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

## [-] Milestone 12: Voice notes, interpret and reply

Dropped on 2026-10-31. The social app brings voice notes into the app itself
(milestone 14), where they arrive already in the listener's language and the
sender's voice, so there is nothing to bring in from WhatsApp. Kept below as
the record of what was considered.

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

## [ ] Milestone 13: Accounts, profile, and your voice on the server

Added on 2026-10-31, when the app became a product (ADR 0015). The first
step of the social app: each person has an account, a profile others can
find, and their voice kept on the server, so that others can hear their
words in it (ADR 0016). Architecture section 9 has the direction.

**Demo**: on both phones, sign up with Apple, Google or a code sent by email;
enter your name, choose your ID and your language; record your voice and
agree to it being kept. Sign in on a second phone: the same profile and
voice are there. Delete your voice, then your account: both are gone from
the server.

Checkpoints, to be detailed when the milestone starts:

- 13.1 Server: users, sign-in identities and sessions in Postgres; checking
  Apple and Google sign-in tokens; codes sent by email; the profile; handles
  that are unique; deleting an account
- 13.2 Server: each user's voice sample, kept with a record of their
  consent, replaceable and deletable; requests to speak use it instead of an
  upload
- 13.3 Android, 13.4 iOS: sign in, the profile, the voice with consent, sign
  out, delete the account
- ADR for sign-in and sessions

---

## [ ] Milestone 14: Voice notes between people

**Demo**: two phones, one set to English and one to Spanish. Add each other
by ID or QR code. Send a voice note in English: the other phone is notified,
and plays it in Spanish, in the sender's voice, with the original words and
the translation on screen. Reply in Spanish the other way.

Checkpoints, to be detailed when the milestone starts:

- 14.1 Server: contacts, conversations and messages. A voice note is the
  sender's audio and transcript; the server translates it into the
  listener's language and speaks it in the sender's voice, as jobs on the
  queue (ADR 0017)
- 14.2 Server: push notifications, through APNs and FCM
- 14.3 Android, 14.4 iOS: contacts, a conversation, record and send, receive
  and play

---

## [ ] Milestone 15: Text messages

**Demo**: type in English; the other side reads it in Spanish, and can tap
it to hear it in the sender's voice.

---

## [ ] Milestone 16: Safety and discovery

Block and report, and a way to act on reports; then finding people beyond
those whose ID you know, including at random. Block and report come first:
both app stores require them in apps where people contact each other, and
contact at random invites abuse without them.

---

## [ ] Milestone 17: Calls, for premium users

Live calls, each side hearing the other in their own language and the
speaker's voice. Starts with a research checkpoint: recognising, translating
and speaking as a stream, within about two seconds, and what a minute costs.
Premium only, so it brings billing: in-app purchases on both stores.

---

## [ ] Milestone 18: Going live

A public server (hosting, a domain, HTTPS, storage for audio), a privacy
policy and terms that cover voices, and listings on the App Store and Google
Play. Before the first outside users, and after milestone 16, since the
stores require block and report.

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

1. Which real cloud transcription provider (milestone 7).
2. When to add API credit on Fish Audio, to move from the free model to the
   paid one (checkpoint 9.4).
3. The profile's details beyond name, ID and language (milestone 13).
4. Codes sent by email need an email-sending service and a domain to send
   from (milestone 13).
5. Sign in with Google: the libraries each phone uses (milestone 13).
6. Which translation service the server uses for messages (milestone 14).
7. Hosting, the domain, and storage for audio (milestone 18).
