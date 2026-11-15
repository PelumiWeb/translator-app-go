package push

import (
	"context"
	"errors"
	"os"
	"strings"
	"testing"
)

// TestLiveKeys checks the real keys with Apple and Google (make push-check).
// It sends to a made-up phone, so nothing arrives anywhere: an answer that
// the phone is unknown proves the server signed in.
func TestLiveKeys(t *testing.T) {
	ctx := context.Background()
	ran := false

	if file := os.Getenv("FCM_SERVICE_ACCOUNT_FILE"); file != "" {
		ran = true
		account, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		f, err := NewFCM(account)
		if err != nil {
			t.Fatal(err)
		}
		err = f.Send(ctx, "made-up-token-from-the-push-check", Notification{Title: "check", Body: "check"})
		// 400: Google read the request, so the sign-in worked.
		if err == nil || !(errors.Is(err, ErrGone) || strings.Contains(err.Error(), "fcm: 400")) {
			t.Errorf("FCM: %v; want the made-up phone refused", err)
		} else {
			t.Logf("FCM: signed in as the service account; the made-up phone was refused (%v)", err)
		}
	}

	if file := os.Getenv("APNS_KEY_FILE"); file != "" {
		ran = true
		p8, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		a, err := NewAPNs(p8, os.Getenv("APNS_KEY_ID"), envOr("APNS_TEAM_ID", "4USZRKF452"), envOr("APPLE_AUDIENCE", "com.pelumi.shinar"))
		if err != nil {
			t.Fatal(err)
		}
		for _, sandbox := range []bool{true, false} {
			err := a.Send(ctx, strings.Repeat("0", 64), sandbox, Notification{Title: "check", Body: "check"})
			if !errors.Is(err, ErrGone) {
				t.Errorf("APNs (sandbox %v): %v; want the made-up phone refused as BadDeviceToken", sandbox, err)
			} else {
				t.Logf("APNs (sandbox %v): the key works; the made-up phone was refused (%v)", sandbox, err)
			}
		}
	}

	if !ran {
		t.Skip("set FCM_SERVICE_ACCOUNT_FILE or APNS_KEY_FILE (and APNS_KEY_ID) to check the keys")
	}
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
