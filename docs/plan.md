# Plan

Milestones are thin vertical slices. Each ends with something that can be
shown working. One milestone at a time; the next one starts only after the
owner approves the current one.

Inside a milestone, work stops at each **checkpoint**: Claude summarises what
changed, how to verify it, and suggests a commit message. The owner reviews and
commits. Then work continues.

Status: `[ ]` not started, `[~]` in progress, `[x]` approved.

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

**Checkpoint 9.3: the app speaks through the server**
- A second `SpeechSynthesizer` that uploads the sample and the translation,
  and plays what comes back
- Fallback to the phone's stock voice when there is no sample, no network, or
  the server fails

**Checkpoint 9.4: a real engine, as a free trial**
- A provider that calls a public Hugging Face Spaces demo of an open
  voice-cloning model. Free and nothing to install; slow, shared, and liable
  to change, and the test recordings go to a public demo. For trying the flow,
  not for real users
- The onboarding screen's description of what happens to the recording must
  change with this: it currently says the server uses it once and deletes it,
  which stops being the whole truth when a third party's demo receives it
- ADR for the voice-cloning engine

Later, not planned in detail:
- A paid hosted engine (about $5 a month tier) when reliability matters
- The owner's long-run idea: a free engine for free users and a paid one for
  paying users. That needs accounts and billing, both non-goals today; the
  engine is chosen per request so it stays possible
- A guest mode: with one enrolled voice per phone, a guest's words come out in
  the owner's voice

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

Still open, not blocking yet:

1. Which real cloud provider (milestone 7).
2. Which paid voice-cloning engine to use once the free trial is not enough
   (milestone 9). A new dependency to approve.
