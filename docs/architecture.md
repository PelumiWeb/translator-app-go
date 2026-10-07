# Architecture

Status: approved 2026-10-04.

## 1. What the system does

A person records speech on an Android phone. The app transcribes it, translates
the text into a target language picked for the receiver, and shows the result.

Transcription happens on the device when it can. When it cannot (no model yet,
slow device, low confidence), the audio goes to a Go backend that queues the
work, runs it through a cloud provider, and streams progress back.

```
                      Android app
  mic -> AudioRecorder -> Transcriber --------> Translator -> screen
                            |      \             (ML Kit)
                on device   |       \  fallback
              (whisper.cpp) |        \
                                      v
                            POST /v1/jobs (WAV)
                            GET  /v1/jobs/{id}/events (SSE)
                                      |
                      Go server       v
  HTTP API -> audio storage + jobs table -> worker pool -> Provider
      ^                                         |
      +-------------- job_events <--------------+
```

The aim of the product goes one stage further than this diagram: the translated
text is spoken aloud, in the voice of whoever spoke, for every user. That stage is not designed
yet; it is planned as milestones 8 and 9 and will sit after `Translator` behind
a `SpeechSynthesizer` interface.

Two things to notice:

- Translation always runs on the device, on both paths. The backend only
  produces a transcript. This keeps the server small and means one translation
  code path to test.
- The backend is also the model distribution point (manifest and model files).

## 2. Android

Kotlin, Jetpack Compose, one Gradle module, in `android/`. minSdk 26. Native
code is built for `arm64-v8a`, which covers phones and the emulator on Apple
silicon.

### 2.1 Core and UI split

Everything that is not a screen lives under `core/` and knows nothing about
Compose or Activities. Two things are the surface a front end uses:

```kotlin
interface AudioRecorder {
    fun start(): Recording          // Recording.stop(): PcmAudio
}

class SpeechTranslationPipeline(
    private val transcriber: Transcriber,
    private val translator: Translator,   // from milestone 3
) {
    fun process(audio: PcmAudio, source: Language, target: Language): Flow<PipelineEvent>
}
```

Recording is kept out of the pipeline because the front end owns the record
button: it decides when a recording starts and stops, then hands the audio
over. A future IME would use these same two classes from its
`InputMethodService`. That is the whole extent of IME support for now. When
the IME is actually built, `core/` moves into its own Gradle module; until then
the boundary is kept by package discipline (see CLAUDE.md).

### 2.2 Audio capture

`AudioRecord` at 16 kHz, mono, 16-bit PCM, which is what Whisper expects, so no
resampling. Capture runs on its own thread, because `AudioRecord.read` blocks.
Recordings are capped at 30 seconds, one Whisper window. For upload
the PCM is wrapped in a WAV header (44 bytes, no encoder dependency). A 30
second clip is about 960 KB.

### 2.3 Transcriber

```kotlin
interface Transcriber {
    fun transcribe(audio: PcmAudio, language: Language): Flow<TranscriptEvent>
}

sealed interface TranscriptEvent {
    data class StageChanged(val stage: TranscriptionStage) : TranscriptEvent  // uploading, queued, processing
    data class Partial(val text: String) : TranscriptEvent
    data class Final(val transcript: Transcript) : TranscriptEvent
}

data class Transcript(
    val text: String,
    val confidence: Float?,   // null when the source cannot provide one
    val source: Source,       // ON_DEVICE or CLOUD
)
```

It returns a `Flow` because the cloud path produces partial results over SSE.
The on-device path emits a single `Final`.

Implementations:

| Class | Milestone | Notes |
| --- | --- | --- |
| `RemoteTranscriber` | 1 | uploads WAV, maps SSE events to `TranscriptEvent` |
| `WhisperTranscriber` | 2 | whisper.cpp through JNI |
| `RoutingTranscriber` | 2, 6 | picks between the two: by a switch on screen from milestone 2, by the fallback policy from milestone 6 |

### 2.4 whisper.cpp via JNI

whisper.cpp is a git submodule in `third_party/`, pinned to a release tag and
built by CMake through the Android Gradle plugin into one shared library. One
Kotlin class, `WhisperContext`, owns the native pointer and is the only place
with `external` functions: load, free, transcribe, confidence of the last
transcript, and system info. A context is not safe for concurrent use, so
`WhisperTranscriber` runs it on a single-thread dispatcher.

