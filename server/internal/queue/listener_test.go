package queue

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"slices"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/PelumiWeb/translator-app-go/server/internal/testdb"
)

// startListener runs a Listener for the test's database and waits until it
// is listening.
func startListener(t *testing.T, db *pgxpool.Pool, store *Store) (*Listener, *Bus) {
	t.Helper()
	bus := NewBus()
	listener := NewListener(db, store, bus, slog.New(slog.NewTextHandler(io.Discard, nil)))

	ctx, cancel := context.WithCancel(context.Background())
	stopped := make(chan struct{})
	go func() {
		listener.Run(ctx)
		close(stopped)
	}()
	t.Cleanup(func() {
		cancel()
		<-stopped
	})

	select {
	case <-listener.Ready():
	case <-time.After(5 * time.Second):
		t.Fatal("the listener did not become ready within 5s")
	}
	return listener, bus
}

// receiveAfter waits for the next event with a seq above afterSeq.
//
// Delivery is asynchronous, so a subscriber can be handed an event that was
// recorded just before it subscribed and was still on its way through
// Postgres. Consumers skip what they already have by seq; the SSE handler
// does, and so do these tests.
func receiveAfter(t *testing.T, ch <-chan Event, afterSeq int) Event {
	t.Helper()
	timeout := time.After(5 * time.Second)
	for {
		select {
		case e, ok := <-ch:
			if !ok {
				t.Fatal("the subscription was closed, want an event")
			}
			if e.Seq > afterSeq {
				return e
			}
		case <-timeout:
			t.Fatal("no event arrived within 5s")
			return Event{}
		}
	}
}

// The store and the bus are no longer connected in code. Events get from one
// to the other only through Postgres, which is what makes this work across
// processes.
func TestListenerDeliversEventsRecordedByTheStore(t *testing.T) {
	db := testdb.New(t)
	store := NewStore(db)
	_, bus := startListener(t, db, store)
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
	if _, err := store.AppendEvent(ctx, id, EventPartial, map[string]string{"text": "hello"}); err != nil {
		t.Fatalf("AppendEvent: %v", err)
	}
	if err := store.Complete(ctx, job, "hello world", "en"); err != nil {
		t.Fatalf("Complete: %v", err)
	}

	// Seq 1 is the "queued" event from Enqueue, recorded before subscribing.
	got := []Event{receiveAfter(t, live, 1), receiveAfter(t, live, 2), receiveAfter(t, live, 3)}

	wantTypes := []string{EventProcessing, EventPartial, EventDone}
	if types := eventTypes(got); !slices.Equal(types, wantTypes) {
		t.Fatalf("delivered %v, want %v", types, wantTypes)
	}
	// Each delivered event is exactly the stored row.
	stored, err := store.Events(ctx, id, 1)
	if err != nil {
		t.Fatalf("Events: %v", err)
	}
	for i := range got {
		if got[i].Seq != stored[i].Seq || string(got[i].Payload) != string(stored[i].Payload) {
			t.Errorf("delivered %+v, stored %+v", got[i], stored[i])
		}
	}
}

// NOTIFY is part of the transaction. A status change that is rolled back
// must not be heard by anyone.
func TestListenerHearsNothingFromARolledBackChange(t *testing.T) {
	db := testdb.New(t)
	store := NewStore(db)
	_, bus := startListener(t, db, store)
	ctx := context.Background()

	id, err := store.Enqueue(ctx, "en", "clip.wav")
	if err != nil {
		t.Fatalf("Enqueue: %v", err)
	}
	live, unsubscribe := bus.Subscribe(id)
	defer unsubscribe()

	// Nobody claimed the job, so this is refused and its transaction rolls
	// back, including the event insert and its notification.
	if err := store.Complete(ctx, Job{ID: id, Attempt: 1}, "nope", "en"); !errors.Is(err, ErrNotProcessing) {
		t.Fatalf("Complete error = %v, want ErrNotProcessing", err)
	}
	// A real event afterwards proves the listener is alive and in order.
	if _, err := store.Claim(ctx, testLease); err != nil {
		t.Fatalf("Claim: %v", err)
	}

	// After "queued" (seq 1) the next event must be the claim. Had the
	// rolled-back "done" been announced, it would arrive first.
	if got := receiveAfter(t, live, 1); got.Type != EventProcessing {
		t.Errorf("first event heard after queued = %s, want processing: the rolled-back event leaked", got.Type)
	}
}

// If the listener's connection drops, notifications sent meanwhile are lost.
// It must end the open subscriptions (so clients replay from the table) and
// then come back.
func TestListenerDisconnectsSubscribersAndRecoversWhenItsConnectionDrops(t *testing.T) {
	db := testdb.New(t)
	store := NewStore(db)
	listener, bus := startListener(t, db, store)
	ctx := context.Background()

	id, err := store.Enqueue(ctx, "en", "clip.wav")
	if err != nil {
		t.Fatalf("Enqueue: %v", err)
	}
	before, unsubscribe := bus.Subscribe(id)
	defer unsubscribe()

	// Kill the listener's connection from the server side, as a database
	// restart or a network fault would.
	oldPID := listener.backendPID.Load()
	if _, err := db.Exec(ctx, "SELECT pg_terminate_backend($1)", oldPID); err != nil {
		t.Fatalf("terminating the listener's connection: %v", err)
	}

	// The subscription must be closed. Events may still be in its buffer
	// (the "queued" one); read past them until the channel ends.
	closed := make(chan struct{})
	go func() {
		for range before {
		}
		close(closed)
	}()
	select {
	case <-closed:
	case <-time.After(5 * time.Second):
		t.Fatal("the subscription was left open after the listener lost its connection")
	}

	// Wait for the reconnect: a new backend PID.
	deadline := time.Now().Add(10 * time.Second)
	for listener.backendPID.Load() == oldPID {
		if time.Now().After(deadline) {
			t.Fatal("the listener did not reconnect within 10s")
		}
		time.Sleep(20 * time.Millisecond)
	}

	after, unsubscribeAfter := bus.Subscribe(id)
	defer unsubscribeAfter()
	if _, err := store.Claim(ctx, testLease); err != nil {
		t.Fatalf("Claim: %v", err)
	}
	if got := receiveAfter(t, after, 1); got.Type != EventProcessing {
		t.Errorf("after reconnecting, heard %s, want processing", got.Type)
	}
}
