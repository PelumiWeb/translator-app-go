package billing

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/PelumiWeb/translator-app-go/server/internal/accounts"
	"github.com/PelumiWeb/translator-app-go/server/internal/testdb"
)

// period is a transaction of the subscription "orig": number n, from start,
// for length, a free trial or paid.
func period(n string, start time.Time, length time.Duration, trial bool) Transaction {
	t := Transaction{
		TransactionID: n, OriginalTransactionID: "orig", BundleID: testBundle, ProductID: "com.pelumi.shinar.monthly",
		PurchaseDate: Millis(start.UnixMilli()), ExpiresDate: Millis(start.Add(length).UnixMilli()),
		Environment: EnvSandbox, SignedDate: Millis(start.UnixMilli()),
	}
	if trial {
		t.OfferType, t.OfferDiscountType = 1, "FREE_TRIAL"
	}
	return t
}

func renewal(signed time.Time, renews bool) *RenewalInfo {
	r := RenewalInfo{OriginalTransactionID: "orig", SignedDate: Millis(signed.UnixMilli())}
	if renews {
		r.AutoRenewStatus = 1
	}
	return &r
}

type billingWorld struct {
	store    *Store
	accounts *accounts.Store
}

func newBillingWorld(t *testing.T) billingWorld {
	db := testdb.New(t)
	return billingWorld{store: NewStore(db), accounts: accounts.NewStore(db)}
}

func (w billingWorld) user(t *testing.T, name string) string {
	t.Helper()
	_, u, _, err := w.accounts.SignIn(context.Background(), accounts.ProviderApple, name, "", name)
	if err != nil {
		t.Fatal(err)
	}
	return u.ID
}

func (w billingWorld) access(t *testing.T, userID string, at time.Time) Access {
	t.Helper()
	a, err := w.store.Access(context.Background(), userID, at)
	if err != nil {
		t.Fatal(err)
	}
	return a
}

var day = 24 * time.Hour

func TestFromTheTrialToPaidToExpired(t *testing.T) {
	w := newBillingWorld(t)
	ctx := context.Background()
	ada := w.user(t, "ada")
	start := time.Date(2026, 11, 23, 9, 0, 0, 0, time.UTC)

	if got := w.access(t, ada, start); got.State != StateNone || got.CanSend() {
		t.Errorf("before buying: %+v, want none", got)
	}
	if err := w.store.Record(ctx, ada, period("t1", start, 7*day, true), renewal(start, true)); err != nil {
		t.Fatal(err)
	}
	trial := w.access(t, ada, start.Add(day))
	if trial.State != StateTrial || !trial.Renews || !trial.Since.Equal(start) || !trial.Until.Equal(start.Add(7*day)) {
		t.Errorf("in the trial: %+v", trial)
	}

	// Apple renews it into the first paid month.
	paid := period("t2", start.Add(7*day), 30*day, false)
	if err := w.store.Record(ctx, "", paid, renewal(start.Add(7*day), true)); err != nil {
		t.Fatal(err)
	}
	if got := w.access(t, ada, start.Add(8*day)); got.State != StateActive || !got.Until.Equal(start.Add(37*day)) {
		t.Errorf("paid: %+v", got)
	}
	// The trial's record arrives again, late: it changes nothing.
	if err := w.store.Record(ctx, "", period("t1", start, 7*day, true), nil); err != nil {
		t.Fatal(err)
	}
	if got := w.access(t, ada, start.Add(8*day)); got.State != StateActive {
		t.Errorf("after a late, older record: %+v, want still active", got)
	}
	if got := w.access(t, ada, start.Add(40*day)); got.State != StateExpired || got.CanSend() {
		t.Errorf("after the month: %+v, want expired", got)
	}
}

