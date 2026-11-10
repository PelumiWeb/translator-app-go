# Architecture

Status: approved 2026-10-04. Section 3 (iOS) added and approved 2026-10-14.
Section 9 (the social product) added 2026-10-31: the direction from milestone
13 on, which changes some of what sections 1 to 4 say, as noted where it does.

The app is called Shinar (2026-11-01; built as PTranslate, working name
Timbre). Both phones have the identifier `com.pelumi.shinar`; the code keeps
its old names (ADR 0021).

## 1. What the system does

A person records speech on a phone, Android or iPhone. The app transcribes it,
translates the text into a target language picked for the receiver, and shows
the result.

There are two apps, both native, built to one design: the same stages, the
same seams, the same rules, the same server. Section 2 describes it through
the Android app, which came first; section 3 records where the iOS app
differs. They share no code; the server's API and this document are what keep
them alike.

Transcription happens on the device when it can. When it cannot (no model yet,
slow device, low confidence), the audio goes to a Go backend that queues the
work, runs it through a cloud provider, and streams progress back.

```
                  Android or iOS app
  mic -> AudioRecorder -> Transcriber --------> Translator -> screen
                            |      \            (on device)
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

The pipeline goes one stage further than this diagram: the translated text is
spoken aloud through a `SpeechSynthesizer` (section 2.9), in the user's own
voice through the server when it can be, and in one of the phone's own voices
otherwise. The aim is the voice of whoever spoke, for every user (milestone 9).

Two things to notice:

- Translation always runs on the device, on both paths. The backend only
  produces a transcript. This keeps the server small and means one translation
  code path to test. Messages between people (milestone 14) will be translated
  on the server instead (section 9, ADR 0017).
- The backend is also the model distribution point (manifest and model files).

## 2. Android

Kotlin, Jetpack Compose, one Gradle module, in `android/`. minSdk 26. Native
code is built for `arm64-v8a`, which covers phones and the emulator on Apple
silicon. The application ID is `com.pelumi.shinar`; the code's package
(`namespace`) is still `com.example.ptranslate` (ADR 0021).

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

`RoutingTranscriber` has three routes: `ON_DEVICE`, `CLOUD`, and `AUTO`, which
decides per recording.

| Trigger | Definition | Checked |
| --- | --- | --- |
| Model missing | the model manager has no verified model | before transcribing |
| Device too slow | measured real-time factor (time taken / audio length) above 1.0 | before transcribing |
| Low confidence | mean token probability of the result below 0.6 | after transcribing |
| No words found | the model's no-speech probability above 0.6, or an empty result | after transcribing |
| Repeated nonsense | the text is a repetition loop, measured by how far it compresses | after transcribing |
| Model failed | the on-device transcriber raised an error | after transcribing |

A silent recording is rejected before any of this and is never uploaded.

The cloud is a fallback, not a requirement. If it cannot be reached, a
low-confidence device result is kept, and a device judged too slow is used
after all. Each transcript carries a note saying how it was routed.

The device's speed is measured by `DeviceBenchmark` once a model is installed:
it times the model on a bundled 11 second clip and stores the result per
model.

The thresholds are provisional and live in one object, `FallbackThresholds`.
ADR 0007 has the reasoning and what still needs measuring.

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

`RemoteTranscriber` expects the connection to fail. It retries an upload on a
connection error or a 5xx, with one idempotency key for all attempts. If the
event stream ends before the job does, it reconnects with `Last-Event-ID` and
the server continues from the next event. It gives up after five consecutive
tries that bring nothing new, and never retries when the server refused the
request or the job itself failed.

The backend URL is a build config field, `http://localhost:8080` for now;
`adb reverse` forwards it to the dev machine. Debug builds allow cleartext HTTP
to `localhost` and `10.0.2.2` through a debug-only network security config;
release builds do not.

### 2.9 Speech output

```kotlin
interface SpeechSynthesizer {
    // Returns when finished or stopped. onStart is called when the speech
    // becomes audible, which can be seconds later for a voice made on the server.
    suspend fun speak(text: String, language: Language, onStart: () -> Unit = {})
    fun stop()
}
```

`AndroidSpeechSynthesizer` uses the platform's `TextToSpeech`: a stock voice
per language, supplied and downloaded by the phone's speech engine. The
pipeline emits the translation, then speaks it. If it cannot be spoken, the
text stays on screen with the reason; the run is not a failure. A voice that is
still downloading on first use is retried a few times. ADR 0008 has the
reasoning.

