package queue

import (
	"context"
	"encoding/json"
	"errors"
	"slices"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/PelumiWeb/translator-app-go/server/internal/testdb"
)

const testLease = time.Minute

func TestClaimOnEmptyQueue(t *testing.T) {
	store := NewStore(testdb.New(t))

	_, err := store.Claim(context.Background(), testLease)
	if !errors.Is(err, ErrNoJobs) {
		t.Errorf("Claim error = %v, want ErrNoJobs", err)
	}
}

func TestClaimMarksJobAsProcessing(t *testing.T) {
	pool := testdb.New(t)
	store := NewStore(pool)
	ctx := context.Background()

	id, err := store.Enqueue(ctx, "en", "clip.wav")
	if err != nil {
		t.Fatalf("Enqueue: %v", err)
	}

	job, err := store.Claim(ctx, testLease)
	if err != nil {
		t.Fatalf("Claim: %v", err)
	}
	want := Job{ID: id, Kind: KindTranscribe, SourceLang: "en", AudioKey: "clip.wav", Attempt: 1, MaxAttempts: 3}
	if job != want {
		t.Errorf("job = %+v, want %+v", job, want)
	}

	var status string
	var leased bool
	err = pool.QueryRow(ctx, "SELECT status, locked_until > now() FROM jobs WHERE id = $1", id).Scan(&status, &leased)
	if err != nil {
		t.Fatalf("reading job: %v", err)
	}
	if status != "processing" || !leased {
		t.Errorf("status = %q, leased = %v; want processing, true", status, leased)
	}

	// The same job must not be handed out twice.
	if _, err := store.Claim(ctx, testLease); !errors.Is(err, ErrNoJobs) {
		t.Errorf("second Claim error = %v, want ErrNoJobs", err)
	}
}

// The property SKIP LOCKED exists for: many workers claiming at the same
// time each get a different job, and every job is handed out exactly once.
func TestClaimConcurrentWorkersNeverShareAJob(t *testing.T) {
	store := NewStore(testdb.New(t))
	ctx := context.Background()

	const jobs, workers = 60, 8
	for range jobs {
		if _, err := store.Enqueue(ctx, "en", "clip.wav"); err != nil {
			t.Fatalf("Enqueue: %v", err)
		}
	}

	var (
		mu      sync.Mutex // guards claimed: maps are not safe for concurrent writes
		claimed = map[string]int{}
		wg      sync.WaitGroup
	)
	for range workers {
		wg.Go(func() {
			for {
				job, err := store.Claim(ctx, testLease)
				if errors.Is(err, ErrNoJobs) {
					return
				}
				if err != nil {
					// t.Fatal may only be called from the test's own
					// goroutine; t.Error is safe from any.
					t.Errorf("Claim: %v", err)
					return
				}
				mu.Lock()
				claimed[job.ID]++
				mu.Unlock()
			}
		})
	}
	wg.Wait()

	if len(claimed) != jobs {
		t.Errorf("claimed %d distinct jobs, want %d", len(claimed), jobs)
	}
	for id, n := range claimed {
		if n != 1 {
			t.Errorf("job %s was claimed %d times", id, n)
		}
	}
}

func TestFinishRequiresProcessingJob(t *testing.T) {
	store := NewStore(testdb.New(t))
	ctx := context.Background()

	id, err := store.Enqueue(ctx, "en", "clip.wav")
	if err != nil {
		t.Fatalf("Enqueue: %v", err)
	}

	// Still queued: nobody claimed it, so nobody may complete it.
	if err := store.Complete(ctx, Job{ID: id, Attempt: 1}, "text", "en"); !errors.Is(err, ErrNotProcessing) {
		t.Errorf("Complete error = %v, want ErrNotProcessing", err)
	}

	// The failed Complete must not have left a "done" event behind.
	events, err := store.Events(ctx, id, 0)
	if err != nil {
		t.Fatalf("Events: %v", err)
	}
	if got := eventTypes(events); len(got) != 1 || got[0] != EventQueued {
		t.Errorf("events = %v, want [queued]", got)
	}
}

