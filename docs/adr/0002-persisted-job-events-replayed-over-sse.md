# ADR 0002: Job events are persisted and replayed over SSE

Status: accepted, 2026-10-04

## Context

A client uploads audio, gets a job id, and then opens
`GET /v1/jobs/{id}/events` to follow progress. The worker that produces the
progress and the handler that streams it are different goroutines, and from
milestone 5 may be different processes.

Sending events straight from worker to handler loses them in ordinary
situations:

- The client connects after the job has started, or after it has finished.
  With the fake provider a job can be claimed within a millisecond of the
  upload returning.
- A mobile connection drops mid-job and the client reconnects.
- The server restarts.

## Decision

Every event is a row in `job_events`, numbered 1, 2, 3 ... per job. The table
is the record of what happened. An in-process bus only announces that a new
row exists.

**Writing.** The store inserts the event in the same transaction as the status
change it describes, and publishes it on the bus only after that transaction
commits. A listener never hears about something that was rolled back.

**Reading.** The SSE handler:

1. subscribes to the bus for that job;
2. reads the job's events from the table, after `Last-Event-ID` if the client
   sent one, and writes them;
3. writes events from the bus as they arrive, skipping any whose `seq` it has
   already sent;
4. closes the stream after `done` or `error`.

Subscribing before reading is what makes this correct. An event recorded
between steps 1 and 2 appears in both and is sent once. In the other order it
would appear in neither.

The event's `seq` is sent as the SSE `id`, so the standard `Last-Event-ID`
reconnect works with no extra protocol.

**Slow clients.** Each subscription has a buffer of 64 events. Publishing
never blocks, so a stalled client cannot slow a worker. When a buffer
overflows, the bus ends that subscription and the handler closes the stream.
The client reconnects and replays from the table. An event is never silently
skipped.

**Shutdown.** Streams end when the server begins shutting down. Without that,
`http.Server.Shutdown` would wait on them for its whole timeout.

## Alternatives considered

**In-memory only.** Simplest, and loses events in every case listed above.

**Polling a status endpoint.** No streaming at all. Works, but partial
transcripts would arrive in visible jumps, and the spec asks for SSE.

**WebSockets.** Bidirectional, which is not needed: the client only listens.
SSE is plain HTTP, has reconnection with `Last-Event-ID` built into the
format, and needs no library on the server.

**Postgres `LISTEN/NOTIFY` from the start.** Needed once the API and the
workers run as separate processes, and planned for milestone 5. Until then it
would add a dedicated connection and reconnect handling for no benefit. The
bus sits behind small interfaces (`queue.Publisher`, `api.EventSubscriber`),
so swapping it does not touch the handler or the store's callers.

## Consequences

- A client can connect at any time, as often as it likes, and sees the same
  sequence of events.
- Every partial transcript is a database write. Fine for 30 second clips; a
  provider that emits hundreds of partials would want them throttled.
- `job_events` grows without bound. Nothing prunes it yet.
- The in-process bus is correct only while the API and the workers share a
  process, which is the case until milestone 5.
- Until the Android client sends `Last-Event-ID` (milestone 5), a dropped
  stream is replayed from the start, which is correct but repeats events.