From checkpoint 9.3 the app speaks in the user's own voice when it can.
`RemoteSpeechSynthesizer` sends the voice sample (section 2.10) and the
translation to `POST /v1/speech`, follows the job like a transcription, then
downloads the audio and plays it. `FallbackSpeechSynthesizer` tries it first
and the phone's voice second: with no voice recorded, no network, or a failed
job, the phone's voice says it instead, so the translation is always heard.
The server speaks with Fish Audio when it has a key for it (checkpoint 9.4,
ADR 0010), or with a fake that answers with one beep per word when started
with `VOICE_ENGINE=fake`. A server with no engine refuses requests to speak
with `501` (`no_voice_engine`), which the phones do not retry: the phone's
voice speaks at once.

### 2.10 The user's voice

To speak translations in the user's own voice, the app needs a sample of it.
The user records one once, by reading a passage of about 20 seconds on a
"Record your voice" screen shown at first launch.

- `VoiceSampleStore` keeps it as one WAV file in the app's private files. It
  is not uploaded when recorded. It is sent with each request to speak, and
  the server deletes it when that job ends.
- `VoiceSampleRules` refuses a recording that is silent or under 10 seconds,
  with the reason in words.
- The user can replace it or delete it at any time. Declining is remembered,
  so the app asks once.
- One voice per phone: there are no accounts, so the sample belongs to the
  device. A guest's words would be spoken in the owner's voice.
- It never leaves the phone by any other route: it is excluded from cloud
  backup and from transfer to a new phone (Android's backup rules, iOS's
  backup exclusion). From milestone 13 the server keeps each user's voice,
  with their consent (section 9, ADR 0016). So are the Whisper model, which can be downloaded again,
  and Android's `device` preferences, which describe this phone only.

The app has three screens, the interpreter, settings and recording the voice,
and no navigation library: `AppRoot` shows one of them from a state value. They
follow the Pearl design (`docs/design/pearl`, ADR 0019).

## 3. iOS

Added 2026-10-14. Swift 6 and SwiftUI, one Xcode project in `ios/` with one
app target, `PTranslate`. The bundle id was `pelumi.PTranslate` (the owner's
own prefix: installing on an iPhone needs an identifier registered to their
Apple developer team, which `com.example` cannot be) and is `com.pelumi.shinar`
since 2026-11-01 (ADR 0021); the display name comes from the `APP_NAME` build
setting. iOS 18 and later since 2026-11-10 (ADR 0025), which reaches every
iPhone from the XS and XR; until then iOS 26, for Apple's translator
(ADR 0012). whisper.cpp is built for arm64 only, which covers iPhones and the
simulator on Apple silicon, and for the A13's instructions, so the A12 of the
XS and XR never runs it and transcribes on the server (section 3.3).

Everything in section 2 applies unless this section says otherwise: the same
pipeline and seams, the same fallback triggers and thresholds, the same model
download rules, the same voice-sample rules, the same server API.

### 3.1 Why native Swift

The alternatives were Kotlin Multiplatform, sharing `core/` with Android, and
React Native. What could be shared is the smaller part: the pipeline, the
routing rules, the API client. The heavy parts are platform APIs on both
phones (recording, translation, speech, loading a C library) and would be
written twice anyway. A native app is also what the portfolio is meant to
show. The cost is that every rule exists twice and must be kept alike by
hand; the "Both phones" convention in CLAUDE.md is how. ADR 0011.

### 3.2 Layout

The same packages as Android, as folders of the app target: `Core/Audio`,
`Core/STT`, `Core/Translate`, `Core/Model`, `Core/Net`, `Core/Speech`,
`Core/Voice`, `Core/Pipeline`, and `UI/`. `Core/` imports no SwiftUI and
nothing from `UI/`, because the keyboard extension of milestone 11 will reuse
it. When the keyboard is built, `Core/` moves into a framework shared by the
app and the extension. Until then the boundary is kept by discipline, as on
Android.

### 3.3 The same design in iOS terms

