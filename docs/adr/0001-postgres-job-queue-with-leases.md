# ADR 0001: Postgres as the job queue, with lease-based claims

Status: accepted, 2026-10-04

## Context

Uploaded audio has to be transcribed in the background: the upload request
returns at once and the client follows progress over SSE. That needs a queue
that survives a restart, lets several workers run without taking the same job,
and can retry.

The server already needs Postgres to store jobs and their results.

## Decision

The `jobs` table is the queue. There is no separate broker.

A worker claims a job with a single short transaction:

```sql
UPDATE jobs
SET status = 'processing', attempts = attempts + 1,
    locked_until = now() + $1::interval, updated_at = now()
WHERE id = (
    SELECT id FROM jobs
    WHERE status = 'queued' AND run_at <= now()
    ORDER BY run_at
    FOR UPDATE SKIP LOCKED
    LIMIT 1
)
RETURNING id, source_lang, audio_path, attempts;
```

`FOR UPDATE SKIP LOCKED` makes concurrent workers pass over a row that another
worker is claiming at that moment, instead of waiting for it. Each worker gets
a different job and none blocks another.

The claim commits immediately. While the provider runs, ownership is recorded
in the row itself: `status = 'processing'` and a lease, `locked_until`. Every
later status change (`done`, `failed`, back to `queued`) includes
`WHERE status = 'processing'`, so a worker that has lost the job changes
nothing.

Shutdown has two stages. On SIGINT or SIGTERM workers stop claiming; running
jobs get a grace period to finish. Jobs still running after that are cancelled
and released back to `queued` without counting the attempt.

## Alternatives considered

**Hold the claiming transaction open while the job runs.** This is the form
most `SKIP LOCKED` examples show, and a crashed worker releases its job for
free when its connection drops. Rejected because:

- each running job pins one database connection for seconds or minutes;
- uncommitted changes are invisible to other connections, so the API could
  never report `processing`, and partial transcripts written inside the
  transaction could not be streamed;
- long transactions hold back Postgres vacuum.

**A dedicated broker (Redis, RabbitMQ, SQS).** One more service to run, and
enqueueing would no longer be atomic with inserting the job row: a crash
between the two leaves a job that is stored but never queued, or the reverse.
At this scale Postgres is more than fast enough.

**A Go queue library (River, and similar).** They implement this same pattern
well. Rejected here because writing the queue is one of the things this
project is meant to demonstrate.

## Consequences

- Enqueue is one transaction: the job row and its `queued` event exist
  together or not at all.
- Idle workers poll once a second. An in-process wake-up on enqueue removes
  that delay for the common case; the poll is the safety net.
- Jobs are claimed in `run_at` order, which is roughly but not strictly first
  in, first out once several workers run.

## Addendum, 2026-10-08: heartbeats, the sweeper and fencing

Milestone 5 completed the lease design. Three parts work together.

**Heartbeat.** While a worker runs a job it renews the lease every third of
its length. A lease is therefore not a time limit on the job; it is the longest
a worker may go silent. A job can run for an hour on a two minute lease.

**Sweeper.** Every server process periodically looks for jobs in `processing`
whose lease has run out, which now can only mean the worker crashed or hung,
and puts them back to `queued`. The lost attempt counts, so a job that kills
every worker that touches it fails after `max_attempts` instead of circulating
forever.

**Fencing.** A worker that was only stalled, not dead, may wake up after its
job has been given to someone else. Checking `status = 'processing'` would not
stop it: the job is `processing` again, for the new worker. So every write a
worker makes also requires `attempts = <the attempt it claimed>`. The attempt
number changes with each claim, which makes it a fencing token: the stale
worker's writes match no row. Its heartbeat fails the same way, which is how it
finds out, and it then stops without writing anything or deleting the audio.

Retries with backoff were added at the same time: a failed attempt goes back
to `queued` with a later `run_at`, unless the provider marked the error as
permanent.

What remains true: a job may run more than once (the first worker may have
done real work before it stalled), so the queue is at-least-once. That is
acceptable here because a transcription has no side effects beyond its result,
and only one attempt's result can be recorded.
