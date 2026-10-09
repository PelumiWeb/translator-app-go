package queue

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log/slog"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/PelumiWeb/translator-app-go/server/internal/provider"
	"github.com/PelumiWeb/translator-app-go/server/internal/provider/fake"
	"github.com/PelumiWeb/translator-app-go/server/internal/testdb"
)

// providerFunc lets a test write a Provider as a plain function, the same
// trick http.HandlerFunc uses for handlers.
type providerFunc func(ctx context.Context, onPartial func(string)) (provider.Result, error)

func (f providerFunc) Transcribe(ctx context.Context, _ io.Reader, _ provider.Options, onPartial func(string)) (provider.Result, error) {
	return f(ctx, onPartial)
}

// fakeAudio hands out empty audio and remembers which keys were deleted.
type fakeAudio struct {
	mu      sync.Mutex
	deleted []string
}

func (a *fakeAudio) Open(string) (io.ReadCloser, error) {
	return io.NopCloser(strings.NewReader("")), nil
}

func (a *fakeAudio) Delete(key string) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.deleted = append(a.deleted, key)
	return nil
}

func (a *fakeAudio) deletedKeys() []string {
	a.mu.Lock()
	defer a.mu.Unlock()
	return slices.Clone(a.deleted)
}

// startPool runs a pool in the background and returns a function that stops
// it and waits for Run to return.
func startPool(t *testing.T, store *Store, p provider.Provider, audio AudioStore, grace time.Duration) (stop func()) {
	t.Helper()
	return startPoolWith(t, store, p, audio, Config{Lease: testLease, ShutdownGrace: grace})
}

// startPoolWith is startPool for tests that need their own lease or sweep
// timing. It fills in the settings every test shares.
func startPoolWith(t *testing.T, store *Store, p provider.Provider, audio AudioStore, cfg Config) (stop func()) {
	t.Helper()
	cfg.Workers = 2
	cfg.PollInterval = 10 * time.Millisecond
	// Retries almost at once, so tests of retrying finish in milliseconds.
	cfg.Backoff = func(int) time.Duration { return 5 * time.Millisecond }
	pool := NewPool(store, p, audio, slog.New(slog.NewTextHandler(io.Discard, nil)), cfg)

	ctx, cancel := context.WithCancel(context.Background())
	stopped := make(chan struct{})
	go func() {
		pool.Run(ctx)
		close(stopped)
	}()

	stop = func() {
		cancel()
		select {
		case <-stopped:
		case <-time.After(5 * time.Second):
			t.Fatal("pool did not stop within 5s")
		}
	}
	// If the test fails before calling stop, still shut the pool down.
	// Calling stop twice is harmless.
	t.Cleanup(stop)
	return stop
}

type jobRow struct {
	Status     string
	Attempts   int
	Locked     bool
	ResultText *string // pointer because the column is NULL until the job is done
	LastError  *string
}

func readJob(t *testing.T, db *pgxpool.Pool, id string) jobRow {
	t.Helper()
	var row jobRow
	err := db.QueryRow(context.Background(), `
		SELECT status, attempts, locked_until IS NOT NULL, result_text, last_error
		FROM jobs WHERE id = $1`, id,
	).Scan(&row.Status, &row.Attempts, &row.Locked, &row.ResultText, &row.LastError)
	if err != nil {
		t.Fatalf("reading job: %v", err)
	}
	return row
}

