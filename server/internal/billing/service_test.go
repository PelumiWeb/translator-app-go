package billing

import (
	"context"
	"errors"
	"testing"
	"time"
)

func newService(t *testing.T) (*Service, billingWorld, chain) {
	t.Helper()
	w := newBillingWorld(t)
	c := newChain(t, chainOptions{})
	return &Service{
		Verifier: Verifier{Roots: c.roots, BundleID: testBundle},
		Store:    w.store,
		Now:      func() time.Time { return signedAt.Add(time.Hour) },
	}, w, c
}

func TestAPurchaseIsTakenByTheAccountItWasMadeFor(t *testing.T) {
	s, w, c := newService(t)
	ctx := context.Background()
	ada, rosa := w.user(t, "ada"), w.user(t, "rosa")
	bought := c.sign(t, transaction(func(t map[string]any) { t["appAccountToken"] = upper(ada) }))

	if _, err := s.AcceptPurchase(ctx, rosa, bought, ""); !errors.Is(err, ErrNotYours) {
		t.Errorf("sent by rosa: err = %v, want ErrNotYours", err)
	}
	access, err := s.AcceptPurchase(ctx, ada, bought, c.sign(t, map[string]any{
		"originalTransactionId": "2000000001", "autoRenewStatus": 1, "signedDate": signedAt.UnixMilli(),
	}))
	if err != nil || access.State != StateTrial || !access.Renews {
		t.Errorf("sent by ada: %+v, %v; want in the trial, renewing", access, err)
	}

	// A purchase with no token, such as an offer code redeemed in the App
	// Store, is taken by whoever sends it first.
	untokened := c.sign(t, transaction(func(t map[string]any) {
		t["originalTransactionId"], t["transactionId"] = "3000000001", "3000000001"
		delete(t, "appAccountToken")
	}))
	if access, err := s.AcceptPurchase(ctx, rosa, untokened, ""); err != nil || access.State != StateTrial {
		t.Errorf("rosa's code: %+v, %v", access, err)
	}
}

func TestAPurchaseWithAnotherSubscriptionsRenewalIsRefused(t *testing.T) {
	s, w, c := newService(t)
	ada := w.user(t, "ada")
	bought := c.sign(t, transaction(func(t map[string]any) { t["appAccountToken"] = ada }))
	other := c.sign(t, map[string]any{"originalTransactionId": "someone-else", "signedDate": signedAt.UnixMilli()})

	if _, err := s.AcceptPurchase(context.Background(), ada, bought, other); !errors.Is(err, ErrInvalid) {
		t.Errorf("err = %v, want ErrInvalid", err)
	}
}

func TestANotificationIsAppliedAndKeptOnce(t *testing.T) {
	s, w, c := newService(t)
	ctx := context.Background()
	ada := w.user(t, "ada")
	refunded := c.sign(t, transaction(func(t map[string]any) {
		t["appAccountToken"] = ada
		t["revocationDate"] = signedAt.Add(30 * time.Minute).UnixMilli()
	}))
	payload := c.sign(t, map[string]any{
		"notificationType": "REFUND", "notificationUUID": "n-1", "signedDate": signedAt.UnixMilli(),
		"data": map[string]any{"bundleId": testBundle, "environment": EnvSandbox, "signedTransactionInfo": refunded},
	})

	for range 2 { // Apple sends it again when unsure it arrived
		n, err := s.HandleNotification(ctx, payload)
		if err != nil || n.Type != "REFUND" {
			t.Fatalf("notification = %+v, %v", n, err)
		}
	}
	if got, _ := s.Access(ctx, ada); got.State != StateExpired {
		t.Errorf("after the refund: %+v, want expired", got)
	}
	var kept int
	if err := s.Store.db.QueryRow(ctx, `SELECT count(*) FROM store_notifications`).Scan(&kept); err != nil || kept != 1 {
		t.Errorf("notifications kept = %d (%v), want 1", kept, err)
	}
}

func TestATestNotificationIsKeptAndChangesNothing(t *testing.T) {
	s, _, c := newService(t)
	payload := c.sign(t, map[string]any{
		"notificationType": "TEST", "notificationUUID": "n-test", "signedDate": signedAt.UnixMilli(),
		"data": map[string]any{"bundleId": testBundle, "environment": EnvSandbox},
	})
	if n, err := s.HandleNotification(context.Background(), payload); err != nil || n.Type != "TEST" {
		t.Errorf("notification = %+v, %v", n, err)
	}
}
