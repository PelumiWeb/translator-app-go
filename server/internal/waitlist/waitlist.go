// Package waitlist keeps the people who asked, on the website, to try Shinar
// before it is on the stores, and hands them out in batches for invitations
// (ADR 0030).
package waitlist

import (
	"context"
	"fmt"
	"slices"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// The phones a person can ask for, as stored in waitlist.phone.
const (
	PhoneIPhone  = "iphone"
	PhoneAndroid = "android"
)

// Entry is one person on the list.
type Entry struct {
	Email     string
	Name      string // their first name, if they gave one
	Phone     string
	CreatedAt time.Time
	InvitedAt *time.Time // nil while they are waiting
}

// Store keeps the list in Postgres.
type Store struct {
	db *pgxpool.Pool
}

func NewStore(db *pgxpool.Pool) *Store {
	return &Store{db: db}
}

// Join puts someone on the list. Joining again keeps their place and takes
// the phone and name given last; an empty name keeps the one given before.
func (s *Store) Join(ctx context.Context, e Entry) error {
	_, err := s.db.Exec(ctx, `
		INSERT INTO waitlist (email, name, phone) VALUES ($1, $2, $3)
		ON CONFLICT (lower(email)) DO UPDATE SET
			phone = excluded.phone,
			name  = CASE WHEN excluded.name = '' THEN waitlist.name ELSE excluded.name END`,
		e.Email, e.Name, e.Phone)
	if err != nil {
		return fmt.Errorf("joining the waitlist: %w", err)
	}
	return nil
}

// Invite takes the next n people waiting for phone, those who joined first,
// marks them invited, and returns them. Taking them is one statement, so two
// runs at once never hand out the same person twice.
func (s *Store) Invite(ctx context.Context, phone string, n int) ([]Entry, error) {
	rows, err := s.db.Query(ctx, `
		UPDATE waitlist SET invited_at = now()
		WHERE id IN (
			SELECT id FROM waitlist WHERE phone = $1 AND invited_at IS NULL
			ORDER BY created_at LIMIT $2
			FOR UPDATE SKIP LOCKED
		)
		RETURNING email, name, phone, created_at, invited_at`,
		phone, n)
	if err != nil {
		return nil, fmt.Errorf("taking people from the waitlist: %w", err)
	}
	entries, err := pgx.CollectRows(rows, pgx.RowToStructByPos[Entry])
	if err != nil {
		return nil, fmt.Errorf("taking people from the waitlist: %w", err)
	}
	// RETURNING gives no order of its own; first come, first listed.
	slices.SortFunc(entries, func(a, b Entry) int { return a.CreatedAt.Compare(b.CreatedAt) })
	return entries, nil
}

// List returns everyone on the list, invited or not, those who joined first
// first. It changes nothing.
func (s *Store) List(ctx context.Context) ([]Entry, error) {
	rows, err := s.db.Query(ctx, `SELECT email, name, phone, created_at, invited_at FROM waitlist ORDER BY created_at`)
	if err != nil {
		return nil, fmt.Errorf("listing the waitlist: %w", err)
	}
	entries, err := pgx.CollectRows(rows, pgx.RowToStructByPos[Entry])
	if err != nil {
		return nil, fmt.Errorf("listing the waitlist: %w", err)
	}
	return entries, nil
}

// Remove takes someone off the list, whether or not they were invited, for
// those who ask to be forgotten. It reports whether they were on it.
func (s *Store) Remove(ctx context.Context, email string) (bool, error) {
	tag, err := s.db.Exec(ctx, `DELETE FROM waitlist WHERE lower(email) = lower($1)`, email)
	if err != nil {
		return false, fmt.Errorf("removing from the waitlist: %w", err)
	}
	return tag.RowsAffected() > 0, nil
}

// Waiting counts who is still waiting for each phone.
func (s *Store) Waiting(ctx context.Context) (map[string]int, error) {
	rows, err := s.db.Query(ctx, `SELECT phone, count(*) FROM waitlist WHERE invited_at IS NULL GROUP BY phone`)
	if err != nil {
		return nil, fmt.Errorf("counting the waitlist: %w", err)
	}
	counts := map[string]int{PhoneIPhone: 0, PhoneAndroid: 0}
	var phone string
	var n int
	_, err = pgx.ForEachRow(rows, []any{&phone, &n}, func() error {
		counts[phone] = n
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("counting the waitlist: %w", err)
	}
	return counts, nil
}