Model: multilingual `ggml-base`, quantised (q5_1, 57 MB), with `tiny` (about
30 MB) as the option for slow devices. The English-only `.en` models are not
used because the source language is not always English.

Details and the reasons are in ADR 0004.

### 2.5 Translator

```kotlin
interface Translator {
    val supportedLanguages: List<Language>
    suspend fun isReady(source: Language, target: Language): Boolean
    suspend fun prepare(source: Language, target: Language)      // downloads language packs
    suspend fun translate(text: String, source: Language, target: Language): String
}
```

`MlKitTranslator` wraps ML Kit on-device translation. ML Kit downloads and
stores its own language packs (about 30 MB each), so they do not go through our
model manager. `isReady` is separate from `prepare` so the pipeline can tell
the screen that a download is about to happen instead of appearing to hang.

The pipeline emits the transcript before it starts translating. If translation
fails, for example offline with no language pack, the transcript is still
shown.

ML Kit's native library adds about 16 MB to the APK.

### 2.6 Fallback policy

`RoutingTranscriber` decides per recording. The three triggers from the spec,
made concrete:

| Trigger | Definition | Checked |
| --- | --- | --- |
| Model missing | `ModelManager` has no verified model on disk | before transcribing |
| Device too slow | real-time factor (processing time / audio length) above 1.0 on a short benchmark clip run once after the model is installed, result stored | before transcribing |
| Low confidence | mean token probability of the result below 0.6, or Whisper's no-speech probability above 0.6 | after transcribing |

The thresholds are starting points and live in one config object.

If the network is unavailable the fallback cannot run. The app then shows the
on-device result marked as low confidence, or an error if there is no result.

### 2.7 Model manager

1. `GET /v1/models/manifest`, and find the model.
2. Download to `<files>/models/<id>.part`. If a partial file exists and belongs
   to the same model, send `Range: bytes=<size>-` with `If-Range: "<sha256>"`.
   If the server answers 200 instead of 206, start over.
3. Check the SHA-256 of the complete file against the manifest. On mismatch,
   delete it and report an error.
4. Rename `.part` to `.bin`. A `.bin` file is therefore always a verified
   model.

An installed model is returned without contacting the server.

State is exposed as `StateFlow<ModelState>`: `Missing`,
`Downloading(bytes, total)`, `Verifying`, `Ready(file)`, `Failed(reason)`.

The download starts when the user asks for it on screen, not on first launch.
`WhisperTranscriber` asks for the installed model on every transcription, so a
model that finishes downloading is used without restarting anything.

The reasons are in ADR 0005.

### 2.8 Networking

OkHttp for upload, download, and SSE (`okhttp-sse`). `BackendClient` is the
only class that knows the backend's URLs and JSON. The stream uses a client
with no read timeout, since an event stream is silent between events.

The backend URL is a build config field, `http://localhost:8080` for now;
`adb reverse` forwards it to the dev machine. Debug builds allow cleartext HTTP
to `localhost` and `10.0.2.2` through a debug-only network security config;
release builds do not.

## 3. Server

Go, one binary that runs the HTTP API and the worker pool in the same process.
Postgres is the only external service.

### 3.1 Endpoints

**`POST /v1/jobs`**: `multipart/form-data` with an `audio` part (WAV) and a
`source_lang` field. The body is limited to 10 MB with `http.MaxBytesReader`.
The handler stores the audio, inserts the job and its `queued` event in one
transaction, and returns `202 Accepted`:

```json
{ "id": "5b1c...", "status": "queued", "events_url": "/v1/jobs/5b1c.../events" }
```

**`GET /v1/jobs/{id}/events`**: Server-Sent Events.

```
id: 3
event: partial
data: {"text":"hello wor"}

id: 4
event: done
data: {"text":"hello world","language":"en"}
```

Event types: `queued`, `processing`, `partial`, `done`, `error`. The stream
closes after `done` or `error`. A comment line is sent every 15 seconds to keep
proxies from closing an idle connection.

**`GET /v1/models/manifest`**

```json
{ "models": [ { "id": "whisper-base-q5_1", "version": "1",
    "size": 59700000, "sha256": "ab12...", "url": "/v1/models/whisper-base-q5_1" } ] }
```

The catalogue is built once at startup from the `.bin` files in `MODELS_DIR`.
A model's `id` is its file name without the extension, and its `version` is the
first 12 characters of its SHA-256, so replacing a file changes the version
with nothing to bump by hand.