func TestRetryDelaysTheJob(t *testing.T) {
	db := testdb.New(t)
	store := NewStore(db)
	ctx := context.Background()

	id, err := store.Enqueue(ctx, "en", "clip.wav")
	if err != nil {
		t.Fatalf("Enqueue: %v", err)
	}
	claimed, err := store.Claim(ctx, testLease)
	if err != nil {
		t.Fatalf("Claim: %v", err)
	}

	if err := store.Retry(ctx, claimed, "provider timed out", time.Hour); err != nil {
		t.Fatalf("Retry: %v", err)
	}

	// Queued again, but not due for an hour: no worker may take it yet.
	if _, err := store.Claim(ctx, testLease); !errors.Is(err, ErrNoJobs) {
		t.Fatalf("Claim before the delay has passed: error = %v, want ErrNoJobs", err)
	}
	var status, lastError string
	var attempts int
	var locked bool
	err = db.QueryRow(ctx, "SELECT status, last_error, attempts, locked_until IS NOT NULL FROM jobs WHERE id = $1", id).
		Scan(&status, &lastError, &attempts, &locked)
	if err != nil {
		t.Fatalf("reading job: %v", err)
	}
	if status != StatusQueued || lastError != "provider timed out" || attempts != 1 || locked {
		t.Errorf("job = %s, %q, attempts %d, locked %v; want queued, the error, 1, false", status, lastError, attempts, locked)
	}

	// Make it due now, as if the hour had passed, and it is claimable as
	// attempt 2.
	if _, err := db.Exec(ctx, "UPDATE jobs SET run_at = now() WHERE id = $1", id); err != nil {
		t.Fatalf("moving run_at: %v", err)
	}
	job, err := store.Claim(ctx, testLease)
	if err != nil {
		t.Fatalf("Claim after the delay: %v", err)
	}
	if job.Attempt != 2 {
		t.Errorf("attempt = %d, want 2", job.Attempt)
	}

	events, err := store.Events(ctx, id, 0)
	if err != nil {
		t.Fatalf("Events: %v", err)
	}
	want := []string{"queued", "processing", "queued", "processing"}
	if got := eventTypes(events); !slices.Equal(got, want) {
		t.Errorf("event types = %v, want %v", got, want)
	}
	var retry struct {
		Reason    string `json:"reason"`
		Error     string `json:"error"`
		RetryInMs int64  `json:"retry_in_ms"`
	}
	if err := json.Unmarshal(events[2].Payload, &retry); err != nil {
		t.Fatalf("decoding retry event: %v", err)
	}
	if retry.Reason != "retry" || retry.Error != "provider timed out" || retry.RetryInMs != time.Hour.Milliseconds() {
		t.Errorf("retry event = %+v", retry)
	}
}

// expireLease makes a claimed job look as if its worker went silent long ago.
func expireLease(t *testing.T, db *pgxpool.Pool, id string) {
	t.Helper()
	_, err := db.Exec(context.Background(), "UPDATE jobs SET locked_until = now() - interval '1 second' WHERE id = $1", id)
	if err != nil {
		t.Fatalf("expiring lease: %v", err)
	}
}

func TestRequeueExpiredReturnsAbandonedJobsToTheQueue(t *testing.T) {
	db := testdb.New(t)
	store := NewStore(db)
	ctx := context.Background()

	abandoned, err := store.Enqueue(ctx, "en", "a.wav")
	if err != nil {
		t.Fatalf("Enqueue: %v", err)
	}
	if _, err := store.Claim(ctx, testLease); err != nil {
		t.Fatalf("Claim: %v", err)
	}
	expireLease(t, db, abandoned)

	// A second job whose worker is alive: its lease has not run out.
	healthy, err := store.Enqueue(ctx, "en", "b.wav")
	if err != nil {
		t.Fatalf("Enqueue: %v", err)
	}
	if _, err := store.Claim(ctx, testLease); err != nil {
		t.Fatalf("Claim: %v", err)
	}

	n, err := store.RequeueExpired(ctx)
	if err != nil {
		t.Fatalf("RequeueExpired: %v", err)
	}
	if n != 1 {
		t.Errorf("requeued %d jobs, want 1", n)
	}
	if status, _ := store.Status(ctx, healthy); status != StatusProcessing {
		t.Errorf("the healthy job is now %q, want it left processing", status)
	}

	// The abandoned job can be claimed again, as attempt 2.
	job, err := store.Claim(ctx, testLease)
	if err != nil {
		t.Fatalf("Claim after requeue: %v", err)
	}
	if job.ID != abandoned || job.Attempt != 2 {
		t.Errorf("claimed %s attempt %d, want %s attempt 2", job.ID, job.Attempt, abandoned)
	}

	events, err := store.Events(ctx, abandoned, 0)
	if err != nil {
		t.Fatalf("Events: %v", err)
	}
	want := []string{"queued", "processing", "queued", "processing"}
	if got := eventTypes(events); !slices.Equal(got, want) {
		t.Errorf("event types = %v, want %v", got, want)
	}
	if got := string(events[2].Payload); got != `{"reason": "lease_expired"}` {
		t.Errorf("requeue event payload = %s", got)
	}

	// Running it again finds nothing left to do.
	if n, err := store.RequeueExpired(ctx); err != nil || n != 0 {
		t.Errorf("second RequeueExpired = %d, %v; want 0, nil", n, err)
	}
}

