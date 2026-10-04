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

## [~] Milestone 1: End to end, ugly

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

**Checkpoint 1.4: Android app**
- The owner creates the Android project in `/android` (application id chosen
  then); Claude builds inside it
- One Compose screen: Record/Stop button, status line, text
- `AudioRecorder` (16 kHz mono PCM), WAV encoding, microphone permission
- `Transcriber` interface and `RemoteTranscriber` (upload and SSE)
- Minimal `SpeechTranslationPipeline` without translation yet
- Verify: the demo above, on the emulator

Deliberately left out of M1: retries with backoff, lease expiry sweep,
`Last-Event-ID` on the client, any error UI beyond a line of text.

---

## [ ] Milestone 2: On-device transcription

**Demo**: airplane mode on, record, the transcript appears.

- whisper.cpp submodule, CMake build, JNI bridge, `WhisperContext`
- `WhisperTranscriber` behind `Transcriber`, with confidence from token
  probabilities
- Model file pushed by hand with `adb push` for now (model manager is M4)
- A debug toggle on screen: on-device or cloud
- Show time taken and real-time factor, so slow devices are visible
- ADR 0004

Checkpoints: (2.1) native build and JNI smoke test, (2.2) `WhisperTranscriber`
wired into the pipeline and screen.

---

## [ ] Milestone 3: Translation

The first point at which the app does what it is for.

**Demo**: pick source and target language, speak, read the translation.

- `Translator` interface, `MlKitTranslator`
- Source and target language pickers
- Language pack download state on screen
- Pipeline emits transcript, then translation
- Tests for the pipeline with fake `Transcriber` and `Translator`

Checkpoints: (3.1) translator and pipeline with tests, (3.2) UI.

---

## [ ] Milestone 4: Model distribution

**Demo**: fresh install with no model. The app downloads it with a progress
bar. Turn the network off halfway and back on: the download resumes. Corrupt
the file on the server: the app rejects it.

- Server: `GET /v1/models/manifest`, `GET /v1/models/{id}` with range support,
  manifest built from the files in `MODELS_DIR`, test that hashes match
- Android: `ModelManager` (manifest, resumable download, SHA-256, atomic
  rename), state on screen
- Checksum and resume tests with `MockWebServer`
- ADR 0005

Checkpoints: (4.1) server endpoints and tests, (4.2) Android model manager and
tests, (4.3) UI and the removal of the `adb push` step.

---

## [ ] Milestone 5: Queue hardening

**Demo**: the fake provider fails twice then succeeds; the job retries with
growing delays and completes. Kill a worker mid-job; the job is picked up
again after its lease expires. Drop the SSE connection; the client reconnects
and continues from where it left off.

- Retries with exponential backoff and jitter, `max_attempts`, retryable and
  non-retryable errors
- Lease expiry sweep
- `LISTEN/NOTIFY` `EventBus`, so API and workers can run as separate processes
- `Last-Event-ID` reconnect in the Android client
- `Idempotency-Key` on `POST /v1/jobs`, so a retried upload does not create a
  second job
- Tests for each of the above
- ADR 0006

Checkpoints: (5.1) retries and backoff, (5.2) lease sweep, (5.3)
`LISTEN/NOTIFY` bus, (5.4) client reconnect and idempotency.

---

## [ ] Milestone 6: Fallback policy

**Demo**: three cases side by side. No model: goes to the cloud. Benchmark
marked slow: goes to the cloud. Mumbled audio: runs on device, confidence is
low, goes to the cloud, and the screen says which path produced the result.

- `RoutingTranscriber`, benchmark after model install, thresholds in one place
- UI shows the source of each result and why
- Tests for the policy with fake transcribers
- ADR 0007

Checkpoints: (6.1) policy and tests, (6.2) benchmark and UI.

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

## Open questions

Answered on 2026-10-04: dependencies approved; Postgres through Homebrew
instead of Docker; the owner creates the Android project at checkpoint 1.4.

Still open, not blocking yet:

1. Which real cloud provider (milestone 7).
2. Whether the source language is always picked by hand or can be detected
   (milestone 3).
