package queue

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"slices"
	"strings"
	"sync"
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
	pool := NewPool(store, p, audio, slog.New(slog.NewTextHandler(io.Discard, nil)), Config{
		Workers:       2,
		PollInterval:  10 * time.Millisecond,
		Lease:         testLease,
		ShutdownGrace: grace,
	})

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

func TestPoolMarksJobFailedOnProviderError(t *testing.T) {
	db := testdb.New(t)
	store := NewStore(db, NewBus())
	ctx := context.Background()

	id, err := store.Enqueue(ctx, "en", "clip.wav")
	if err != nil {
		t.Fatalf("Enqueue: %v", err)
	}
	startPool(t, store, fake.Provider{Err: errors.New("provider exploded")}, &fakeAudio{}, time.Second)

	row := waitForStatus(t, db, id, "failed")
	if row.LastError == nil || *row.LastError != "provider exploded" {
		t.Errorf("last_error = %v, want %q", row.LastError, "provider exploded")
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
