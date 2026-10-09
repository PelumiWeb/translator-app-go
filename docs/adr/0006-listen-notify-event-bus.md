# ADR 0006: Postgres LISTEN/NOTIFY to carry events between processes

Status: accepted, 2026-10-09. Replaces the in-process delivery described in
ADR 0002; the rest of ADR 0002 stands.

## Context

ADR 0002 made `job_events` the record of what happened to a job, and used an
in-process bus to tell open SSE streams that a new row existed. The store
called the bus directly after each commit.

That only works while the code that records an event and the code that streams
it share a process. It stops working the moment there are two server
processes, which is the normal shape of a deployment: more than one API
instance behind a load balancer, or workers scaled separately from the API. A
client's stream is held by one process, and the worker doing its job is in
another. The events would be written to the table, and the client would never
hear of them until it reconnected.

This was seen in practice during milestone 5: a second server started against
the same database picked up a job, and the first server's stream stayed
silent.

## Decision

Events are announced with Postgres `NOTIFY` and heard with `LISTEN`.

**Announcing.** The statement that inserts an event also calls
`pg_notify('job_events', '<job id> <seq>')`. Postgres makes a notification part
of the transaction that sends it: it is delivered when the transaction commits
and dropped if it rolls back. The rule from ADR 0002, "commit, then publish",
is now enforced by the database instead of by the order of two lines of Go.

**Hearing.** Every process that serves streams runs one `Listener`. It holds a
dedicated connection, executes `LISTEN job_events`, and waits. For each
notification it checks whether anyone in this process is following that job;
if so it loads the event row and hands it to the local `Bus`, which feeds the
open streams exactly as before.

**One path.** The same route is used when the worker is in the same process as
the stream. There is no shortcut for the local case, so single-process
development exercises the code that production depends on.

**The notification is a pointer, not the event.** It carries the job id and
sequence number only. The table stays the single source of the event's
content, and the payload can be any size, where a notification is limited to
8000 bytes.

**When in doubt, end the stream.** Notifications are not stored: one sent
while a listener is disconnected is gone. So if the listener's connection
drops, it ends every open subscription in its process before reconnecting, and
if it cannot load an announced event, it ends the subscriptions for that job.
The SSE stream closes, the client reconnects with `Last-Event-ID`, and the
handler replays what was missed from the table. A stream is never left open
with a silent gap in it.

**Roles.** The server takes a `-role` flag: `all` (the default), `api`, or
`worker`. An `api` process runs the HTTP server and the listener; a `worker`
process runs the worker pool and the sweeper.

## Alternatives considered

**Poll the table.** Each stream asks every half second for events after its
last sequence number. No listener, no dedicated connection, nothing to lose.
It costs a query per open stream per interval whether or not anything
happened, and partial transcripts arrive in visible steps. A reasonable
fallback if `LISTEN/NOTIFY` ever became a problem.

**Redis pub/sub, NATS, Kafka.** Purpose-built, and the right answer at a scale
this project does not have. Each is another service to run, and none can make
publishing atomic with the database commit, which Postgres gives for free.

**Put the whole event in the notification.** Saves the listener one query per
event. It caps the payload at 8000 bytes and makes the notification a second
copy of the truth.

**Sticky routing, so a client's stream and its job share a process.** Avoids
the problem instead of solving it, and fails as soon as the API and the
workers are different programs.

## Consequences

- The API and the workers can run as separate processes, and there can be
  several of each.
- Each process that serves streams holds one extra database connection, for
  as long as it runs. With a connection pooler in transaction mode
  (PgBouncer), `LISTEN` does not work; the listener would need a direct
  connection.
- Delivery is now asynchronous. A subscriber can receive an event recorded
  just before it subscribed. The SSE handler already skips events at or below
  the last sequence number it sent, so this is invisible to clients.
- Every listener hears every event in the system and discards those for jobs
  it has no subscribers for. That check is a map lookup; the database query
  happens only when someone is watching.
- When the API has no workers in its own process, it cannot wake one directly
  on upload. The other process's workers find the job on their next poll, at
  most a second later.
- Processes on different machines need the uploaded audio in storage they can
  all reach. The local-directory store only works while they share a disk.
- The Android client must reconnect a stream that ends early. Until checkpoint
  5.4 adds that, a listener reconnect surfaces to the user as a failed
  transcription.
