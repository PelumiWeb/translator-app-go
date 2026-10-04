# Voice Translation

Record speech on Android, transcribe it, translate it into a target language
chosen for the receiver. Portfolio piece that must prove three things: native
Android, on-device inference, and Go backend services.

Read `docs/architecture.md` for the design and `docs/plan.md` for the current
milestone before starting any work.

## Who the owner is

Senior React Native engineer, comfortable in Kotlin/Compose, newer to Go.

- Favour idiomatic, readable code over cleverness, in both languages.
- When a Go decision is not obvious, explain it in a one or two line comment at
  the point of the decision (why this, not what it does).
- Kotlin needs no such hand-holding; comment only what is surprising.

## Working agreement

1. One milestone at a time, in the order of `docs/plan.md`. Do not start the
   next one until the owner approves the current one.
2. The owner makes the commits. Never run `git commit`, `git push`, or anything
   that rewrites history. At each checkpoint in the plan: stop, summarise what
   changed and how to verify it, suggest a commit message, and wait.
3. Ask before adding any dependency that is not in the approved list in
   `docs/architecture.md` (section "Dependencies"). That includes Gradle
   plugins, Go modules, Homebrew formulas and git submodules.
4. If something in the spec or the plan looks wrong, say so before building on
   it. Do not quietly work around it.
5. A significant decision gets an ADR in `docs/adr/NNNN-short-title.md`, written
   in the milestone where the decision lands.
6. Report results as they are. If tests fail or a step was skipped, say so.

## Structure

```
/android   Kotlin, Jetpack Compose, single Gradle module (:app)
/server    Go service: HTTP API, job queue, workers
/docs      architecture.md, plan.md, adr/
Makefile   common commands for both sides
```

Android package layout (inside `:app`):

```
core/audio       microphone capture, 16 kHz mono PCM, WAV encoding
core/stt         Transcriber interface and its implementations
core/translate   Translator interface and its implementations
core/model       model manifest, download with resume, SHA-256 check
core/net         backend client: upload, SSE
core/pipeline    record -> transcribe -> translate, the reusable entry point
ui/              Compose screens and ViewModels
```

Server layout:

```
cmd/server           main: wiring, config, graceful shutdown
internal/api         HTTP handlers, SSE
internal/queue       job store (Postgres), worker pool
internal/provider    Provider interface, fake provider
internal/models      manifest and model file serving
internal/storage     audio blob storage
migrations/          plain SQL, embedded in the binary
```

## Conventions

### Android

- `core/` must stay reusable by a future IME. It may not import Compose,
  `Activity`, `ViewModel`, or anything from `ui/`. If it needs a `Context`, it
  takes the application context through its constructor.
- Dependencies are passed through constructors. One `AppContainer` builds the
  object graph. No DI framework.
- Coroutines and `Flow` for async work. No `GlobalScope`. Blocking and native
  calls run on an injected dispatcher so tests can replace it.
- `Transcriber` and `Translator` are the seams. UI and pipeline code depend on
  the interfaces, never on whisper.cpp or ML Kit types.
- JNI surface stays small: one Kotlin class owns the native handle and is the
  only caller of `external` functions.

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

### API

- JSON, snake_case field names, versioned under `/v1`.
- Errors: `{"error": {"code": "...", "message": "..."}}` with a matching status.

## Commands

Postgres is a local Homebrew install (`postgresql@16`), not Docker: the dev
machine is short on disk and RAM. `make help` lists every target. The Android
targets arrive with checkpoint 1.4.

| Command | What it does |
| --- | --- |
| `make db-install` | one-time `brew install postgresql@16` |
| `make db-up` / `make db-down` | start / stop Postgres, create the dev and test databases |
| `make db-psql` | open `psql` on the dev database |
| `make server-run` | run the Go server (applies migrations on start) |
| `make server-test` | Go tests; queue tests need `make db-up` first |
| `make server-test-race` | the same with the race detector, each test run 3 times |
| `make server-lint` | `go vet` and `gofmt -l` |
| `make sample-job` | upload a test clip to the running server and stream its events |
| `make android-build` | `./gradlew assembleDebug` |
| `make android-test` | JVM unit tests |
| `make android-install` | install the debug build on the connected device |
| `make device-proxy` | `adb reverse tcp:8080 tcp:8080` for a physical device |

The emulator reaches the host server at `http://10.0.2.2:8080`.

## Non-goals for now

Do not build, scaffold, or add placeholders for any of these:

- iOS
- the system keyboard (IME); only keep `core/` reusable for it
- auth, accounts, billing
- multi-region
