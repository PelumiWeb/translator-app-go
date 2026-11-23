package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/PelumiWeb/translator-app-go/server/internal/billing"
	"github.com/PelumiWeb/translator-app-go/server/internal/rooms"
)

// fakeBilling answers with access, and with err for a purchase or a
// notification that does not verify.
type fakeBilling struct {
	access    billing.Access
	err       error
	purchases []string // "user signed renewal"
}

func (f *fakeBilling) Access(context.Context, string) (billing.Access, error) { return f.access, nil }

func (f *fakeBilling) AcceptPurchase(_ context.Context, userID, signed, renewal string) (billing.Access, error) {
	f.purchases = append(f.purchases, userID+" "+signed+" "+renewal)
	return f.access, f.err
}

func (f *fakeBilling) HandleNotification(_ context.Context, payload string) (billing.Notification, error) {
	if f.err != nil {
		return billing.Notification{}, f.err
	}
	return billing.Notification{Type: "DID_RENEW", UUID: payload}, nil
}

// billedWorld is a rooms world on a server that requires a subscription.
func billedWorld(t *testing.T, access billing.Access) (roomsWorld, *fakeBilling, string) {
	t.Helper()
	w := newRoomsWorld(t, map[string]string{"ada": "en", "rosa": "es"})
	b := &fakeBilling{access: access}
	w.api.Billing, w.api.RequireSubscription = b, true
	w.api.TrialLimits = rooms.Limits{VoiceMS: 10_000, Texts: 2}
	return w, b, w.open(t, "ada", "rosa")
}

func TestWithoutASubscriptionNothingCanBeSent(t *testing.T) {
	for _, state := range []billing.State{billing.StateNone, billing.StateExpired} {
		w, _, room := billedWorld(t, billing.Access{State: state})
		requests := []struct {
			name string
			rec  func() int
		}{
			{"a text", func() int {
				return serve(w.api, jsonRequest(http.MethodPost, "/v1/rooms/"+room+"/messages", w.tokens["ada"], `{"text":"hello"}`)).Code
			}},
			{"a voice note", func() int {
				return serve(w.api, messageRequest(t, "/v1/rooms/"+room+"/messages", w.tokens["ada"], "hello", "2400", wavBytes(200))).Code
			}},
			{"the interpreter's transcription", func() int { return w.do(http.MethodPost, "/v1/jobs", "ada", "").Code }},
			{"the interpreter's speech", func() int { return w.do(http.MethodPost, "/v1/speech", "ada", "").Code }},
		}
		for _, r := range requests {
			if got := r.rec(); got != http.StatusPaymentRequired {
				t.Errorf("%s, %s: status %d, want 402", state, r.name, got)
			}
		}
		if keys := w.api.Messages.(*fakeAudio).keys(); len(keys) != 0 {
			t.Errorf("%s: stored %v, want no recording kept of a refused note", state, keys)
		}
		// Reading and hearing what one is sent needs nothing.
		if rec := w.do(http.MethodGet, "/v1/rooms/"+room+"/messages", "rosa", ""); rec.Code != http.StatusOK {
			t.Errorf("%s, reading: status %d, want 200", state, rec.Code)
		}
	}
}

func TestTheTrialSendsUntilItsCap(t *testing.T) {
	w, _, room := billedWorld(t, billing.Access{
		State: billing.StateTrial, Since: time.Now().Add(-time.Minute), Until: time.Now().Add(7 * 24 * time.Hour), Renews: true,
	})
	w.sendText(t, room, "ada", "one")
	w.sendText(t, room, "ada", "two")
	rec := serve(w.api, jsonRequest(http.MethodPost, "/v1/rooms/"+room+"/messages", w.tokens["ada"], `{"text":"three"}`))
	if rec.Code != http.StatusTooManyRequests || errorCode(rec.Body.Bytes()) != "trial_limit_reached" {
		t.Errorf("past the cap: status %d, body %s; want 429 trial_limit_reached", rec.Code, rec.Body)
	}

	var got subscriptionResponse
	if err := json.Unmarshal(w.do(http.MethodGet, "/v1/me/subscription", "ada", "").Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if !got.Required || got.State != "trial" || !got.Renews || got.Until == nil || got.Trial == nil ||
		got.Trial.TextsLeft != 0 || got.Trial.VoiceMSLeft != 10_000 {
		t.Errorf("subscription = %+v, trial %+v", got, got.Trial)
	}
}

func TestASubscriberIsHeldToTheDailyLimitsOnly(t *testing.T) {
	w, _, room := billedWorld(t, billing.Access{State: billing.StateActive, Until: time.Now().Add(24 * time.Hour)})
	for _, text := range []string{"one", "two", "three"} { // past the trial's 2
		w.sendText(t, room, "ada", text)
	}
}

func TestTheSubscriptionOfAServerThatTakesNoPurchases(t *testing.T) {
	w := newRoomsWorld(t, map[string]string{"ada": "en"})
	var got subscriptionResponse
	if err := json.Unmarshal(w.do(http.MethodGet, "/v1/me/subscription", "ada", "").Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if got.Required || got.State != "none" {
		t.Errorf("subscription = %+v, want not required", got)
	}
	if rec := w.do(http.MethodPost, "/v1/me/subscription/apple", "ada", `{}`); rec.Code != http.StatusNotImplemented {
		t.Errorf("purchase: status %d, want 501", rec.Code)
	}
}

func TestAPurchaseFromThePhone(t *testing.T) {
	tests := []struct {
		name       string
		err        error
		wantStatus int
		wantCode   string
	}{
		{"recorded", nil, http.StatusOK, ""},
		{"not Apple's", billing.ErrInvalid, http.StatusBadRequest, "invalid_purchase"},
		{"another app's", billing.ErrWrongApp, http.StatusBadRequest, "invalid_purchase"},
		{"another user's", billing.ErrNotYours, http.StatusForbidden, "not_your_purchase"},
		{"another account's subscription", billing.ErrOtherAccount, http.StatusConflict, "subscription_in_use"},
		{"the database", errors.New("connection reset"), http.StatusInternalServerError, "internal"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w, b, _ := billedWorld(t, billing.Access{State: billing.StateActive, Until: time.Now().Add(time.Hour)})
			b.err = tt.err
			rec := w.do(http.MethodPost, "/v1/me/subscription/apple", "ada", `{"signed_transaction":"tx","signed_renewal_info":"ri"}`)
			if rec.Code != tt.wantStatus || (tt.wantCode != "" && errorCode(rec.Body.Bytes()) != tt.wantCode) {
				t.Errorf("status %d, body %s; want %d %s", rec.Code, rec.Body, tt.wantStatus, tt.wantCode)
			}
			if len(b.purchases) != 1 || !strings.HasSuffix(b.purchases[0], " tx ri") {
				t.Errorf("purchases = %v", b.purchases)
			}
		})
	}
}

func TestAnAppStoreNotification(t *testing.T) {
	tests := []struct {
		name       string
		err        error
		wantStatus int
	}{
		{"applied", nil, http.StatusOK},
		{"not Apple's", billing.ErrInvalid, http.StatusBadRequest},
		{"failed to apply, so Apple sends it again", errors.New("connection reset"), http.StatusInternalServerError},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w, b, _ := billedWorld(t, billing.Access{})
			b.err = tt.err
			// No sign-in: Apple sends it.
			rec := send(w.api, http.MethodPost, "/v1/billing/apple/notifications", "", `{"signedPayload":"payload"}`)
			if rec.Code != tt.wantStatus {
				t.Errorf("status %d, body %s; want %d", rec.Code, rec.Body, tt.wantStatus)
			}
		})
	}
}