| Android | iOS |
| --- | --- |
| `AudioRecord` at 16 kHz | `AVAudioEngine`, converted to 16 kHz mono 16-bit with `AVAudioConverter`: the hardware records at 44.1 or 48 kHz and cannot be asked for 16 |
| `Flow` | `AsyncThrowingStream`, fed by a task the stream owns and cancels when nobody is listening |
| `Dispatchers` | Swift concurrency. The target's default actor isolation is left at nonisolated, not Xcode's newer main-actor default, so `Core/` assumes no actor and can move into a framework unchanged; screens and view models are `@MainActor` explicitly |
| `WhisperContext` through JNI | `WhisperContext` calling whisper's C functions directly: Swift imports C headers, so there is no bridge layer to write |
| a single-thread dispatcher for whisper | `WhisperContext` is an actor on a dispatch queue of its own, so one transcription runs at a time and it never blocks Swift's shared thread pool |
| whisper.cpp built by Gradle through CMake | a script builds the same submodule with CMake and Ninja into `whisper.xcframework` (iPhone and simulator), git-ignored; on the CPU like Android, compiled for the A13 (ADR 0004 addendum). A phone without the A13's instructions (the A12 of the XS and XR) never loads it: `WhisperSupport` reads the processor's features, and such a phone sends every recording to the server, with no route to choose and no model to download (ADR 0025) |
| ML Kit Translate | ML Kit Translate too, from Google's own pre-built files, fetched and repackaged by a script (section 3.4) |
| `TextToSpeech` | `AVSpeechSynthesizer`, in the voice for the language's default region. iOS installs its voices with the system, so there is no first-use download to retry (ADR 0008 addendum) |
| `AudioTrack` | `AVAudioPlayerNode` on an `AVAudioEngine`; the mixer converts 16 kHz to the output's rate |
| `SpeechSynthesizer.stop()`, a plain function | `async`: the player belongs to the main actor, which another thread reaches only by waiting. The voice store and the player reach `RemoteSpeechSynthesizer` as main-actor closures |
| `<files>/voice/` | Application Support `voice/`, excluded from iCloud backup, so that "the recording stays on this phone" holds |
| OkHttp and `okhttp-sse` | `URLSession`. The event stream is parsed by hand from `URLSession.bytes`, because its `lines` leaves out the empty lines that end each event |
| `Sha256` | CryptoKit's `SHA256` |
| `ModelManager` with a `StateFlow` | `ModelManager`, `@Observable` on the main actor; the download (read from `URLSession.bytes`) and the checksum run off it |
| `java.util.zip.Deflater` for the repetition check | zlib's `compress2`, the same algorithm and level, so the ratio is identical |
| `<files>/models/` | Application Support `models/`, excluded from iCloud backup since it can be downloaded again |
| `DeviceBenchmark` with `StateFlow`s, started from the app's scope when a model is ready | `DeviceBenchmark`, `@Observable` on the main actor; the container watches the installed model with `nextChange` (built on `withObservationTracking`, since `Observations` needs iOS 26) and measures when one appears |
| `SharedPreferences` | `UserDefaults` |
| ViewModel and `StateFlow` | `@Observable` view model classes |
| JUnit and `MockWebServer` | Swift Testing and a `URLProtocol` stub |

### 3.4 Translation

ML Kit, as on Android, since 2026-11-10 (ADR 0024). Until then the iPhone used
Apple's Translation framework (ADR 0012): 22 languages, without a screen only
from iOS 26, and language packs iOS downloads itself behind its own sheet.
With ML Kit both phones offer the same 59 languages and translate the same
way, the app downloads the packs (31 to 48 MB each) behind its own
"Downloading language pack", and nothing needs a screen on any iOS version.

`MLKitTranslator` follows Android's `MlKitTranslator`: the same languages,
readiness check (both packs downloaded), downloads allowed over mobile data,
and messages. ML Kit keeps one translator per pair itself, so the class keeps
none. The real English-to-Spanish test runs in the simulator.

Google publishes ML Kit for iOS only for CocoaPods, as pre-built frameworks
with no code for the simulator on an Apple-silicon Mac. Neither CocoaPods nor
anyone else's repackaging is used:

- `ios/scripts/fetch-mlkit.sh` (`make ios-mlkit`) downloads MLKitTranslate
  8.0.0, MLKitNaturalLanguage 10.0.0 and MLKitCommon 14.0.0 from Google's
  server, checks each against a SHA-256 in the script, and repackages them
  into `ios/MLKit/Frameworks/` (git-ignored), as xcframeworks of static
  libraries: Xcode embeds a package's frameworks even when they are static,
  which would put three empty frameworks in the app.
- Each holds Google's arm64 iPhone code untouched, and a copy for the
  simulator: the same machine code, with the platform in each object's
  build-version load command changed from iOS to the simulator
  (`retag-for-simulator.py`). The App Store build never contains the copy.
