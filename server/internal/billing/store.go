package billing

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	// ErrOtherAccount means the subscription belongs to another Shinar
	// account that still exists: one subscription, one account.
	ErrOtherAccount = errors.New("billing: the subscription belongs to another account")
	// ErrNotYours means a purchase made with another user's ID.
	ErrNotYours = errors.New("billing: the purchase was made for another account")
)

// State is where someone stands, as the phones are told it.
type State string

const (
	StateNone    State = "none"    // never subscribed
	StateTrial   State = "trial"   // in the free trial
	StateActive  State = "active"  // paid for
	StateGrace   State = "grace"   // a payment failed; Apple is retrying, and they keep access meanwhile
	StateExpired State = "expired" // ended, cancelled or refunded
)

// Access is what someone may do, and until when.
type Access struct {
	State State
	// Until is when the trial, the period or the grace period ends.
	Until time.Time
	// Renews says whether it renews at Until.
	Renews bool
	// Since is when the current period began: for the trial, the start of
	// what its cap counts.
	Since time.Time
}

// CanSend reports whether the person may send: in the trial, paid for, or in
// the grace period.
func (a Access) CanSend() bool {
	return a.State == StateTrial || a.State == StateActive || a.State == StateGrace
}

// Store keeps subscriptions in Postgres.
type Store struct {
	db *pgxpool.Pool
}

func NewStore(db *pgxpool.Pool) *Store {
	return &Store{db: db}
}

// Record applies a checked transaction, and its renewal information when
// there is one, to the subscription they belong to.
//
// userID is the account that sent it, or empty for a notification from
// Apple. A subscription belongs to one account: sent by another that still
// exists, it is refused with ErrOtherAccount; one whose account was deleted
// moves to the account that sends it next. A new subscription first heard of
// from Apple takes the account its purchase named, when there is one.
//
// Records can arrive out of order, from the phone and from Apple, so a
// transaction is applied only when it reaches further than the one kept, or
// is the same one again (now refunded, say); renewal information only when it
// is newer than the one kept.
func (s *Store) Record(ctx context.Context, userID string, t Transaction, r *RenewalInfo) error {
	err := pgx.BeginFunc(ctx, s.db, func(tx pgx.Tx) error {
		var owner, transactionID string
		var expires time.Time
		var renewalSigned *time.Time
		err := tx.QueryRow(ctx, `
			SELECT coalesce(user_id::text, ''), transaction_id, expires_at, renewal_signed_at
			FROM subscriptions WHERE original_transaction_id = $1 FOR UPDATE`, t.OriginalTransactionID,
		).Scan(&owner, &transactionID, &expires, &renewalSigned)
		switch {
		case errors.Is(err, pgx.ErrNoRows):
			if userID == "" {
				userID = s.accountNamed(ctx, tx, t.AppAccountToken)
			}
			return insert(ctx, tx, userID, t, r)
		case err != nil:
			return err
		case userID != "" && owner != "" && owner != userID:
			return ErrOtherAccount
		}
		if userID != "" && owner == "" {
			if _, err := tx.Exec(ctx, `UPDATE subscriptions SET user_id = $2 WHERE original_transaction_id = $1`,
				t.OriginalTransactionID, userID); err != nil {
				return err
			}
		}
		if t.ExpiresDate.Time().After(expires) || t.TransactionID == transactionID {
			if _, err := tx.Exec(ctx, `
				UPDATE subscriptions SET product_id = $2, environment = $3, transaction_id = $4, period_start = $5,
				       expires_at = $6, free_trial = $7, revoked_at = $8, updated_at = now()
				WHERE original_transaction_id = $1`,
				t.OriginalTransactionID, t.ProductID, t.Environment, t.TransactionID, t.PurchaseDate.Time(),
				t.ExpiresDate.Time(), t.FreeTrial(), nullTime(t.RevocationDate),
			); err != nil {
				return err
			}
		}
		if r != nil && (renewalSigned == nil || !r.SignedDate.Time().Before(*renewalSigned)) {
			return applyRenewal(ctx, tx, t.OriginalTransactionID, *r)
		}
		return nil
	})
	if errors.Is(err, ErrOtherAccount) {
		return err
	}
	if err != nil {
		return fmt.Errorf("recording a purchase: %w", err)
	}
	return nil
}