// A job that keeps killing its workers must not be handed out forever.
func TestRequeueExpiredFailsAJobThatIsOutOfAttempts(t *testing.T) {
	db := testdb.New(t)
	store := NewStore(db)
	ctx := context.Background()

	id, err := store.Enqueue(ctx, "en", "poison.wav")
	if err != nil {
		t.Fatalf("Enqueue: %v", err)
	}
	// Three workers in a row take it and die: the default max_attempts.
	for attempt := 1; attempt <= 3; attempt++ {
		if _, err := store.Claim(ctx, testLease); err != nil {
			t.Fatalf("Claim %d: %v", attempt, err)
		}
		expireLease(t, db, id)
		if _, err := store.RequeueExpired(ctx); err != nil {
			t.Fatalf("RequeueExpired %d: %v", attempt, err)
		}
	}

	if status, _ := store.Status(ctx, id); status != StatusFailed {
		t.Errorf("status = %q, want failed", status)
	}
	if _, err := store.Claim(ctx, testLease); !errors.Is(err, ErrNoJobs) {
		t.Errorf("a failed job was handed out again: %v", err)
	}
	events, err := store.Events(ctx, id, 0)
	if err != nil {
		t.Fatalf("Events: %v", err)
	}
	if last := events[len(events)-1]; last.Type != EventError {
		t.Errorf("last event = %s, want error", last.Type)
	}
}

// The scenario the attempt number guards against: worker A stalls, the job
// is given to worker B, then A wakes up and tries to finish it.
func TestAStaleWorkerCannotTouchAJobThatWasTakenOver(t *testing.T) {
	db := testdb.New(t)
	store := NewStore(db)
	ctx := context.Background()

	id, err := store.Enqueue(ctx, "en", "clip.wav")
	if err != nil {
		t.Fatalf("Enqueue: %v", err)
	}
	workerA, err := store.Claim(ctx, testLease)
	if err != nil {
		t.Fatalf("Claim A: %v", err)
	}
	expireLease(t, db, id)
	if _, err := store.RequeueExpired(ctx); err != nil {
		t.Fatalf("RequeueExpired: %v", err)
	}
	workerB, err := store.Claim(ctx, testLease)
	if err != nil {
		t.Fatalf("Claim B: %v", err)
	}

	// Every way A could write is refused.
	stale := map[string]error{
		"Complete":    store.Complete(ctx, workerA, "A's stale transcript", "en"),
		"Fail":        store.Fail(ctx, workerA, "A gave up"),
		"Retry":       store.Retry(ctx, workerA, "A wants a retry", time.Second),
		"Release":     store.Release(ctx, workerA),
		"ExtendLease": store.ExtendLease(ctx, workerA, testLease),
	}
	for name, err := range stale {
		if !errors.Is(err, ErrNotProcessing) {
			t.Errorf("stale worker's %s: error = %v, want ErrNotProcessing", name, err)
		}
	}

	// B is unaffected and finishes normally.
	if err := store.ExtendLease(ctx, workerB, testLease); err != nil {
		t.Errorf("B's ExtendLease: %v", err)
	}
	if err := store.Complete(ctx, workerB, "B's transcript", "en"); err != nil {
		t.Fatalf("B's Complete: %v", err)
	}
	var result string
	if err := db.QueryRow(ctx, "SELECT result_text FROM jobs WHERE id = $1", id).Scan(&result); err != nil {
		t.Fatalf("reading result: %v", err)
	}
	if result != "B's transcript" {
		t.Errorf("result = %q, want B's", result)
	}
}

