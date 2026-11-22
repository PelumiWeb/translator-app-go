package rooms

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"
	"time"
)

func TestABlockStopsTheRoomBothWays(t *testing.T) {
	w := newWorld(t)
	ctx := context.Background()
	ada, rosa := w.user(t, "ada", "en"), w.user(t, "rosa", "es")
	room := w.open(t, ada, rosa)

	if err := w.rooms.Block(ctx, ada, rosa); err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		sender string
		want   error
	}{
		{rosa, ErrBlocked},
		{ada, ErrYouBlocked},
	}
	for _, tt := range tests {
		_, _, err := w.rooms.AddMessage(ctx, NewMessage{RoomID: room, SenderID: tt.sender, Lang: "en", Transcript: "hello", AudioKey: "a.wav", DurationMS: 1000})
		if !errors.Is(err, tt.want) {
			t.Errorf("sending while blocked: err = %v, want %v", err, tt.want)
		}
	}
	var kept int
	if err := w.db.QueryRow(ctx, `SELECT count(*) FROM messages`).Scan(&kept); err != nil || kept != 0 {
		t.Errorf("messages kept = %d (%v), want none: nothing of a refused message is kept", kept, err)
	}

	if err := w.rooms.Unblock(ctx, ada, rosa); err != nil {
		t.Fatal(err)
	}
	w.send(t, room, rosa, "es", "hola")
	w.send(t, room, ada, "en", "hello")
}

func TestABlockLeavesAResendOfAnEarlierNoteAlone(t *testing.T) {
	w := newWorld(t)
	ctx := context.Background()
	ada, rosa := w.user(t, "ada", "en"), w.user(t, "rosa", "es")
	room := w.open(t, ada, rosa)
	note := NewMessage{RoomID: room, SenderID: rosa, Lang: "es", Transcript: "hola", AudioKey: "a.wav", DurationMS: 1000, IdempotencyKey: "k1"}
	first, _, err := w.rooms.AddMessage(ctx, note)
	if err != nil {
		t.Fatal(err)
	}
	if err := w.rooms.Block(ctx, ada, rosa); err != nil {
		t.Fatal(err)
	}

	// The phone could not tell whether the first try arrived, and sends again.
	again, created, err := w.rooms.AddMessage(ctx, note)
	if err != nil || created || again.ID != first.ID {
		t.Errorf("resend = %v, created %v, err %v; want the note kept before the block", again.ID, created, err)
	}
	note.IdempotencyKey = "k2"
	if _, _, err := w.rooms.AddMessage(ctx, note); !errors.Is(err, ErrBlocked) {
		t.Errorf("a new note: err = %v, want ErrBlocked", err)
	}
}

func TestOnlyTheBlockerSeesTheRoomBlocked(t *testing.T) {
	w := newWorld(t)
	ctx := context.Background()
	ada, rosa := w.user(t, "ada", "en"), w.user(t, "rosa", "es")
	room := w.open(t, ada, rosa)
	if err := w.rooms.Block(ctx, ada, rosa); err != nil {
		t.Fatal(err)
	}

	for _, tt := range []struct {
		who  string
		want bool
	}{{ada, true}, {rosa, false}} {
		got, err := w.rooms.Room(ctx, tt.who, room)
		if err != nil {
			t.Fatal(err)
		}
		if got.YouBlocked != tt.want {
			t.Errorf("YouBlocked = %v, want %v", got.YouBlocked, tt.want)
		}
	}
}

func TestBlockedListsThePeopleBlocked(t *testing.T) {
	w := newWorld(t)
	ctx := context.Background()
	ada, rosa, kenji := w.user(t, "ada", "en"), w.user(t, "rosa", "es"), w.user(t, "kenji", "ja")

	for _, other := range []string{rosa, kenji, kenji} { // twice is not an error
		if err := w.rooms.Block(ctx, ada, other); err != nil {
			t.Fatal(err)
		}
	}
	blocked, err := w.rooms.Blocked(ctx, ada)
	if err != nil {
		t.Fatal(err)
	}
	if len(blocked) != 2 {
		t.Fatalf("blocked = %+v, want rosa and kenji", blocked)
	}
	if err := w.rooms.Unblock(ctx, ada, kenji); err != nil {
		t.Fatal(err)
	}
	if err := w.rooms.Unblock(ctx, ada, kenji); err != nil { // nor is lifting one that is gone
		t.Fatal(err)
	}
	blocked, err = w.rooms.Blocked(ctx, ada)
	if err != nil || len(blocked) != 1 || blocked[0].ID != rosa || blocked[0].Language != "es" {
		t.Errorf("blocked = %+v (%v), want rosa only", blocked, err)
	}
	if theirs, _ := w.rooms.Blocked(ctx, rosa); len(theirs) != 0 {
		t.Errorf("rosa's list = %+v, want empty: she blocked no one", theirs)
	}
}

func TestBlockingNoOne(t *testing.T) {
	w := newWorld(t)
	ctx := context.Background()
	ada := w.user(t, "ada", "en")
	for name, other := range map[string]string{
		"themselves": ada,
		"no such":    "00000000-0000-0000-0000-000000000000",
	} {
		if err := w.rooms.Block(ctx, ada, other); !errors.Is(err, ErrNoPerson) {
			t.Errorf("blocking %s: err = %v, want ErrNoPerson", name, err)
		}
	}
}

func TestRetryIsRefusedWhileBlocked(t *testing.T) {
	w := newWorld(t)
	ctx := context.Background()
	ada, rosa := w.user(t, "ada", "en"), w.user(t, "rosa", "es")
	room := w.open(t, ada, rosa)
	m := w.send(t, room, rosa, "es", "hola")
	if _, err := w.db.Exec(ctx, `UPDATE renditions SET status = 'failed' WHERE message_id = $1`, m.ID); err != nil {
		t.Fatal(err)
	}
	if err := w.rooms.Block(ctx, ada, rosa); err != nil {
		t.Fatal(err)
	}
	if _, err := w.rooms.Retry(ctx, rosa, m.ID); !errors.Is(err, ErrBlocked) {
		t.Errorf("retry: err = %v, want ErrBlocked", err)
	}
}

func TestABlockIsAnnouncedOnlyToTheBlocker(t *testing.T) {
	w := newWorld(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	ada, rosa := w.user(t, "ada", "en"), w.user(t, "rosa", "es")
	room := w.open(t, ada, rosa)

	hub := NewHub(w.db, slog.New(slog.NewTextHandler(io.Discard, nil)))
	go hub.Run(ctx)
	<-hub.Ready()
	adas, stopAda := hub.Subscribe(ada)
	defer stopAda()
	rosas, stopRosa := hub.Subscribe(rosa)
	defer stopRosa()

	if err := w.rooms.Block(ctx, ada, rosa); err != nil {
		t.Fatal(err)
	}
	select {
	case change := <-adas:
		if change.RoomID != room {
			t.Errorf("ada heard of room %s, want %s", change.RoomID, room)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("ada's stream heard nothing of her block")
	}
	select {
	case change := <-rosas:
		t.Errorf("rosa heard %+v: the stream would give the block away", change)
	case <-time.After(300 * time.Millisecond):
	}
}
