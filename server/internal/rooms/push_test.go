package rooms

import (
	"context"
	"errors"
	"slices"
	"sync"
	"testing"

	"github.com/PelumiWeb/translator-app-go/server/internal/accounts"
	"github.com/PelumiWeb/translator-app-go/server/internal/provider"
	"github.com/PelumiWeb/translator-app-go/server/internal/push"
)

// fakePusher records what it sends, answering each phone's token with
// errs[token], or nil.
type fakePusher struct {
	mu   sync.Mutex
	errs map[string]error
	sent []string // "service token"
	last push.Notification
}

func (f *fakePusher) Push(_ context.Context, to push.Device, n push.Notification) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.sent = append(f.sent, to.Service+" "+to.Token)
	f.last = n
	return f.errs[to.Token]
}

// phone signs the user called name in on a new phone, which gives the
// server its push address.
func (w world) phone(t *testing.T, name, service, token string) {
	t.Helper()
	ctx := context.Background()
	session, _, _, err := w.accounts.SignIn(ctx, accounts.ProviderApple, name, "", name)
	if err != nil {
		t.Fatal(err)
	}
	if err := w.accounts.SetPushToken(ctx, session, service, token); err != nil {
		t.Fatal(err)
	}
}

// pushesFor lists the languages a message has push jobs for.
func (w world) pushesFor(t *testing.T, messageID string) []string {
	t.Helper()
	rows, err := w.db.Query(context.Background(),
		`SELECT target_lang FROM jobs WHERE message_id = $1 AND kind = 'push' ORDER BY target_lang`, messageID)
	if err != nil {
		t.Fatal(err)
	}
	var langs []string
	for rows.Next() {
		var lang string
		if err := rows.Scan(&lang); err != nil {
			t.Fatal(err)
		}
		langs = append(langs, lang)
	}
	return langs
}

func TestAMessageIsPushedWhenItIsReadyForTheListener(t *testing.T) {
	t.Run("at once, to a listener of the same language", func(t *testing.T) {
		w := newWorld(t)
		ada, ben := w.user(t, "ada", "en"), w.user(t, "ben", "en")
		m := w.send(t, w.open(t, ada, ben), ada, "en", "hello")
		if got := w.pushesFor(t, m.ID); !slices.Equal(got, []string{"en"}) {
			t.Errorf("pushes = %v, want one in English", got)
		}
	})
	t.Run("once translated and spoken, to another", func(t *testing.T) {
		w, d, m := deliveryWorld(t)
		if got := w.pushesFor(t, m.ID); len(got) != 0 {
			t.Errorf("pushes before delivery = %v, want none", got)
		}
		if err := d.Deliver(context.Background(), m.ID, "es", 1); err != nil {
			t.Fatal(err)
		}
		if got := w.pushesFor(t, m.ID); !slices.Equal(got, []string{"es"}) {
			t.Errorf("pushes = %v, want one in Spanish", got)
		}
	})
	t.Run("when only the voice failed, for the phone to read", func(t *testing.T) {
		w, d, m := deliveryWorld(t)
		ctx := context.Background()
		if err := w.rooms.saveTranslation(ctx, m.ID, "es", "hola"); err != nil {
			t.Fatal(err)
		}
		if err := d.DeliveryFailed(ctx, m.ID, "es", "engine down"); err != nil {
			t.Fatal(err)
		}
		if got := w.pushesFor(t, m.ID); !slices.Equal(got, []string{"es"}) {
			t.Errorf("pushes = %v, want one in Spanish", got)
		}
	})
	t.Run("not when the delivery failed", func(t *testing.T) {
		w, d, m := deliveryWorld(t)
		if err := d.DeliveryFailed(context.Background(), m.ID, "es", "no such language"); err != nil {
			t.Fatal(err)
		}
		if got := w.pushesFor(t, m.ID); len(got) != 0 {
			t.Errorf("pushes = %v, want none", got)
		}
	})
}

func TestPushReachesTheListenersPhones(t *testing.T) {
	w, d, m := deliveryWorld(t)
	pusher := &fakePusher{}
	d.Pusher, d.Phones = pusher, w.accounts
	// Rosa has two phones; Ada, who sent it, has one, which hears nothing.
	w.phone(t, "rosa", push.ServiceAPNs, "rosa-iphone")
	w.phone(t, "rosa", push.ServiceFCM, "rosa-android")
	w.phone(t, "ada", push.ServiceAPNsSandbox, "ada-iphone")

	if err := d.Push(context.Background(), m.ID, "es"); err != nil {
		t.Fatal(err)
	}
	slices.Sort(pusher.sent)
	if want := []string{"apns rosa-iphone", "fcm rosa-android"}; !slices.Equal(pusher.sent, want) {
		t.Errorf("sent to %v, want %v", pusher.sent, want)
	}
	n := pusher.last
	if n.Title != "ada" || n.Body != "Sent a voice note · 0:02" || n.CollapseID != m.ID ||
		n.Thread != m.RoomID || n.Data["room_id"] != m.RoomID || n.Data["message_id"] != m.ID {
		t.Errorf("notification = %+v", n)
	}

	// Nobody speaks English but Ada.
	pusher.sent = nil
	if err := d.Push(context.Background(), m.ID, "en"); err != nil || len(pusher.sent) != 0 {
		t.Errorf("English: sent %v, err %v; want nothing", pusher.sent, err)
	}
}

func TestPushForgetsPhonesThatAreGoneAndRetriesWhatMayWork(t *testing.T) {
	tests := []struct {
		name          string
		err           error
		wantRetry     bool
		wantPermanent bool
		wantForgotten bool
	}{
		{"gone", push.ErrGone, false, false, true},
		{"busy", errors.New("apns: 503"), true, false, false},
		{"refused", provider.Permanent(errors.New("apns: 400 BadTopic")), false, true, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w, d, m := deliveryWorld(t)
			d.Pusher = &fakePusher{errs: map[string]error{"rosa-iphone": tt.err}}
			d.Phones = w.accounts
			w.phone(t, "rosa", push.ServiceAPNs, "rosa-iphone")

			err := d.Push(context.Background(), m.ID, "es")
			if (err != nil && !provider.IsPermanent(err)) != tt.wantRetry || provider.IsPermanent(err) != tt.wantPermanent {
				t.Errorf("err = %v; want retry %v, permanent %v", err, tt.wantRetry, tt.wantPermanent)
			}
			var left int
			if err := w.db.QueryRow(context.Background(), `SELECT count(*) FROM push_tokens`).Scan(&left); err != nil {
				t.Fatal(err)
			}
			if (left == 0) != tt.wantForgotten {
				t.Errorf("%d push tokens left; want forgotten %v", left, tt.wantForgotten)
			}
		})
	}
}

func TestPushWithoutAPushServiceDoesNothing(t *testing.T) {
	_, d, m := deliveryWorld(t)
	if err := d.Push(context.Background(), m.ID, "es"); err != nil {
		t.Errorf("err = %v", err)
	}
}

func TestClock(t *testing.T) {
	for ms, want := range map[int]string{0: "0:00", 1400: "0:01", 1500: "0:02", 14_000: "0:14", 65_000: "1:05"} {
		if got := clock(ms); got != want {
			t.Errorf("clock(%d) = %q, want %q", ms, got, want)
		}
	}
}