func TestExtendLeasePushesTheExpiryOut(t *testing.T) {
	db := testdb.New(t)
	store := NewStore(db)
	ctx := context.Background()

	id, err := store.Enqueue(ctx, "en", "clip.wav")
	if err != nil {
		t.Fatalf("Enqueue: %v", err)
	}
	job, err := store.Claim(ctx, time.Second)
	if err != nil {
		t.Fatalf("Claim: %v", err)
	}

	if err := store.ExtendLease(ctx, job, time.Hour); err != nil {
		t.Fatalf("ExtendLease: %v", err)
	}

	var farOff bool
	err = db.QueryRow(ctx, "SELECT locked_until > now() + interval '59 minutes' FROM jobs WHERE id = $1", id).Scan(&farOff)
	if err != nil {
		t.Fatalf("reading lease: %v", err)
	}
	if !farOff {
		t.Error("the lease was not extended to about an hour from now")
	}
}

func TestEnqueueOnceCreatesOneJobPerKey(t *testing.T) {
	db := testdb.New(t)
	store := NewStore(db)
	ctx := context.Background()

	first, created, err := store.EnqueueOnce(ctx, "en", "first.wav", "key-1")
	if err != nil || !created {
		t.Fatalf("first EnqueueOnce = %q, %v, %v; want a new job", first, created, err)
	}
	repeat, created, err := store.EnqueueOnce(ctx, "en", "second-copy.wav", "key-1")
	if err != nil {
		t.Fatalf("repeat EnqueueOnce: %v", err)
	}
	if created || repeat != first {
		t.Errorf("repeat = %q, created %v; want the first job %q and created false", repeat, created, first)
	}

	other, created, err := store.EnqueueOnce(ctx, "en", "other.wav", "key-2")
	if err != nil || !created || other == first {
		t.Errorf("a different key = %q, %v, %v; want a new, different job", other, created, err)
	}

	// The repeat must not have added a second "queued" event to the job.
	events, err := store.Events(ctx, first, 0)
	if err != nil {
		t.Fatalf("Events: %v", err)
	}
	if got := eventTypes(events); !slices.Equal(got, []string{"queued"}) {
		t.Errorf("events = %v, want [queued]", got)
	}
	// And the job keeps the audio it was created with.
	var audioPath string
	if err := db.QueryRow(ctx, "SELECT audio_path FROM jobs WHERE id = $1", first).Scan(&audioPath); err != nil {
		t.Fatalf("reading job: %v", err)
	}
	if audioPath != "first.wav" {
		t.Errorf("audio_path = %q, want first.wav", audioPath)
	}
}

func TestEnqueueOnceWithoutAKeyAlwaysCreates(t *testing.T) {
	store := NewStore(testdb.New(t))
	ctx := context.Background()

	a, createdA, errA := store.EnqueueOnce(ctx, "en", "a.wav", "")
	b, createdB, errB := store.EnqueueOnce(ctx, "en", "b.wav", "")

	if errA != nil || errB != nil || !createdA || !createdB || a == b {
		t.Errorf("got %q (%v, %v) and %q (%v, %v); want two different new jobs", a, createdA, errA, b, createdB, errB)
	}
}

// Two attempts with one key arriving at the same instant: the unique index,
// not luck, must leave exactly one job.
func TestEnqueueOnceUnderConcurrencyCreatesOneJob(t *testing.T) {
	db := testdb.New(t)
	store := NewStore(db)
	ctx := context.Background()

	const attempts = 16
	ids := make([]string, attempts)
	createdCount := 0
	var mu sync.Mutex
	var wg sync.WaitGroup
	for i := range attempts {
		wg.Go(func() {
			id, created, err := store.EnqueueOnce(ctx, "en", "clip.wav", "same-key")
			if err != nil {
				t.Errorf("EnqueueOnce: %v", err)
				return
			}
			mu.Lock()
			defer mu.Unlock()
			ids[i] = id
			if created {
				createdCount++
			}
		})
	}
	wg.Wait()

	if createdCount != 1 {
		t.Errorf("%d attempts reported creating the job, want exactly 1", createdCount)
	}
	for _, id := range ids {
		if id != ids[0] {
			t.Fatalf("attempts got different jobs: %v", ids)
		}
	}
	var rows int
	if err := db.QueryRow(ctx, "SELECT count(*) FROM jobs").Scan(&rows); err != nil {
		t.Fatalf("counting jobs: %v", err)
	}
	if rows != 1 {
		t.Errorf("%d jobs in the table, want 1", rows)
	}
}

