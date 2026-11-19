package rooms

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"

	"github.com/PelumiWeb/translator-app-go/server/internal/accounts"
	"github.com/PelumiWeb/translator-app-go/server/internal/queue"
	"github.com/PelumiWeb/translator-app-go/server/internal/testdb"
)

func newLimitedWorld(t *testing.T, limits Limits) world {
	t.Helper()
	db := testdb.New(t)
	return world{db: db, accounts: accounts.NewStore(db), rooms: NewStore(db, queue.NewStore(db), limits)}
}

func voiceNote(room, sender string, ms int, key string) NewMessage {
	return NewMessage{RoomID: room, SenderID: sender, Lang: "en", Transcript: "hello", AudioKey: key + ".wav", DurationMS: ms, IdempotencyKey: key}
}

func textMessage(room, sender, key string) NewMessage {
	return NewMessage{RoomID: room, SenderID: sender, Kind: KindText, Lang: "en", Transcript: "hello", IdempotencyKey: key}
}

func TestSendingStopsAtTheDailyLimit(t *testing.T) {
	cases := []struct {
		name    string
		limits  Limits
		allowed []NewMessage // sent in order, all accepted
		refused NewMessage
		want    error
		other   NewMessage // of the other kind, still accepted after the refusal
	}{
		{
			name:    "voice notes, by their length",
			limits:  Limits{VoiceMS: 5000, Texts: 100},
			allowed: []NewMessage{voiceNote("", "", 2000, "v1"), voiceNote("", "", 3000, "v2")},
			refused: voiceNote("", "", 1, "v3"),
			want:    ErrVoiceLimit,
			other:   textMessage("", "", "t1"),
		},
		{
			name:    "a voice note that would go past it",
			limits:  Limits{VoiceMS: 5000, Texts: 100},
			allowed: []NewMessage{voiceNote("", "", 4000, "v1")},
			refused: voiceNote("", "", 2000, "v2"),
			want:    ErrVoiceLimit,
			other:   textMessage("", "", "t1"),
		},
		{
			name:    "text messages, by their number",
			limits:  Limits{VoiceMS: 60_000, Texts: 2},
			allowed: []NewMessage{textMessage("", "", "t1"), textMessage("", "", "t2")},
			refused: textMessage("", "", "t3"),
			want:    ErrTextLimit,
			other:   voiceNote("", "", 2000, "v1"),
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			w := newLimitedWorld(t, tc.limits)
			ctx := context.Background()
			ada, rosa := w.user(t, "ada", "en"), w.user(t, "rosa", "es")
			room := w.open(t, ada, rosa)
			with := func(m NewMessage) NewMessage {
				m.RoomID, m.SenderID = room, ada
				return m
			}

			for _, m := range tc.allowed {
				if _, _, err := w.rooms.AddMessage(ctx, with(m)); err != nil {
					t.Fatalf("sending %s: %v", m.IdempotencyKey, err)
				}
			}
			if _, _, err := w.rooms.AddMessage(ctx, with(tc.refused)); !errors.Is(err, tc.want) {
				t.Fatalf("past the limit: err = %v, want %v", err, tc.want)
			}
			// Nothing of the refused message is kept, nor any work to deliver it.
			var kept int
			if err := w.db.QueryRow(ctx, `SELECT count(*) FROM messages WHERE idempotency_key = $1`, tc.refused.IdempotencyKey).Scan(&kept); err != nil {
				t.Fatal(err)
			}
			if kept != 0 {
				t.Errorf("the refused message was kept")
			}
			if _, _, err := w.rooms.AddMessage(ctx, with(tc.other)); err != nil {
				t.Errorf("the other kind, after the refusal: %v", err)
			}
			// The limit is the sender's own: the other person can still send.
			theirs := tc.refused
			theirs.RoomID, theirs.SenderID, theirs.Lang = room, rosa, "es"
			if _, _, err := w.rooms.AddMessage(ctx, theirs); err != nil {
				t.Errorf("someone else sending: %v", err)
			}
		})
	}
}

func TestAResendPastTheLimitGetsTheKeptMessage(t *testing.T) {
	w := newLimitedWorld(t, Limits{VoiceMS: 60_000, Texts: 1})
	ctx := context.Background()
	ada, rosa := w.user(t, "ada", "en"), w.user(t, "rosa", "es")
	room := w.open(t, ada, rosa)

	first, created, err := w.rooms.AddMessage(ctx, textMessage(room, ada, "t1"))
	if err != nil || !created {
		t.Fatalf("first send: created = %v, err = %v", created, err)
	}
	again, created, err := w.rooms.AddMessage(ctx, textMessage(room, ada, "t1"))
	if err != nil || created || again.ID != first.ID {
		t.Errorf("resend at the limit: %q, created = %v, err = %v; want %q", again.ID, created, err, first.ID)
	}
}

func TestMessagesOlderThanADayDoNotCount(t *testing.T) {
	w := newLimitedWorld(t, Limits{VoiceMS: 5000, Texts: 1})
	ctx := context.Background()
	ada, rosa := w.user(t, "ada", "en"), w.user(t, "rosa", "es")
	room := w.open(t, ada, rosa)

	for _, m := range []NewMessage{voiceNote(room, ada, 5000, "v1"), textMessage(room, ada, "t1")} {
		if _, _, err := w.rooms.AddMessage(ctx, m); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := w.db.Exec(ctx, `UPDATE messages SET created_at = now() - interval '25 hours'`); err != nil {
		t.Fatal(err)
	}
	for _, m := range []NewMessage{voiceNote(room, ada, 5000, "v2"), textMessage(room, ada, "t2")} {
		if _, _, err := w.rooms.AddMessage(ctx, m); err != nil {
			t.Errorf("a day later, %s: %v", m.IdempotencyKey, err)
		}
	}
}

func TestMessagesSentAtOnceStayWithinTheLimit(t *testing.T) {
	const limit = 3
	w := newLimitedWorld(t, Limits{VoiceMS: 60_000, Texts: limit})
	ctx := context.Background()
	ada, rosa := w.user(t, "ada", "en"), w.user(t, "rosa", "es")
	room := w.open(t, ada, rosa)

	var (
		wg       sync.WaitGroup
		mu       sync.Mutex
		accepted int
	)
	for i := range 8 {
		wg.Go(func() {
			_, _, err := w.rooms.AddMessage(ctx, textMessage(room, ada, fmt.Sprintf("t%d", i)))
			switch {
			case err == nil:
				mu.Lock()
				accepted++
				mu.Unlock()
			case !errors.Is(err, ErrTextLimit):
				t.Errorf("send %d: %v", i, err)
			}
		})
	}
	wg.Wait()
	if accepted != limit {
		t.Errorf("%d of 8 sent at once were accepted, want %d", accepted, limit)
	}
}
