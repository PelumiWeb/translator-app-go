// Package queue is the job queue: a Postgres-backed store and the worker
// pool that drains it.
package queue

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Sentinel errors: callers compare with errors.Is instead of matching text.
var (
	ErrNoJobs        = errors.New("queue: no job is ready")
	ErrNotProcessing = errors.New("queue: job is not being processed")
	ErrJobNotFound   = errors.New("queue: job not found")
)

// Job statuses, as stored in jobs.status.
const (
	StatusQueued     = "queued"
	StatusProcessing = "processing"
	StatusDone       = "done"
	StatusFailed     = "failed"
)

// Event types, also used as the SSE event names.
const (
	EventQueued     = "queued"
	EventProcessing = "processing"
	EventPartial    = "partial"
	EventDone       = "done"
	EventError      = "error"
)

// Job is what a worker needs to process one claimed job.
type Job struct {
	ID         string
	SourceLang string
	AudioKey   string
	Attempt    int // 1 on the first try
}

// Event is one entry in a job's history. Seq counts 1, 2, 3 ... per job.
type Event struct {
	JobID   string
	Seq     int
	Type    string
	Payload json.RawMessage
}

// Publisher is told about each event once it is safely in the database.
type Publisher interface {
	Publish(Event)
}

type Store struct {
	pool   *pgxpool.Pool
	events Publisher
}

// NewStore returns a store that announces every event it records on events.
//
// The rule throughout is "commit, then publish". An event is never announced
// before its transaction has committed, so a listener can never hear about
// something that is then rolled back.
func NewStore(pool *pgxpool.Pool, events Publisher) *Store {
	return &Store{pool: pool, events: events}
}

// Enqueue adds a job and its first event, and returns the job id.
func (s *Store) Enqueue(ctx context.Context, sourceLang, audioKey string) (string, error) {
	var id string
	var event Event
	// BeginFunc commits if the function returns nil and rolls back if it
	// returns an error, so the job and its "queued" event exist together or
	// not at all.
	err := pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		err := tx.QueryRow(ctx, `
			INSERT INTO jobs (status, source_lang, audio_path)
			VALUES ('queued', $1, $2)
			RETURNING id`,
			sourceLang, audioKey,
		).Scan(&id)
		if err != nil {
			return err
		}
		event, err = insertEvent(ctx, tx, id, EventQueued, struct{}{})
		return err
	})
	if err != nil {
		return "", fmt.Errorf("enqueueing job: %w", err)
	}
	s.events.Publish(event)
	return id, nil
}

// Claim takes the oldest ready job for the caller, or returns ErrNoJobs.
//
// FOR UPDATE SKIP LOCKED is what makes this safe with many workers: a row
// another worker is claiming right now is skipped instead of waited for, so
// two workers never get the same job and never queue up behind each other.
//
// The transaction is short. After it commits, the job is protected by its
// status and the locked_until lease, not by a row lock, so no database
// connection is held while the provider runs.
func (s *Store) Claim(ctx context.Context, lease time.Duration) (Job, error) {
	var job Job
	var event Event
	err := pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		err := tx.QueryRow(ctx, `
			UPDATE jobs
			SET status = 'processing',
			    attempts = attempts + 1,
			    locked_until = now() + $1::interval,
			    updated_at = now()
			WHERE id = (
				SELECT id FROM jobs
				WHERE status = 'queued' AND run_at <= now()
				ORDER BY run_at
				FOR UPDATE SKIP LOCKED
				LIMIT 1
			)
			RETURNING id, source_lang, audio_path, attempts`,
			lease,
		).Scan(&job.ID, &job.SourceLang, &job.AudioKey, &job.Attempt)
		if err != nil {
			return err
		}
		event, err = insertEvent(ctx, tx, job.ID, EventProcessing, map[string]int{"attempt": job.Attempt})
		return err
	})
	// UPDATE ... RETURNING that matched nothing yields no row.
	if errors.Is(err, pgx.ErrNoRows) {
		return Job{}, ErrNoJobs
	}
	if err != nil {
		return Job{}, fmt.Errorf("claiming job: %w", err)
	}
	s.events.Publish(event)
	return job, nil
}

// AppendEvent records an event for a job, for example a partial transcript.
func (s *Store) AppendEvent(ctx context.Context, jobID, eventType string, payload any) (Event, error) {
	event, err := insertEvent(ctx, s.pool, jobID, eventType, payload)
	if err != nil {
		return Event{}, fmt.Errorf("appending %s event: %w", eventType, err)
	}
	s.events.Publish(event)
	return event, nil
}