// waitForStatus polls instead of sleeping a fixed time, so the test is as
// fast as the code and does not flake on a slow machine.
func waitForStatus(t *testing.T, db *pgxpool.Pool, id, want string) jobRow {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for {
		row := readJob(t, db, id)
		if row.Status == want {
			return row
		}
		if time.Now().After(deadline) {
			t.Fatalf("job status = %q after 5s, want %q", row.Status, want)
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func TestPoolProcessesJobToDone(t *testing.T) {
	db := testdb.New(t)
	store := NewStore(db, NewBus())
	audio := &fakeAudio{}
	ctx := context.Background()

	id, err := store.Enqueue(ctx, "en", "clip.wav")
	if err != nil {
		t.Fatalf("Enqueue: %v", err)
	}
	startPool(t, store, fake.Provider{Text: "hello brave world"}, audio, time.Second)

	row := waitForStatus(t, db, id, "done")
	if row.ResultText == nil || *row.ResultText != "hello brave world" {
		t.Errorf("result_text = %v, want %q", row.ResultText, "hello brave world")
	}
	if row.Locked {
		t.Error("locked_until is still set on a finished job")
	}

	events, err := store.Events(ctx, id, 0)
	if err != nil {
		t.Fatalf("Events: %v", err)
	}
	wantTypes := []string{"queued", "processing", "partial", "partial", "partial", "done"}
	if got := eventTypes(events); !slices.Equal(got, wantTypes) {
		t.Errorf("event types = %v, want %v", got, wantTypes)
	}
	for i, e := range events {
		if e.Seq != i+1 {
			t.Errorf("event %d has seq %d, want %d", i, e.Seq, i+1)
		}
	}
	// Decode rather than compare strings: Postgres reformats stored JSON.
	var done struct{ Text, Language string }
	if err := json.Unmarshal(events[len(events)-1].Payload, &done); err != nil {
		t.Fatalf("decoding done payload: %v", err)
	}
	if done.Text != "hello brave world" || done.Language != "en" {
		t.Errorf("done payload = %+v", done)
	}

	if got := audio.deletedKeys(); !slices.Equal(got, []string{"clip.wav"}) {
		t.Errorf("deleted audio = %v, want [clip.wav]", got)
	}
}

// The fake provider fails the first two attempts, as a provider having a bad
// minute would, and the job still ends up done.
func TestPoolRetriesTemporaryFailures(t *testing.T) {
	db := testdb.New(t)
	store := NewStore(db, NewBus())
	audio := &fakeAudio{}
	ctx := context.Background()

	id, err := store.Enqueue(ctx, "en", "clip.wav")
	if err != nil {
		t.Fatalf("Enqueue: %v", err)
	}
	startPool(t, store, fake.Provider{Text: "third time lucky", FailAttempts: 2}, audio, time.Second)

	row := waitForStatus(t, db, id, "done")
	if row.Attempts != 3 {
		t.Errorf("attempts = %d, want 3", row.Attempts)
	}
	if row.ResultText == nil || *row.ResultText != "third time lucky" {
		t.Errorf("result_text = %v", row.ResultText)
	}

	events, err := store.Events(ctx, id, 0)
	if err != nil {
		t.Fatalf("Events: %v", err)
	}
	wantTypes := []string{
		"queued",
		"processing", "queued", // attempt 1 fails, retry scheduled
		"processing", "queued", // attempt 2 fails, retry scheduled
		"processing", "partial", "partial", "partial", "done",
	}
	if got := eventTypes(events); !slices.Equal(got, wantTypes) {
		t.Errorf("event types = %v, want %v", got, wantTypes)
	}

	// The audio has to survive the failed attempts and go only at the end.
	if got := audio.deletedKeys(); !slices.Equal(got, []string{"clip.wav"}) {
		t.Errorf("deleted audio = %v, want [clip.wav] exactly once", got)
	}
}

func TestPoolGivesUpAfterMaxAttempts(t *testing.T) {
	db := testdb.New(t)
	store := NewStore(db, NewBus())
	audio := &fakeAudio{}
	ctx := context.Background()

	id, err := store.Enqueue(ctx, "en", "clip.wav")
	if err != nil {
		t.Fatalf("Enqueue: %v", err)
	}
	startPool(t, store, fake.Provider{Err: errors.New("provider exploded")}, audio, time.Second)

	row := waitForStatus(t, db, id, "failed")
	if row.Attempts != 3 {
		t.Errorf("attempts = %d, want 3 (the default max_attempts)", row.Attempts)
	}
	if row.LastError == nil || *row.LastError != "provider exploded" {
		t.Errorf("last_error = %v, want %q", row.LastError, "provider exploded")
	}

	events, err := store.Events(ctx, id, 0)
	if err != nil {
		t.Fatalf("Events: %v", err)
	}
	wantTypes := []string{"queued", "processing", "queued", "processing", "queued", "processing", "error"}
	if got := eventTypes(events); !slices.Equal(got, wantTypes) {
		t.Errorf("event types = %v, want %v", got, wantTypes)
	}
	if got := audio.deletedKeys(); !slices.Equal(got, []string{"clip.wav"}) {
		t.Errorf("deleted audio = %v, want [clip.wav]", got)
	}
}

func TestPoolDoesNotRetryPermanentFailures(t *testing.T) {
	db := testdb.New(t)
	store := NewStore(db, NewBus())
	ctx := context.Background()

	id, err := store.Enqueue(ctx, "en", "clip.wav")
	if err != nil {
		t.Fatalf("Enqueue: %v", err)
	}
	bad := fake.Provider{Err: provider.Permanent(errors.New("audio is not speech"))}
	startPool(t, store, bad, &fakeAudio{}, time.Second)

	row := waitForStatus(t, db, id, "failed")
	if row.Attempts != 1 {
		t.Errorf("attempts = %d, want 1: a permanent failure must not be retried", row.Attempts)
	}
	if row.LastError == nil || *row.LastError != "audio is not speech" {
		t.Errorf("last_error = %v", row.LastError)
	}

	events, err := store.Events(ctx, id, 0)
	if err != nil {
		t.Fatalf("Events: %v", err)
	}
	wantTypes := []string{"queued", "processing", "error"}
	if got := eventTypes(events); !slices.Equal(got, wantTypes) {
		t.Errorf("event types = %v, want %v", got, wantTypes)
	}
}

// Audio that cannot be opened will not appear on a later attempt either.
func TestPoolDoesNotRetryWhenTheAudioIsMissing(t *testing.T) {
	db := testdb.New(t)
	store := NewStore(db, NewBus())

	id, err := store.Enqueue(context.Background(), "en", "gone.wav")
	if err != nil {
		t.Fatalf("Enqueue: %v", err)
	}
	startPool(t, store, fake.Provider{Text: "never reached"}, missingAudio{}, time.Second)

	row := waitForStatus(t, db, id, "failed")
	if row.Attempts != 1 {
		t.Errorf("attempts = %d, want 1", row.Attempts)
	}
}

type missingAudio struct{}

func (missingAudio) Open(key string) (io.ReadCloser, error) {
	return nil, fmt.Errorf("opening blob %s: %w", key, fs.ErrNotExist)
}
func (missingAudio) Delete(string) error { return nil }

// A job that finishes inside the grace period is not interrupted.
func TestPoolShutdownLetsRunningJobFinish(t *testing.T) {
	db := testdb.New(t)
	store := NewStore(db, NewBus())

	started := make(chan struct{})
	finish := make(chan struct{})
	slow := providerFunc(func(ctx context.Context, _ func(string)) (provider.Result, error) {
		close(started)
		select {
		case <-finish:
			return provider.Result{Text: "made it", Language: "en"}, nil
		case <-ctx.Done():
			return provider.Result{}, ctx.Err()
		}
	})

	id, err := store.Enqueue(context.Background(), "en", "clip.wav")
	if err != nil {
		t.Fatalf("Enqueue: %v", err)
	}
	stop := startPool(t, store, slow, &fakeAudio{}, 5*time.Second)

	<-started
	// Let the job finish shortly after shutdown has begun.
	time.AfterFunc(50*time.Millisecond, func() { close(finish) })
	stop()

	if row := readJob(t, db, id); row.Status != "done" {
		t.Errorf("status = %q, want done", row.Status)
	}
}

// A job still running when the grace period ends goes back to the queue
// untouched, ready for the next server start.
func TestPoolShutdownReleasesJobAfterGrace(t *testing.T) {
	db := testdb.New(t)
	store := NewStore(db, NewBus())
	audio := &fakeAudio{}

	started := make(chan struct{})
	stuck := providerFunc(func(ctx context.Context, _ func(string)) (provider.Result, error) {
		close(started)
		<-ctx.Done() // only stops when its context is cancelled
		return provider.Result{}, ctx.Err()
	})

	id, err := store.Enqueue(context.Background(), "en", "clip.wav")
	if err != nil {
		t.Fatalf("Enqueue: %v", err)
	}
	stop := startPool(t, store, stuck, audio, 50*time.Millisecond)

	<-started
	stop()

	row := readJob(t, db, id)
	if row.Status != "queued" || row.Attempts != 0 || row.Locked {
		t.Errorf("job = %+v; want status queued, attempts 0, not locked", row)
	}
	if got := audio.deletedKeys(); len(got) != 0 {
		t.Errorf("deleted audio = %v; a released job must keep its audio", got)
	}

	events, err := store.Events(context.Background(), id, 0)
	if err != nil {
		t.Fatalf("Events: %v", err)
	}
	wantTypes := []string{"queued", "processing", "queued"}
	if got := eventTypes(events); !slices.Equal(got, wantTypes) {
		t.Errorf("event types = %v, want %v", got, wantTypes)
	}
}

// A worker claims a job and is never heard from again. Here that worker is
// the test itself: it claims and then does nothing.
func TestPoolRecoversAJobFromAWorkerThatDied(t *testing.T) {
	db := testdb.New(t)
	store := NewStore(db, NewBus())
	ctx := context.Background()

	id, err := store.Enqueue(ctx, "en", "clip.wav")
	if err != nil {
		t.Fatalf("Enqueue: %v", err)
	}
	if _, err := store.Claim(ctx, 50*time.Millisecond); err != nil {
		t.Fatalf("Claim by the worker that will die: %v", err)
	}

	startPoolWith(t, store, fake.Provider{Text: "rescued"}, &fakeAudio{}, Config{
		Lease:         testLease,
		ShutdownGrace: time.Second,
		SweepInterval: 20 * time.Millisecond,
	})

	row := waitForStatus(t, db, id, "done")
	if row.Attempts != 2 {
		t.Errorf("attempts = %d, want 2: the lost attempt and the rescue", row.Attempts)
	}
	if row.ResultText == nil || *row.ResultText != "rescued" {
		t.Errorf("result_text = %v", row.ResultText)
	}
}

// A job that takes several times longer than the lease must still run once:
// the heartbeat keeps the lease alive, so the sweeper never sees it as dead.
func TestPoolKeepsTheLeaseOfALongJobAlive(t *testing.T) {
	db := testdb.New(t)
	store := NewStore(db, NewBus())

	var calls atomic.Int32
	slow := providerFunc(func(ctx context.Context, _ func(string)) (provider.Result, error) {
		calls.Add(1)
		select {
		case <-time.After(600 * time.Millisecond): // four leases long
			return provider.Result{Text: "slow but steady", Language: "en"}, nil
		case <-ctx.Done():
			return provider.Result{}, ctx.Err()
		}
	})

	id, err := store.Enqueue(context.Background(), "en", "clip.wav")
	if err != nil {
		t.Fatalf("Enqueue: %v", err)
	}
	startPoolWith(t, store, slow, &fakeAudio{}, Config{
		Lease:         150 * time.Millisecond,
		ShutdownGrace: time.Second,
		SweepInterval: 20 * time.Millisecond,
	})

	row := waitForStatus(t, db, id, "done")
	if row.Attempts != 1 {
		t.Errorf("attempts = %d, want 1: the job was taken away from a live worker", row.Attempts)
	}
	if got := calls.Load(); got != 1 {
		t.Errorf("the provider ran %d times, want 1", got)
	}
}

// If the job has been given to someone else, the worker stops and leaves it
// alone: no status change, no event, and the audio stays for the new owner.
func TestPoolAbandonsAJobItNoLongerOwns(t *testing.T) {
	db := testdb.New(t)
	store := NewStore(db, NewBus())
	audio := &fakeAudio{}
	ctx := context.Background()

	started := make(chan struct{})
	cancelled := make(chan struct{})
	stuck := providerFunc(func(ctx context.Context, _ func(string)) (provider.Result, error) {
		close(started)
		<-ctx.Done()
		close(cancelled)
		return provider.Result{}, ctx.Err()
	})

	id, err := store.Enqueue(ctx, "en", "clip.wav")
	if err != nil {
		t.Fatalf("Enqueue: %v", err)
	}
	startPoolWith(t, store, stuck, audio, Config{
		Lease:         90 * time.Millisecond, // heartbeat every 30 ms
		ShutdownGrace: time.Second,
		SweepInterval: time.Hour, // keep the sweeper out of this test
	})
	<-started

	// Another worker takes the job over: same effect as an expired lease
	// followed by a new claim, done directly so the pool cannot claim it.
	if _, err := db.Exec(ctx, "UPDATE jobs SET attempts = attempts + 1 WHERE id = $1", id); err != nil {
		t.Fatalf("simulating a takeover: %v", err)
	}

	select {
	case <-cancelled:
	case <-time.After(2 * time.Second):
		t.Fatal("the worker kept running a job it no longer owns")
	}
	time.Sleep(50 * time.Millisecond) // time for a wrong write to happen, if there is one

	row := readJob(t, db, id)
	if row.Status != "processing" || row.Attempts != 2 {
		t.Errorf("job = %+v; want it untouched: processing, attempts 2", row)
	}
	events, err := store.Events(ctx, id, 0)
	if err != nil {
		t.Fatalf("Events: %v", err)
	}
	if got := eventTypes(events); !slices.Equal(got, []string{"queued", "processing"}) {
		t.Errorf("event types = %v; the stale worker must add none", got)
	}
	if got := audio.deletedKeys(); len(got) != 0 {
		t.Errorf("deleted audio = %v; it belongs to the new owner", got)
	}
}