**`GET /v1/models/{id}`**: the model file. Served with `http.ServeContent`,
which implements `Range`, `If-Range` and `206 Partial Content` in the standard
library, so resume support needs no code of ours. The SHA-256 is sent as the
`ETag`. A client resuming a download sends it back in `If-Range`; if the file
has been replaced, the server sends the whole new file instead of a tail that
would not match the half the client already has.

### 3.2 Data model

```sql
CREATE TABLE jobs (
    id           uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    status       text NOT NULL,          -- queued | processing | done | failed
    source_lang  text NOT NULL,
    audio_path   text NOT NULL,
    attempts     int  NOT NULL DEFAULT 0,
    max_attempts int  NOT NULL DEFAULT 3,
    run_at       timestamptz NOT NULL DEFAULT now(),  -- not before this time
    locked_until timestamptz,                         -- lease expiry
    result_text  text,
    last_error   text,
    created_at   timestamptz NOT NULL DEFAULT now(),
    updated_at   timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX jobs_ready ON jobs (run_at) WHERE status = 'queued';

CREATE TABLE job_events (
    job_id     uuid NOT NULL REFERENCES jobs(id) ON DELETE CASCADE,
    seq        int  NOT NULL,            -- 1, 2, 3 ... per job; the SSE id
    type       text NOT NULL,
    payload    jsonb NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (job_id, seq)
);
```

### 3.3 Job queue

A worker claims a job with one short statement:

```sql
UPDATE jobs
SET status = 'processing', attempts = attempts + 1,
    locked_until = now() + interval '2 minutes', updated_at = now()
WHERE id = (
    SELECT id FROM jobs
    WHERE status = 'queued' AND run_at <= now()
    ORDER BY run_at
    FOR UPDATE SKIP LOCKED
    LIMIT 1
)
RETURNING id, source_lang, audio_path, attempts, max_attempts;
```

`SKIP LOCKED` lets several workers run this at once without waiting on each
other or claiming the same row.

The claim commits immediately; the row lock is not held while the provider
runs. Ownership during processing is the `locked_until` lease instead. The
alternative, keeping the transaction open for the whole transcription, pins one
database connection per in-flight job and loses the `processing` status for
everyone else, because uncommitted changes are invisible to other connections.

- **Worker pool**: N goroutines, each looping claim -> process -> finish. When
  the queue is empty a worker waits for a poll tick (1 second) or a wake-up
  signal sent on enqueue.
- **Retries**: on a retryable error with attempts left, the job goes back to
  `queued` with `run_at = now() + backoff`. Backoff is exponential with jitter
  (2 s, 4 s, 8 s, plus or minus 20%). Out of attempts, or a non-retryable error:
  `failed`, and an `error` event.
- **Crashed workers**: a job whose `locked_until` has passed is put back to
  `queued` by a periodic sweep.
- **Graceful shutdown**: on SIGINT or SIGTERM, stop accepting HTTP requests,
  stop claiming jobs, let in-flight jobs finish for up to 30 seconds, then
  cancel their contexts. A cancelled job is released back to `queued` without
  counting the attempt.

### 3.4 Events and SSE

The worker and the SSE handler are different goroutines, and later may be
different processes, so events go through the database rather than straight
from one to the other.

1. The worker appends each event to `job_events`.
2. It then publishes the event on an `EventBus`.
3. The SSE handler subscribes to the bus first, then reads the job's existing
   events from the table (after `Last-Event-ID` if the client sent one), writes
   them, and then writes live events, dropping any whose `seq` it already sent.

Subscribing before reading closes the gap in which an event could be missed. It
also covers the common case of a client connecting after the job has already
finished, and reconnects after a dropped connection.

`EventBus` starts as an in-process implementation (a map of job id to
subscriber channels). That is correct for one server process. Milestone 5
replaces it with Postgres `LISTEN/NOTIFY` so it works across processes; the
interface does not change.

### 3.5 Provider

```go
type Provider interface {
    // Transcribe sends partial text to onPartial as it becomes available and
    // returns the final result. onPartial may never be called.
    Transcribe(ctx context.Context, audio io.Reader, opts Options, onPartial func(text string)) (Result, error)
}
```

A callback is used for partials instead of a returned channel: the caller does
not have to drain or close anything, and a batch-only provider just never calls
it.

