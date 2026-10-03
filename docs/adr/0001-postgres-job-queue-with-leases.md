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
- A worker that dies without releasing its job leaves it in `processing`
  until something notices the expired lease. That sweep, and retries with
  backoff, arrive in milestone 5. Until then a hard crash mid-job strands that
  job. A normal shutdown does not.
- Jobs are claimed in `run_at` order, which is roughly but not strictly first
  in, first out once several workers run.