- The local package `ios/MLKit` links them with the open-source code they
  call, from those libraries' own Swift packages, at the versions Google's
  podspecs ask for (pinned in the project's `Package.resolved`), and
  GoogleToolboxForMac 4.2.1, built from source by the same script, since its
  Swift package cannot be linked by Xcode.
- Google's privacy manifest ships in `MLKitTranslate_resource.bundle`, which
  the app copies and the translator reads its pack list from.

It costs about 31 MB in the app (16 MB on Android). ML Kit reports usage and
diagnostics to Google, not linked to the user, as its privacy manifest
declares; the store listings must say so on both phones.

### 3.5 Reaching the server

The simulator shares the Mac's network, so `http://localhost:8080` works as on
the Android emulator. A real iPhone has no `adb reverse`: it uses the Mac's
address on the same Wi-Fi, such as `http://<mac-name>.local:8080`. The server
already listens on every interface (`HTTP_ADDR` defaults to `:8080`). The
address is a build setting, `BACKEND_URL`: `localhost` for the simulator, and
for iPhone builds the owner's Mac by its Bonjour name
(`http://OgundipePelumi.local:8080`). `make ios-device` reads the Mac's name
afresh, builds, installs on the connected iPhone and opens the app;
`MAC_HOST=<address>` overrides the name, for a network where Bonjour names do
not resolve. Plain HTTP is allowed only to local addresses, through App
Transport Security's `NSAllowsLocalNetworking`, and iOS asks the user once
for permission to reach the local network.

### 3.6 Simulator and iPhone

The simulator covers most of the work: it records from the Mac's microphone
and runs whisper.cpp on the Mac's hardware. Translation is the exception:
Apple's does not run there (section 3.4). Its timings say nothing
about a phone, and the device benchmark (section 2.6) only means something on
a real iPhone. A free Apple ID is enough to install on the owner's iPhone; the
app stops opening after 7 days and is installed again from Xcode.

### 3.7 The keyboard, ahead

iOS does not let a keyboard extension use the microphone. Apps that offer
voice typing have their main app record in the background and pass the words
to the keyboard through a shared App Group container. Apple documents none of
this, and an app in the background may not use the GPU, so whisper.cpp would
run on the CPU there. Milestone 11 starts with a test on a real iPhone before
anything is built on it.

## 4. Server

Go, one binary. By default it runs the HTTP API and the worker pool in the
same process; with `-role api` or `-role worker` it runs one of them, so they
can be separate processes. Postgres is the only external service.

### 4.1 Endpoints

**`POST /v1/jobs`**: `multipart/form-data` with an `audio` part (WAV) and a
`source_lang` field. Signed in only, since 2026-11-09 (ADR 0023). The body is limited to 10 MB with `http.MaxBytesReader`.
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

An optional `Idempotency-Key` header makes the upload safe to repeat. A phone
whose connection drops after sending cannot know whether the job was created;
it sends the same request again with the same key and gets the same job back.
A unique index on the key guarantees one job even if both attempts arrive
together.

Event types: `queued`, `processing`, `partial`, `done`, `error`. The stream
closes after `done` or `error`. A comment line is sent every 15 seconds to keep
proxies from closing an idle connection.

**`POST /v1/speech`**: `multipart/form-data` with the `text` to say, its
language as `lang`, and the voice to say it in: a `voice` part (a WAV sample
of the voice to imitate), or, with no `voice` part, the user's kept voice.
Signed in only, since 2026-11-09 (ADR 0023). Like a transcription it is queued and
answered with `202 Accepted`:

```json
{ "id": "c688...", "status": "queued",
  "events_url": "/v1/jobs/c688.../events", "audio_url": "/v1/jobs/c688.../audio" }
```

Progress comes over the same events endpoint (`queued`, `processing`, `done`
or `error`). The job works on its own copy of the voice and deletes it when
it ends; a kept voice stays. A signed-in user with no kept voice gets `409`
with code `no_voice`. A server with no voice engine answers
`501 Not Implemented` with code `no_voice_engine` instead of queuing the
request.

**`GET /v1/jobs/{id}/audio`**: the generated speech, a WAV file, with range
support. `409` with code `not_ready` while the job has no audio: it is
unfinished, it failed, or an hour has passed and the audio was deleted.

**Accounts** (milestone 13, ADR 0018). Signed-in requests carry
`Authorization: Bearer <token>`; without a working session they get `401`
with code `unauthenticated`.

| Endpoint | What it does |
| --- | --- |
| `POST /v1/sessions/apple` | JSON `identity_token`, `nonce`, optional `name`. Checks the token, creates the account on its first sign-in, answers `{ "token", "user", "new_user" }`: `201` for a new account, `200` otherwise. `401 invalid_token` for a token that fails a check; `503 sign_in_unavailable` when Apple's keys cannot be fetched |
| `POST /v1/sessions/google` | the same, with a Google ID token; `501 sign_in_not_configured` when the server has no `GOOGLE_CLIENT_ID` |
| `DELETE /v1/sessions/current` | signs out this session |
| `GET /v1/me` | the profile: `{ "id", "display_name", "handle", "language" }` |
| `PATCH /v1/me` | changes any of `display_name`, `handle`, `language`; `409 handle_taken` |
| `DELETE /v1/me` | deletes the account, its sessions and its voice |
| `PUT /v1/me/voice` | `multipart/form-data`: the `voice` (WAV) and `consent`, the version of the consent wording the user agreed to. Replaces any earlier voice. Answers `{ "consent_version", "consented_at" }` |
| `GET /v1/me/voice` | the same, or `404 no_voice` |
| `GET /v1/me/voice/audio` | the user's own kept sample, for a phone they sign in on later |
| `DELETE /v1/me/voice` | deletes the voice |
| `GET /v1/handles/{handle}` | whether the user may take an ID: `{ "handle", "available", "suggestions" }`, with three free IDs when it is taken; `400 invalid_handle` for one that breaks the rules |
| `GET /v1/users/{handle}` | someone by their ID, the same shape as `/v1/me`; `404 user_not_found`, also for someone who has not chosen a language |

**Rooms** (milestone 14, ADR 0020). All signed in. A room the user is not in
is answered as not found.

| Endpoint | What it does |
| --- | --- |
| `POST /v1/rooms` | JSON `handle`: opens a room of two with that person, or returns the one the two have (`201` / `200`). `400 cannot_add_self`, `404 user_not_found`, `409 profile_incomplete` before the user has a language |
| `GET /v1/rooms` | `{ "rooms": [ { "id", "people", "last_message", "unread", "created_at" } ] }`, most recent first; `people` is everyone but the user |
| `GET /v1/rooms/{id}` | one room |
| `GET /v1/rooms/{id}/messages` | `{ "messages": [...] }`, oldest first: the newest 50, or `?before=<message id>&limit=` for older ones |
| `POST /v1/rooms/{id}/messages` | `multipart/form-data`: `audio` (WAV, up to 30 s), `transcript` (written on the phone, up to 1000 characters), `duration_ms`, optional `lang`; `Idempotency-Key` as for jobs. `201` with the message; `409 room_closed` when the other person has deleted their account |
| `POST /v1/rooms/{id}/read` | the user has seen the room so far |
| `GET /v1/messages/{id}` | one message |
| `POST /v1/messages/{id}/retry` | the sender asks for a failed translation to be tried again |
| `GET /v1/messages/{id}/audio?lang=` | the audio in one language: the sender's recording for the language it was said in, or the spoken translation once ready; `404 no_audio` until then |
| `GET /v1/me/events` | Server-Sent Events: `ready` once listening, then `room` with `{ "room_id", "message_id" }` whenever something in one of the user's rooms changes. Hints only, nothing replayed: the phone fetches on `ready` |

A message as the user sees it: `{ "id", "room_id", "sender_id", "mine",
"lang", "transcript", "duration_ms", "created_at", "renditions" }`. Each
rendition is `{ "lang", "status", "text", "voice", "audio_url" }`, with
`status` one of `translating`, `speaking`, `ready`, `failed`, and `voice` one
of `own` (the sender's voice), `original` (their recording) or `phone` (the
listener's phone reads `text`). The sender sees a rendition per language the
others speak; a listener sees their own.

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

### 4.2 Data model

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

-- Added later: jobs.idempotency_key (milestone 5), and for speech synthesis
-- (milestone 9) jobs.kind, target_lang, input_text and result_path.
-- Accounts (milestone 13): users, identities (provider, subject) -> user,
-- sessions (token_hash -> user), and voices (one per user: the blob key,
-- the consent version and when it was given). See migrations 0004 and 0005.
-- Rooms (milestone 14): rooms (pair, created_by), room_members (read_at),
-- messages (sender, language, transcript, audio, duration, idempotency key)
-- and renditions (one per message and listener language: status, text,
-- voice, audio). jobs.kind gains 'deliver', with jobs.message_id. See
-- migration 0006 and ADR 0020.

CREATE TABLE job_events (
    job_id     uuid NOT NULL REFERENCES jobs(id) ON DELETE CASCADE,
    seq        int  NOT NULL,            -- 1, 2, 3 ... per job; the SSE id
    type       text NOT NULL,
    payload    jsonb NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (job_id, seq)
);
```

### 4.3 Job queue

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
- **Retries**: an error is assumed to be temporary unless the provider marks
  it with `provider.Permanent`. A temporary error with attempts left puts the
  job back to `queued` with `run_at = now() + backoff`. Backoff is exponential
  with jitter (2 s, 4 s, 8 s ... capped at a minute, each moved by up to 20%).
  The retry is recorded as a `queued` event carrying the error and the delay.
  Out of attempts, or a permanent error: `failed`, and an `error` event.
- **Crashed workers**: a running worker renews its lease as a heartbeat. A
  job whose lease has run out therefore has no live worker, and a periodic
  sweep puts it back to `queued`. The lost attempt counts.
- **Stale workers**: every write a worker makes requires the attempt number
  it claimed (`AND attempts = $2`). A worker that stalled and lost its job to
  another cannot overwrite it.
- **Graceful shutdown**: on SIGINT or SIGTERM, stop accepting HTTP requests,
  stop claiming jobs, let in-flight jobs finish for up to 30 seconds, then
  cancel their contexts. A cancelled job is released back to `queued` without
  counting the attempt.

### 4.4 Events and SSE

The worker and the SSE handler are different goroutines and may be different
processes, so events go through the database rather than straight from one to
the other.

1. Whoever records an event inserts it into `job_events` and, in the same
   statement, announces it with `pg_notify('job_events', '<job id> <seq>')`.
   Postgres delivers the announcement when the transaction commits.
2. Each process that serves streams runs a `Listener` on a dedicated
   connection. It hears every announcement, and for jobs that have a
   subscriber in its process it loads the event and puts it on the local
   `Bus`.
3. The SSE handler subscribes to the bus first, then reads the job's existing
   events from the table (after `Last-Event-ID` if the client sent one), writes
   them, and then writes live events, dropping any whose `seq` it already sent.

Subscribing before reading closes the gap in which an event could be missed. It
also covers the common case of a client connecting after the job has already
finished, and reconnects after a dropped connection.

If the listener loses its connection, announcements made meanwhile are gone.
It therefore ends every open stream before reconnecting; clients reconnect and
replay from the table. ADR 0002 and ADR 0006 have the reasoning.

The server runs in one of three roles, chosen with `-role`: `all` (default),
`api` (HTTP and the listener), or `worker` (the pool and the sweeper).

### 4.5 Providers

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

Speech synthesis has the same shape, a second interface in the same package:

```go
type Synthesizer interface {
    // Synthesize speaks opts.Text in the voice heard in the sample, writing a
    // WAV file to out.
    Synthesize(ctx context.Context, voice io.Reader, opts SynthesisOptions, out io.Writer) error
}
```

Two implementations: `fish.Synthesizer` calls Fish Audio (ADR 0010), sending
the voice sample with each request along with its transcript, the passage the
phones ask users to read (`provider.OnboardingPassage`); and a fake that
answers with one beep per word, for tests and for working without a key.
Both kinds of work run through the
one queue: a job row has a `kind`, and a worker looks at it to choose the
provider. Claiming, leases, heartbeats, retries, fencing and events are shared
and know nothing about which kind they are handling.

### 4.6 Audio storage

A small `BlobStore` interface (`Put`, `Open`, `Delete`) with a local directory
implementation. Audio is deleted when its job reaches `done` or `failed`.
Users' kept voices live in a second directory (`VOICES_DIR`), apart from job
audio, and are deleted only when the user replaces or deletes the voice, or
deletes the account (ADR 0016). Room messages live in a third
(`MESSAGES_DIR`): each sender's recording and the copies spoken in their
voice, kept until the sender deletes their account (ADR 0020).

### 4.7 Configuration and logging

Environment variables read once in `main` (`DATABASE_URL`, `HTTP_ADDR`,
`WORKERS`, `AUDIO_DIR`, `VOICES_DIR`, `MODELS_DIR`, `JOB_LEASE`,
`SWEEP_INTERVAL`, `VOICE_ENGINE`, `FISH_AUDIO_API_KEY`, `FISH_AUDIO_MODEL`,
`APPLE_AUDIENCE` (default `com.pelumi.shinar`), `GOOGLE_CLIENT_ID`, `MESSAGES_DIR`, `TRANSLATE_ENGINE`,
`GOOGLE_TRANSLATE_KEY`, `AZURE_TRANSLATOR_KEY`, `AZURE_TRANSLATOR_REGION`). Secrets live in
`server/.env`, which git ignores and the Makefile's server targets load.
Structured logs with `log/slog`, with
the job id on every line that concerns a job.

### 4.8 Hosting

Since 2026-11-09 (ADR 0023): one Ubuntu 24.04 machine on Azure (B2ats v2,
South Africa North), with Caddy in front for HTTPS on the machine's Azure
name and the server under systemd behind it, its files in `/var/lib/shinar`,
its settings in `/etc/shinar/env` (template `deploy/env.example`); and Azure
Database for PostgreSQL 18 (flexible server, B1ms) in the same region,
reachable only from the machine. `deploy/setup.sh` prepares the machine once. GitHub Actions
(`.github/workflows/server.yml`) tests every push to `main` that changes the
server and ships it with `deploy/ship.sh`, which `make server-ship` also
runs.

## 5. Testing

| Area | How |
| --- | --- |
| Queue (required) | Go tests against the local Postgres, in a separate test database where every test gets its own schema. Covers: concurrent workers never claim the same job, retry with backoff, giving up after max attempts, expired lease recovery, shutdown releasing in-flight jobs. `SKIP LOCKED` behaviour cannot be faked meaningfully. |
| SSE handler (required) | `httptest` with an in-memory event source, no database. Covers: framing, replay from `Last-Event-ID`, no duplicates across the replay/live boundary, stream closes on terminal event, handler returns when the client disconnects. |
| Model checksum (required) | Kotlin JVM tests with OkHttp `MockWebServer`: good hash, bad hash, resumed download, server ignoring `Range`. Go test that manifest hashes match the files on disk. |
| Fallback policy | Kotlin JVM tests with fake `Transcriber`s. |
| iOS | Swift Testing, on the simulator. The same cases as the Kotlin tests wherever the logic is the same (routing, model download, upload retries and reconnect, transcript checks, voice-sample rules), with fakes and a `URLProtocol` stub. whisper.cpp is tested on the simulator with its sample clip. |
| Everything else | Tested where logic exists; no tests for wiring. |

## 6. Dependencies

Anything outside this list needs approval first. Items marked *added* were not
in the original spec; the owner approved them on 2026-10-04.

**Server**

| Dependency | Why |
| --- | --- |
| Go standard library | HTTP, routing, logging, tests |
| Postgres 16, Homebrew `postgresql@16` (*added*) | database and queue. Installed locally instead of Docker to save disk and RAM on the dev machine. |
| `github.com/jackc/pgx/v5` (*added*) | Postgres driver and connection pool. A driver is unavoidable; pgx is the standard one and supports `LISTEN/NOTIFY`. |
| Fish Audio API (*added* 2026-10-31) | the voice-cloning engine, a paid service (ADR 0010). Called with `net/http` and a small MessagePack encoder of our own: no Go module. |
| Google Cloud Translation API, basic edition (*added* 2026-10-31) | translates room messages, a paid service (ADR 0020). Called with `net/http`: no Go module. |
| Azure AI Translator, free tier F0 (*added* 2026-11-09) | translates room messages while Google cannot be billed (ADR 0020, addendum). Called with `net/http`: no Go module. |

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
| `androidx.credentials:credentials` and `credentials-play-services-auth` 1.6.0, `com.google.android.libraries.identity.googleid:googleid` 1.2.0 (*added* 2026-10-31) | Sign in with Google through Android's Credential Manager (ADR 0018). The second artefact makes it work on Android 13 and older, through Play services. `googleid` 1.2.1 was approved, but it is built with Kotlin 2.4, which the Kotlin bundled with AGP 9 cannot read, so 1.2.0 is used |
| `com.google.zxing:core` 3.5.4 (*added* 2026-10-31) | draws the user's QR code. Plain Java, no other libraries |
| `com.google.android.gms:play-services-code-scanner` 16.1.0 (*added* 2026-10-31) | scans someone's QR code. Play services draws the camera screen, so the app needs no camera permission |

No DI framework, no Retrofit, no Room.

**iOS** (approved by the owner on 2026-10-14)

| Dependency | Why |
| --- | --- |
| Xcode 26.4 and the iOS 26 SDK | needed for any iOS app; already installed |
| SwiftUI, AVFoundation, CryptoKit, Foundation (`URLSession`) | part of the SDK; nothing to install |
| AuthenticationServices, Security (Keychain) | part of the SDK; Sign in with Apple and keeping the session token (ADR 0018) |
| Core Image, VisionKit | part of the SDK; drawing the user's QR code, and scanning someone's |
| whisper.cpp (the same submodule) | the same engine as on Android |
| CMake and Ninja, through Homebrew | build whisper.cpp into an xcframework. Already installed on the dev machine (CMake 4.0, Ninja 1.12), but new as build tools of this project |
| Swift Testing | tests; part of Xcode |
| ML Kit Translate for iOS: MLKitTranslate 8.0.0, MLKitNaturalLanguage 10.0.0, MLKitCommon 14.0.0 (*added* 2026-11-10) | the translator, as on Android (ADR 0024), in place of Apple's Translation framework. Google's pre-built files from `dl.google.com`, fetched and checked by `make ios-mlkit` |
| Swift packages GTMSessionFetcher 3.x, GoogleDataTransport 10.x (with nanopb and Promises), GoogleUtilities 8.x, ZipArchive 2.x (*added* 2026-11-10) | the open-source code ML Kit calls, at the versions its podspecs ask for, linked by the local package `ios/MLKit` |
| GoogleToolboxForMac 4.2.1 (*added* 2026-11-10) | three small parts ML Kit calls, built from Google's tagged source by the same script: version 4 has no Swift package, and the later one cannot be linked |

No CocoaPods and no project generator: the owner creates the Xcode project, as
with the Android one. The only Swift packages are the ones ML Kit needs.

**Builds** (*added* 2026-11-02, ADR 0022)

| Dependency | Why |
| --- | --- |
| GitHub Actions, the `macos-26` runner, and GitHub's `actions/checkout` | the TestFlight workflow. Signing and the upload are `xcodebuild` and `security`, with no other actions or tools |
| An App Store Connect API key, a distribution certificate and profile | kept as repository secrets; the key uploads, the certificate signs |

**Hosting** (*added* 2026-11-09, ADR 0023)

| Dependency | Why |
| --- | --- |
| An Azure virtual machine (B2ats v2, Ubuntu 24.04 LTS) and Azure Database for PostgreSQL (flexible server, B1ms) | the server and its database, free for 12 months |
| Caddy, from Caddy's own apt repository | HTTPS in front of the server, with certificates it gets and renews itself |
| GitHub's `actions/setup-go`, and the `postgres:18` image as a service | the server workflow: Go at the version in `go.mod`, and a database for the tests |

## 7. Planned ADRs

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
| 0008 | Speech output with the phone's voices (written) | 8 |
| 0009 | Cloud transcription provider | 7 |
| 0010 | Voice-cloning engine: Fish Audio (written) | 9 |
| 0004 addendum | whisper.cpp on iOS: the build, GPU or CPU | 10 |
| 0011 | A native iOS app on the same server | 10 |
| 0012 | Translation on iOS (superseded by 0024) | 10 |
| 0013 | Voice keyboards, and recording for the iPhone keyboard | 11 |
| 0014 | Voice notes from other apps (not written: milestone 12 was dropped) | 12 |
| 0015 | From a portfolio piece to a social product (written) | 13 |
| 0016 | Users' voices are kept on the server (written) | 13 |
| 0017 | Messages are translated on the server (written) | 14 |
| 0018 | Sign-in and sessions (written) | 13 |
| 0019 | The Pearl design in both apps (written) | 19 |
| 0020 | Rooms, message delivery, and Google Cloud Translation (written) | 14 |
| 0021 | The name Shinar, and the app identifiers (written) | 13 |
| 0022 | TestFlight builds from GitHub Actions (written) | 18 |
| 0023 | Hosting the server on Azure (written) | 18 |
| 0024 | ML Kit on iOS (written) | 18 |
| 0025 | iOS 18 as the iPhone app's minimum (written) | 18 |

## 8. Known limits

- The job, speech and model endpoints need no sign-in yet: anyone who can
  reach the server can upload audio and ask for speech. Job ids are random
  UUIDs, so one client cannot guess another's event stream, and uploads are
  size-limited. Not suitable for a public deployment. They will require a
  session once both phones sign in (ADR 0018).
- ML Kit translation covers about 59 languages and translates through English
  when neither side is English, so quality between two non-English languages is
  modest.
- ML Kit adds about 31 MB to the iPhone app.
- On the iPhone XS and XR, speech is always turned into text on the server:
  the Whisper build needs the A13's instructions.
- Recordings are limited to 30 seconds.
- The low-confidence fallback makes the user wait twice: once for the device,
  then for the cloud.

## 9. The social product (direction)

Added 2026-10-31, when the app became a product (ADR 0015). This is the
direction, not yet the design: each milestone from 13 on designs its part,
and sections 1 to 4 are corrected as it lands.

People sign up with their name and their voice, and can talk to anyone, each
in their own language: what one says reaches the other in the listener's
language and the speaker's voice. Voice notes first (milestone 14), then text
messages (15), later live calls for premium users (17).

What stays: the two native apps and their seams; transcription on the phone
with the server as the fallback; the Go server, Postgres, the job queue and
its events. The interpreter screen built so far stays, and keeps translating
on the phone.

What changes:

- **Accounts** (milestone 13). A user has a display name, a handle others
  find them by, and the language they speak. They sign in with Apple, with
  Google, or with a one-time code sent to their email. The phone keeps a
  session token; the server keeps only a hash of it.
- **Voices on the server** (milestone 13, ADR 0016). The sample recorded at
  onboarding is uploaded once, with the user's explicit consent, and kept
  until they replace or delete it, or delete their account. It is used only
  to speak that user's own words. The "stays on this phone" promise of
  section 2.10 ends here, and the onboarding screens say so.
- **Translation on the server, for messages** (milestone 14, ADR 0017). A
  message is translated into its listener's language before it is spoken in
  the sender's voice, which happens on the server. The phones' translators
  cannot run on a server, so this needs a translation service.
- **A voice note's path.** The sender's phone records and transcribes it, as
  today, and uploads the audio, the transcript and its language. The server
  stores the message, translates the transcript for the listener, speaks the
  translation in the sender's voice, stores that audio, and notifies the
  listener's phone (APNs, FCM). That phone plays it and shows both texts.
  Translating and speaking are jobs on the existing queue, with its retries,
  leases and events.
- **Rooms** (milestones 14 and 20). A room is a conversation: two people
  first, groups later. Its messages update live while it is open, over the
  same event stream and `LISTEN/NOTIFY` bus as jobs; push notifications reach
  a phone that is not in it.
- **Safety** (milestone 16): block, report, and acting on reports, before
  anyone can be found at random.
- **Calls** (milestone 17): a stream, not jobs; designed when they come.
- **Going live** (milestone 18): a public HTTPS server, storage for audio,
  and a privacy policy that covers voices.

Every voice note costs money: a translation and a synthesis per listener.
The engine is chosen per request (section 4.5), so a cheaper engine for free
users stays possible.