`fake.Provider` returns a canned sentence word by word with a configurable
delay, and can be told to fail the first N attempts. It serves local dev and
tests, and is what makes retries demoable.

A real provider is chosen in milestone 7 and gets its own ADR.

### 3.6 Audio storage

A small `BlobStore` interface (`Put`, `Open`, `Delete`) with a local directory
implementation. Audio is deleted when its job reaches `done` or `failed`.

### 3.7 Configuration and logging

Environment variables read once in `main` (`DATABASE_URL`, `HTTP_ADDR`,
`WORKERS`, `AUDIO_DIR`, `MODELS_DIR`). Structured logs with `log/slog`, with
the job id on every line that concerns a job.

## 4. Testing

| Area | How |
| --- | --- |
| Queue (required) | Go tests against the local Postgres, in a separate test database where every test gets its own schema. Covers: concurrent workers never claim the same job, retry with backoff, giving up after max attempts, expired lease recovery, shutdown releasing in-flight jobs. `SKIP LOCKED` behaviour cannot be faked meaningfully. |
| SSE handler (required) | `httptest` with an in-memory event source, no database. Covers: framing, replay from `Last-Event-ID`, no duplicates across the replay/live boundary, stream closes on terminal event, handler returns when the client disconnects. |
| Model checksum (required) | Kotlin JVM tests with OkHttp `MockWebServer`: good hash, bad hash, resumed download, server ignoring `Range`. Go test that manifest hashes match the files on disk. |
| Fallback policy | Kotlin JVM tests with fake `Transcriber`s. |
| Everything else | Tested where logic exists; no tests for wiring. |

## 5. Dependencies

Anything outside this list needs approval first. Items marked *added* were not
in the original spec; the owner approved them on 2026-10-04.

**Server**

| Dependency | Why |
| --- | --- |
| Go standard library | HTTP, routing, logging, tests |
| Postgres 16, Homebrew `postgresql@16` (*added*) | database and queue. Installed locally instead of Docker to save disk and RAM on the dev machine. |
| `github.com/jackc/pgx/v5` (*added*) | Postgres driver and connection pool. A driver is unavoidable; pgx is the standard one and supports `LISTEN/NOTIFY`. |

No router (Go 1.22+ `ServeMux` covers method and path patterns, so chi is not
needed), no migration tool (SQL files embedded with `embed`, applied by a small
runner at startup), no UUID library (Postgres generates ids), no test library.

**Android**

| Dependency | Why |
| --- | --- |
| AndroidX core, activity-compose, lifecycle-viewmodel-compose, Compose BOM, Material 3 | implied by "Jetpack Compose" |
| kotlinx-coroutines | implied by Kotlin on Android |
| whisper.cpp (git submodule), NDK, CMake | named in the spec |
| ML Kit Translate | named in the spec |
| OkHttp and `okhttp-sse` (*added*) | upload, ranged download, SSE client |
| kotlinx-serialization-json and its Kotlin compiler plugin (*added*) | JSON for the manifest and SSE payloads |
| lifecycle-runtime-compose | `collectAsStateWithLifecycle`; part of AndroidX lifecycle |
| JUnit 4, kotlinx-coroutines-test, OkHttp MockWebServer (*added*, test only) | unit tests |

No DI framework, no Retrofit, no Room.

## 6. Planned ADRs

Written in the milestone where the decision is implemented.

| # | Decision | Milestone |
| --- | --- | --- |
| 0001 | Postgres as the job queue, lease-based claims | 1 |
| 0002 | Events persisted in `job_events`, replayed over SSE | 1 |
| 0003 | Translation stays on the device on both paths | 1 |
| 0004 | whisper.cpp through JNI, model choice and quantisation | 2 |
| 0005 | Model distribution: manifest, resume, checksum | 4 |
| 0006 | `LISTEN/NOTIFY` as the cross-process event bus | 5 |
| 0007 | Fallback policy and thresholds | 6 |
| 0008 | Cloud transcription provider | 7 |

## 7. Known limits

- No auth: anyone who can reach the server can upload audio and download
  models. Job ids are random UUIDs, so one client cannot guess another's event
  stream, and uploads are size-limited. Not suitable for a public deployment.
- ML Kit translation covers about 59 languages and translates through English
  when neither side is English, so quality between two non-English languages is
  modest.
- Recordings are limited to 30 seconds.
- The low-confidence fallback makes the user wait twice: once for the device,
  then for the cloud.
