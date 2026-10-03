package queue

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

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
	want := Job{ID: id, SourceLang: "en", AudioKey: "clip.wav", Attempt: 1}
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
	if err := store.Complete(ctx, id, "text", "en"); !errors.Is(err, ErrNotProcessing) {
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

func eventTypes(events []Event) []string {
	types := make([]string, len(events))
	for i, e := range events {
		types[i] = e.Type
	}
	return types
}