func TestAFailedPaymentKeepsAccessThroughTheGracePeriod(t *testing.T) {
	w := newBillingWorld(t)
	ctx := context.Background()
	ada := w.user(t, "ada")
	start := time.Date(2026, 11, 23, 9, 0, 0, 0, time.UTC)
	ends := start.Add(30 * day)
	if err := w.store.Record(ctx, ada, period("t1", start, 30*day, false), renewal(start, true)); err != nil {
		t.Fatal(err)
	}

	retrying := renewal(ends, true)
	retrying.IsInBillingRetryPeriod = true
	retrying.GracePeriodExpiresDate = Millis(ends.Add(16 * day).UnixMilli())
	if err := w.store.Record(ctx, "", period("t1", start, 30*day, false), retrying); err != nil {
		t.Fatal(err)
	}
	if got := w.access(t, ada, ends.Add(day)); got.State != StateGrace || !got.CanSend() {
		t.Errorf("in the grace period: %+v", got)
	}
	// An older renewal record, arriving late, does not undo the newer one.
	if err := w.store.Record(ctx, "", period("t1", start, 30*day, false), renewal(start.Add(day), true)); err != nil {
		t.Fatal(err)
	}
	if got := w.access(t, ada, ends.Add(day)); got.State != StateGrace {
		t.Errorf("after a late, older renewal record: %+v, want still in grace", got)
	}
	if got := w.access(t, ada, ends.Add(17*day)); got.State != StateExpired {
		t.Errorf("after the grace period: %+v, want expired", got)
	}
}

func TestARefundEndsAccess(t *testing.T) {
	w := newBillingWorld(t)
	ctx := context.Background()
	ada := w.user(t, "ada")
	start := time.Date(2026, 11, 23, 9, 0, 0, 0, time.UTC)
	month := period("t1", start, 30*day, false)
	if err := w.store.Record(ctx, ada, month, nil); err != nil {
		t.Fatal(err)
	}
	month.RevocationDate = Millis(start.Add(2 * day).UnixMilli()) // the same transaction, now refunded
	if err := w.store.Record(ctx, "", month, nil); err != nil {
		t.Fatal(err)
	}
	if got := w.access(t, ada, start.Add(3*day)); got.State != StateExpired {
		t.Errorf("after the refund: %+v, want expired", got)
	}
}

func TestOneSubscriptionOneAccount(t *testing.T) {
	w := newBillingWorld(t)
	ctx := context.Background()
	ada, rosa := w.user(t, "ada"), w.user(t, "rosa")
	start := time.Date(2026, 11, 23, 9, 0, 0, 0, time.UTC)
	month := period("t1", start, 30*day, false)
	if err := w.store.Record(ctx, ada, month, nil); err != nil {
		t.Fatal(err)
	}

	// The same Apple account, signed in to Shinar as someone else.
	if err := w.store.Record(ctx, rosa, month, nil); !errors.Is(err, ErrOtherAccount) {
		t.Errorf("another live account: err = %v, want ErrOtherAccount", err)
	}
	// Ada deletes her account; Apple goes on billing, and Rosa restores it.
	if err := w.accounts.DeleteUser(ctx, ada); err != nil {
		t.Fatal(err)
	}
	if err := w.store.Record(ctx, rosa, month, nil); err != nil {
		t.Fatal(err)
	}
	if got := w.access(t, rosa, start.Add(day)); got.State != StateActive {
		t.Errorf("rosa after restoring: %+v, want active", got)
	}
}

func TestApplesFirstWordNamesTheAccountFromThePurchase(t *testing.T) {
	w := newBillingWorld(t)
	ctx := context.Background()
	ada := w.user(t, "ada")
	start := time.Date(2026, 11, 23, 9, 0, 0, 0, time.UTC)

	// Apple's notification arrives before the phone has said anything. The
	// phone's UUID type writes the token in capitals.
	trial := period("t1", start, 7*day, true)
	trial.AppAccountToken = upper(ada)
	if err := w.store.Record(ctx, "", trial, nil); err != nil {
		t.Fatal(err)
	}
	if got := w.access(t, ada, start.Add(day)); got.State != StateTrial {
		t.Errorf("ada: %+v, want in the trial", got)
	}

	// A token that names no account leaves the subscription with none.
	stray := period("s1", start, 7*day, true)
	stray.OriginalTransactionID, stray.AppAccountToken = "stray", "not an id"
	if err := w.store.Record(ctx, "", stray, nil); err != nil {
		t.Fatal(err)
	}
}

func upper(id string) string {
	b := []byte(id)
	for i, c := range b {
		if c >= 'a' && c <= 'f' {
			b[i] = c - 'a' + 'A'
		}
	}
	return string(b)
}
