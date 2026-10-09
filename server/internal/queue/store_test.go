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
	store := NewStore(testdb.New(t), NewBus())

	_, err := store.Claim(context.Background(), testLease)
	if !errors.Is(err, ErrNoJobs) {
		t.Errorf("Claim error = %v, want ErrNoJobs", err)
	}
}

func TestClaimMarksJobAsProcessing(t *testing.T) {
	pool := testdb.New(t)
	store := NewStore(pool, NewBus())
	ctx := context.Background()

	id, err := store.Enqueue(ctx, "en", "clip.wav")
	if err != nil {
		t.Fatalf("Enqueue: %v", err)
	}

	job, err := store.Claim(ctx, testLease)
	if err != nil {
		t.Fatalf("Claim: %v", err)
	}
	want := Job{ID: id, SourceLang: "en", AudioKey: "clip.wav", Attempt: 1, MaxAttempts: 3}
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
	store := NewStore(testdb.New(t), NewBus())
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
	store := NewStore(testdb.New(t), NewBus())
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
	store := NewStore(db, NewBus())
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
	store := NewStore(db, NewBus())
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
	store := NewStore(db, NewBus())
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
	store := NewStore(db, NewBus())
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
	store := NewStore(db, NewBus())
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

func TestStatus(t *testing.T) {
	store := NewStore(testdb.New(t), NewBus())
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

// Events reach the bus only after their transaction has committed.
func TestStorePublishesRecordedEvents(t *testing.T) {
	bus := NewBus()
	store := NewStore(testdb.New(t), bus)
	ctx := context.Background()

	id, err := store.Enqueue(ctx, "en", "clip.wav")
	if err != nil {
		t.Fatalf("Enqueue: %v", err)
	}
	live, unsubscribe := bus.Subscribe(id)
	defer unsubscribe()

	job, err := store.Claim(ctx, testLease)
	if err != nil {
		t.Fatalf("Claim: %v", err)
	}
	if err := store.Complete(ctx, job, "hello", "en"); err != nil {
		t.Fatalf("Complete: %v", err)
	}
	// A rejected change must publish nothing.
	if err := store.Complete(ctx, job, "again", "en"); !errors.Is(err, ErrNotProcessing) {
		t.Fatalf("second Complete error = %v, want ErrNotProcessing", err)
	}
	unsubscribe() // closes live, which ends the loop below

	var got []Event
	for e := range live {
		got = append(got, e)
	}
	if types := eventTypes(got); len(types) != 2 || types[0] != EventProcessing || types[1] != EventDone {
		t.Errorf("published %v, want [processing done]", types)
	}

	// What was published must be byte-for-byte what a replay returns.
	stored, err := store.Events(ctx, id, 1)
	if err != nil {
		t.Fatalf("Events: %v", err)
	}
	for i := range got {
		if string(got[i].Payload) != string(stored[i].Payload) {
			t.Errorf("event %d: published payload %s, stored payload %s", got[i].Seq, got[i].Payload, stored[i].Payload)
		}
	}
}

func eventTypes(events []Event) []string {
	types := make([]string, len(events))
	for i, e := range events {
		types[i] = e.Type
	}
	return types
}