func insert(ctx context.Context, tx pgx.Tx, userID string, t Transaction, r *RenewalInfo) error {
	if _, err := tx.Exec(ctx, `
		INSERT INTO subscriptions (original_transaction_id, user_id, product_id, environment, transaction_id,
		                           period_start, expires_at, free_trial, revoked_at)
		VALUES ($1, NULLIF($2, '')::uuid, $3, $4, $5, $6, $7, $8, $9)`,
		t.OriginalTransactionID, userID, t.ProductID, t.Environment, t.TransactionID,
		t.PurchaseDate.Time(), t.ExpiresDate.Time(), t.FreeTrial(), nullTime(t.RevocationDate),
	); err != nil {
		return err
	}
	if r == nil {
		return nil
	}
	return applyRenewal(ctx, tx, t.OriginalTransactionID, *r)
}

func applyRenewal(ctx context.Context, tx pgx.Tx, originalTransactionID string, r RenewalInfo) error {
	// The grace period counts only while Apple is still retrying the payment.
	var grace *time.Time
	if r.IsInBillingRetryPeriod {
		grace = nullTime(r.GracePeriodExpiresDate)
	}
	_, err := tx.Exec(ctx, `
		UPDATE subscriptions SET auto_renew = $2, grace_until = $3, renewal_signed_at = $4, updated_at = now()
		WHERE original_transaction_id = $1`,
		originalTransactionID, r.AutoRenewStatus == 1, grace, r.SignedDate.Time(),
	)
	return err
}

// accountNamed is the account a purchase was made for, when its token is
// the ID of an account that exists; empty otherwise.
func (s *Store) accountNamed(ctx context.Context, tx pgx.Tx, token string) string {
	if token == "" {
		return ""
	}
	var id string
	// The phone sends the ID as Apple's UUID type writes it, in capitals.
	err := tx.QueryRow(ctx, `SELECT id::text FROM users WHERE id::text = $1`, strings.ToLower(token)).Scan(&id)
	if err != nil {
		return "" // no such account, or not an ID at all: Apple's records are kept either way
	}
	return id
}

// Access is where userID stands at now: the subscription that reaches
// furthest decides.
func (s *Store) Access(ctx context.Context, userID string, now time.Time) (Access, error) {
	rows, err := s.db.Query(ctx, `
		SELECT period_start, expires_at, free_trial, auto_renew, grace_until, revoked_at
		FROM subscriptions WHERE user_id = $1`, userID)
	if err != nil {
		return Access{}, fmt.Errorf("reading subscriptions: %w", err)
	}
	type period struct {
		start, expires time.Time
		trial, renews  bool
		grace, revoked *time.Time
	}
	periods, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (period, error) {
		var p period
		err := row.Scan(&p.start, &p.expires, &p.trial, &p.renews, &p.grace, &p.revoked)
		return p, err
	})
	if err != nil {
		return Access{}, fmt.Errorf("reading subscriptions: %w", err)
	}
	if len(periods) == 0 {
		return Access{State: StateNone}, nil
	}
	best := Access{State: StateExpired}
	for _, p := range periods {
		if p.revoked != nil {
			continue
		}
		var a Access
		switch {
		case now.Before(p.expires) && p.trial:
			a = Access{State: StateTrial, Until: p.expires, Renews: p.renews, Since: p.start}
		case now.Before(p.expires):
			a = Access{State: StateActive, Until: p.expires, Renews: p.renews, Since: p.start}
		case p.grace != nil && now.Before(*p.grace):
			a = Access{State: StateGrace, Until: *p.grace, Renews: true, Since: p.start}
		default:
			continue
		}
		if !best.CanSend() || a.Until.After(best.Until) {
			best = a
		}
	}
	return best, nil
}

// Noted records a notification from Apple, once.
func (s *Store) Noted(ctx context.Context, n Notification, signedPayload string) error {
	_, err := s.db.Exec(ctx, `
		INSERT INTO store_notifications (id, type, subtype, signed_payload) VALUES ($1, $2, $3, $4)
		ON CONFLICT (id) DO NOTHING`, n.UUID, n.Type, n.Subtype, signedPayload)
	if err != nil {
		return fmt.Errorf("recording a notification: %w", err)
	}
	return nil
}

func nullTime(m Millis) *time.Time {
	if m == 0 {
		return nil
	}
	t := m.Time()
	return &t
}