// Complete marks a processing job as done and stores its transcript.
func (s *Store) Complete(ctx context.Context, jobID, text, language string) error {
	return s.finish(ctx, jobID, `
		UPDATE jobs
		SET status = 'done', result_text = $2, locked_until = NULL, updated_at = now()
		WHERE id = $1 AND status = 'processing'`,
		[]any{jobID, text},
		EventDone, map[string]string{"text": text, "language": language},
	)
}

// Fail marks a processing job as failed for good.
func (s *Store) Fail(ctx context.Context, jobID, message string) error {
	return s.finish(ctx, jobID, `
		UPDATE jobs
		SET status = 'failed', last_error = $2, locked_until = NULL, updated_at = now()
		WHERE id = $1 AND status = 'processing'`,
		[]any{jobID, message},
		EventError, map[string]string{"message": message},
	)
}

// Release puts a processing job back in the queue without counting the
// attempt. Used when the server shuts down in the middle of a job.
func (s *Store) Release(ctx context.Context, jobID string) error {
	return s.finish(ctx, jobID, `
		UPDATE jobs
		SET status = 'queued', attempts = attempts - 1, locked_until = NULL, updated_at = now()
		WHERE id = $1 AND status = 'processing'`,
		[]any{jobID},
		EventQueued, map[string]string{"reason": "requeued"},
	)
}

// finish runs a status change and records its event in one transaction.
func (s *Store) finish(ctx context.Context, jobID, update string, args []any, eventType string, payload any) error {
	var event Event
	err := pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		tag, err := tx.Exec(ctx, update, args...)
		if err != nil {
			return err
		}
		// The WHERE clause requires status = 'processing'. No row changed
		// means this worker no longer owns the job.
		if tag.RowsAffected() == 0 {
			return ErrNotProcessing
		}
		event, err = insertEvent(ctx, tx, jobID, eventType, payload)
		return err
	})
	if err != nil {
		return fmt.Errorf("recording %s for job %s: %w", eventType, jobID, err)
	}
	s.events.Publish(event)
	return nil
}

// Status returns a job's current status, or ErrJobNotFound.
func (s *Store) Status(ctx context.Context, jobID string) (string, error) {
	var status string
	err := s.pool.QueryRow(ctx, "SELECT status FROM jobs WHERE id = $1", jobID).Scan(&status)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", ErrJobNotFound
	}
	if err != nil {
		return "", fmt.Errorf("reading job status: %w", err)
	}
	return status, nil
}

// Events returns a job's events with seq greater than afterSeq, in order.
// Pass 0 for the full history.
func (s *Store) Events(ctx context.Context, jobID string, afterSeq int) ([]Event, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT job_id, seq, type, payload
		FROM job_events
		WHERE job_id = $1 AND seq > $2
		ORDER BY seq`,
		jobID, afterSeq,
	)
	if err != nil {
		return nil, fmt.Errorf("reading events: %w", err)
	}
	// RowToStructByPos fills Event's fields in the order of the columns.
	events, err := pgx.CollectRows(rows, pgx.RowToStructByPos[Event])
	if err != nil {
		return nil, fmt.Errorf("reading events: %w", err)
	}
	return events, nil
}

// querier is satisfied by both *pgxpool.Pool and pgx.Tx, so insertEvent can
// run on its own or as part of a larger transaction.
type querier interface {
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

func insertEvent(ctx context.Context, q querier, jobID, eventType string, payload any) (Event, error) {
	body, err := json.Marshal(payload)
	if err != nil {
		return Event{}, fmt.Errorf("encoding payload: %w", err)
	}

	event := Event{JobID: jobID, Type: eventType}
	// The next seq is computed in the same statement as the insert. Only
	// one worker owns a job at a time, so two writers do not race for the
	// same number; if they ever did, the primary key would reject one.
	//
	// The payload is read back because jsonb storage reformats JSON (spacing
	// and key order). Publishing the stored form means a client sees the
	// same bytes whether an event reaches it live or from a replay.
	err = q.QueryRow(ctx, `
		INSERT INTO job_events (job_id, seq, type, payload)
		SELECT $1, COALESCE(MAX(seq), 0) + 1, $2, $3
		FROM job_events
		WHERE job_id = $1
		RETURNING seq, payload`,
		jobID, eventType, body,
	).Scan(&event.Seq, &event.Payload)
	if err != nil {
		return Event{}, err
	}
	return event, nil
}
