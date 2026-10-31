package rooms

import (
	"context"
	"io"
	"log/slog"
	"testing"
	"time"
)

func TestHubTellsTheMembersOfARoomOfItsChanges(t *testing.T) {
	w := newWorld(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	hub := NewHub(w.db, slog.New(slog.NewTextHandler(io.Discard, nil)))
	go hub.Run(ctx)
	<-hub.Ready()

	ada, rosa, eve := w.user(t, "ada", "en"), w.user(t, "rosa", "es"), w.user(t, "eve", "fr")
	toRosa, stopRosa := hub.Subscribe(rosa)
	defer stopRosa()
	toEve, stopEve := hub.Subscribe(eve)
	defer stopEve()

	room := w.open(t, ada, rosa)
	if got := receive(t, toRosa); got != (Change{RoomID: room}) {
		t.Errorf("first change = %+v, want the room opened", got)
	}
	m := w.send(t, room, ada, "en", "hello")
	if got := receive(t, toRosa); got != (Change{RoomID: room, MessageID: m.ID}) {
		t.Errorf("second change = %+v, want the message", got)
	}

	select {
	case got := <-toEve:
		t.Errorf("Eve, not in the room, heard %+v", got)
	case <-time.After(100 * time.Millisecond):
	}
}

func receive(t *testing.T, changes <-chan Change) Change {
	t.Helper()
	select {
	case c := <-changes:
		return c
	case <-time.After(5 * time.Second):
		t.Fatal("no change within 5s")
		return Change{}
	}
}