func TestSynthesisJobIsClaimedWithItsDetails(t *testing.T) {
	store := NewStore(testdb.New(t))
	ctx := context.Background()

	id, created, err := store.EnqueueSynthesis(ctx, "fr", "bonjour tout le monde", "voice.wav", "key-1")
	if err != nil || !created {
		t.Fatalf("EnqueueSynthesis = %q, %v, %v", id, created, err)
	}
	// The idempotency key works for this kind of job too.
	if repeat, created, err := store.EnqueueSynthesis(ctx, "fr", "bonjour tout le monde", "copy.wav", "key-1"); err != nil || created || repeat != id {
		t.Errorf("repeat = %q, %v, %v; want the same job and created false", repeat, created, err)
	}

	job, err := store.Claim(ctx, testLease)
	if err != nil {
		t.Fatalf("Claim: %v", err)
	}
	want := Job{
		ID: id, Kind: KindSynthesize, AudioKey: "voice.wav",
		TargetLang: "fr", Text: "bonjour tout le monde",
		Attempt: 1, MaxAttempts: 3,
	}
	if job != want {
		t.Errorf("job = %+v\nwant  %+v", job, want)
	}

	// Not ready while it is being worked on.
	if _, err := store.ResultAudio(ctx, id); !errors.Is(err, ErrNotReady) {
		t.Errorf("ResultAudio before completion: error = %v, want ErrNotReady", err)
	}
	if err := store.CompleteSynthesis(ctx, job, "speech.wav"); err != nil {
		t.Fatalf("CompleteSynthesis: %v", err)
	}
	if key, err := store.ResultAudio(ctx, id); err != nil || key != "speech.wav" {
		t.Errorf("ResultAudio = %q, %v; want speech.wav", key, err)
	}

	const unknown = "00000000-0000-0000-0000-000000000000"
	if _, err := store.ResultAudio(ctx, unknown); !errors.Is(err, ErrJobNotFound) {
		t.Errorf("ResultAudio of an unknown job: error = %v, want ErrJobNotFound", err)
	}
}

func TestTakeExpiredResultsReturnsOnlyOldAudioAndOnlyOnce(t *testing.T) {
	db := testdb.New(t)
	store := NewStore(db)
	ctx := context.Background()

	finish := func(resultKey string) string {
		t.Helper()
		id, _, err := store.EnqueueSynthesis(ctx, "es", "hola", "voice.wav", "")
		if err != nil {
			t.Fatalf("EnqueueSynthesis: %v", err)
		}
		job, err := store.Claim(ctx, testLease)
		if err != nil {
			t.Fatalf("Claim: %v", err)
		}
		if err := store.CompleteSynthesis(ctx, job, resultKey); err != nil {
			t.Fatalf("CompleteSynthesis: %v", err)
		}
		return id
	}
	old := finish("old.wav")
	finish("fresh.wav")
	// Make the first one look as if it finished two hours ago.
	if _, err := db.Exec(ctx, "UPDATE jobs SET updated_at = now() - interval '2 hours' WHERE id = $1", old); err != nil {
		t.Fatalf("ageing the job: %v", err)
	}

	keys, err := store.TakeExpiredResults(ctx, time.Hour)
	if err != nil {
		t.Fatalf("TakeExpiredResults: %v", err)
	}
	if !slices.Equal(keys, []string{"old.wav"}) {
		t.Errorf("keys = %v, want [old.wav]", keys)
	}
	if _, err := store.ResultAudio(ctx, old); !errors.Is(err, ErrNotReady) {
		t.Errorf("the expired result is still on offer: %v", err)
	}
	// A second sweep must not hand the same file out for deletion again.
	if again, err := store.TakeExpiredResults(ctx, time.Hour); err != nil || len(again) != 0 {
		t.Errorf("second call = %v, %v; want nothing", again, err)
	}
}

func TestStatus(t *testing.T) {
	store := NewStore(testdb.New(t))
	ctx := context.Background()

	id, err := store.Enqueue(ctx, "en", "clip.wav")
	if err != nil {
		t.Fatalf("Enqueue: %v", err)
	}
	if status, err := store.Status(ctx, id); err != nil || status != StatusQueued {
		t.Errorf("Status = %q, %v; want queued, nil", status, err)
	}

	const unknown = "00000000-0000-0000-0000-000000000000"
	if _, err := store.Status(ctx, unknown); !errors.Is(err, ErrJobNotFound) {
		t.Errorf("Status of unknown job: error = %v, want ErrJobNotFound", err)
	}
}

func eventTypes(events []Event) []string {
	types := make([]string, len(events))
	for i, e := range events {
		types[i] = e.Type
	}
	return types
}
