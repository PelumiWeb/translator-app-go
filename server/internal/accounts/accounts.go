// Package accounts stores users, the identities they sign in with, and their
// sessions.
package accounts

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Sentinel errors: callers compare with errors.Is instead of matching text.
var (
	ErrNoSession   = errors.New("accounts: no such session")
	ErrNoUser      = errors.New("accounts: no such user")
	ErrHandleTaken = errors.New("accounts: the handle is taken")
)

// Identity providers, as stored in identities.provider.
const (
	ProviderApple  = "apple"
	ProviderGoogle = "google"
)

// User is a person with an account.
type User struct {
	ID          string
	DisplayName string
	Handle      string // empty until chosen
	Language    string // empty until chosen
}

// ProfileUpdate holds the fields to change. A nil field is left as it is,
// which is why they are pointers: "not given" differs from "set to empty".
type ProfileUpdate struct {
	DisplayName *string
	Handle      *string // lower case
	Language    *string
}

// Store keeps accounts in Postgres.
type Store struct {
	db *pgxpool.Pool
}

func NewStore(db *pgxpool.Pool) *Store {
	return &Store{db: db}
}

// SignIn finds the user behind an identity, creating one on its first sign-in,
// and opens a session for them. The returned token is the only copy: the
// server keeps just its hash, so a stolen database holds no usable sessions.
func (s *Store) SignIn(ctx context.Context, provider, subject, email, name string) (token string, user User, created bool, err error) {
	// Two first sign-ins with the same identity at once would both try to
	// create it. The loser's insert fails on the primary key and it tries
	// again, now finding the winner's user.
	for attempt := 1; ; attempt++ {
		user, created, err = s.findOrCreate(ctx, provider, subject, email, name)
		if err == nil {
			break
		}
		if !isUniqueViolation(err) || attempt == 2 {
			return "", User{}, false, err
		}
	}

	token, hash, err := newToken()
	if err != nil {
		return "", User{}, false, err
	}
	if _, err := s.db.Exec(ctx, `INSERT INTO sessions (token_hash, user_id) VALUES ($1, $2)`, hash, user.ID); err != nil {
		return "", User{}, false, fmt.Errorf("opening a session: %w", err)
	}
	return token, user, created, nil
}

func (s *Store) findOrCreate(ctx context.Context, provider, subject, email, name string) (User, bool, error) {
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return User{}, false, fmt.Errorf("starting a transaction: %w", err)
	}
	// Rollback after Commit does nothing, so deferring it covers every early return.
	defer tx.Rollback(ctx)

	var userID string
	err = tx.QueryRow(ctx, `SELECT user_id FROM identities WHERE provider = $1 AND subject = $2`, provider, subject).Scan(&userID)
	created := false
	switch {
	case errors.Is(err, pgx.ErrNoRows):
		if err := tx.QueryRow(ctx, `INSERT INTO users (display_name) VALUES ($1) RETURNING id`, name).Scan(&userID); err != nil {
			return User{}, false, fmt.Errorf("creating a user: %w", err)
		}
		if _, err := tx.Exec(ctx,
			`INSERT INTO identities (provider, subject, user_id, email) VALUES ($1, $2, $3, NULLIF($4, ''))`,
			provider, subject, userID, email,
		); err != nil {
			return User{}, false, fmt.Errorf("recording the identity: %w", err)
		}
		created = true
	case err != nil:
		return User{}, false, fmt.Errorf("looking up the identity: %w", err)
	}

	user, err := scanUser(tx.QueryRow(ctx, selectUser+` WHERE id = $1`, userID))
	if err != nil {
		return User{}, false, err
	}
	if err := tx.Commit(ctx); err != nil {
		return User{}, false, fmt.Errorf("committing the sign-in: %w", err)
	}
	return user, created, nil
}

// UserForToken returns the user a session token belongs to, and notes that
// the session was used.
func (s *Store) UserForToken(ctx context.Context, token string) (User, error) {
	user, err := scanUser(s.db.QueryRow(ctx, `
		WITH session AS (
			UPDATE sessions SET last_used_at = now() WHERE token_hash = $1 RETURNING user_id
		)
		SELECT u.id, u.display_name, coalesce(u.handle, ''), u.language
		FROM users u JOIN session ON u.id = session.user_id`,
		hashToken(token),
	))
	if errors.Is(err, ErrNoUser) {
		return User{}, ErrNoSession
	}
	return user, err
}

// SignOut ends a session. Ending one that does not exist is not an error.
func (s *Store) SignOut(ctx context.Context, token string) error {
	if _, err := s.db.Exec(ctx, `DELETE FROM sessions WHERE token_hash = $1`, hashToken(token)); err != nil {
		return fmt.Errorf("ending the session: %w", err)
	}
	return nil
}

// UpdateProfile changes the fields given and returns the user as they now are.
func (s *Store) UpdateProfile(ctx context.Context, userID string, update ProfileUpdate) (User, error) {
	// coalesce keeps the stored value wherever the update passes NULL.
	user, err := scanUser(s.db.QueryRow(ctx, `
		UPDATE users SET
			display_name = coalesce($2, display_name),
			handle       = coalesce($3, handle),
			language     = coalesce($4, language),
			updated_at   = now()
		WHERE id = $1
		RETURNING id, display_name, coalesce(handle, ''), language`,
		userID, update.DisplayName, update.Handle, update.Language,
	))
	if isUniqueViolation(err) {
		return User{}, ErrHandleTaken
	}
	return user, err
}

// DeleteUser deletes a user, and with them their identities and sessions.
func (s *Store) DeleteUser(ctx context.Context, userID string) error {
	tag, err := s.db.Exec(ctx, `DELETE FROM users WHERE id = $1`, userID)
	if err != nil {
		return fmt.Errorf("deleting the user: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNoUser
	}
	return nil
}

const selectUser = `SELECT id, display_name, coalesce(handle, ''), language FROM users`

func scanUser(row pgx.Row) (User, error) {
	var u User
	err := row.Scan(&u.ID, &u.DisplayName, &u.Handle, &u.Language)
	if errors.Is(err, pgx.ErrNoRows) {
		return User{}, ErrNoUser
	}
	if err != nil {
		return User{}, fmt.Errorf("reading a user: %w", err)
	}
	return u, nil
}

// newToken makes a session token: 32 random bytes, which no one can guess,
// written as URL-safe text so it travels in a header unchanged.
func newToken() (token string, hash []byte, err error) {
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", nil, fmt.Errorf("making a session token: %w", err)
	}
	token = base64.RawURLEncoding.EncodeToString(raw)
	return token, hashToken(token), nil
}

func hashToken(token string) []byte {
	sum := sha256.Sum256([]byte(token))
	return sum[:]
}

// isUniqueViolation reports a Postgres unique-constraint failure (SQLSTATE
// 23505), such as a handle someone else already has.
func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}
