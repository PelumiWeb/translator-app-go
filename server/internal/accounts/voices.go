package accounts

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
)

// ErrNoVoice means the user has not recorded a voice, or has deleted it.
var ErrNoVoice = errors.New("accounts: no voice recorded")

// Voice is a user's kept voice sample: where its audio is, and their consent.
type Voice struct {
	BlobKey        string
	ConsentVersion string // the version of the consent wording they agreed to
	ConsentedAt    time.Time
}

// SetVoice records a new voice sample for the user, with their consent, and
// returns the blob key of the sample it replaces, or "" if there was none,
// for the caller to delete.
func (s *Store) SetVoice(ctx context.Context, userID, blobKey, consentVersion string) (voice Voice, replaced string, err error) {
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return Voice{}, "", fmt.Errorf("starting a transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	// Locking the user's row makes two uploads at once take turns, so each
	// learns which sample it replaces and none is left behind; it also keeps
	// the user from being deleted halfway. NO KEY UPDATE, the weaker lock,
	// still lets them sign in meanwhile.
	err = tx.QueryRow(ctx, `SELECT 1 FROM users WHERE id = $1 FOR NO KEY UPDATE`, userID).Scan(new(int))
	if errors.Is(err, pgx.ErrNoRows) {
		return Voice{}, "", ErrNoUser
	}
	if err != nil {
		return Voice{}, "", fmt.Errorf("locking the user: %w", err)
	}
	err = tx.QueryRow(ctx, `SELECT blob_key FROM voices WHERE user_id = $1`, userID).Scan(&replaced)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return Voice{}, "", fmt.Errorf("reading the old voice: %w", err)
	}
	err = tx.QueryRow(ctx, `
		INSERT INTO voices (user_id, blob_key, consent_version) VALUES ($1, $2, $3)
		ON CONFLICT (user_id) DO UPDATE SET
			blob_key        = excluded.blob_key,
			consent_version = excluded.consent_version,
			consented_at    = now()
		RETURNING consented_at`,
		userID, blobKey, consentVersion,
	).Scan(&voice.ConsentedAt)
	if err != nil {
		return Voice{}, "", fmt.Errorf("keeping the voice: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return Voice{}, "", fmt.Errorf("committing the voice: %w", err)
	}
	voice.BlobKey, voice.ConsentVersion = blobKey, consentVersion
	return voice, replaced, nil
}

// Voice returns the user's kept voice sample.
func (s *Store) Voice(ctx context.Context, userID string) (Voice, error) {
	var v Voice
	err := s.db.QueryRow(ctx,
		`SELECT blob_key, consent_version, consented_at FROM voices WHERE user_id = $1`, userID,
	).Scan(&v.BlobKey, &v.ConsentVersion, &v.ConsentedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return Voice{}, ErrNoVoice
	}
	if err != nil {
		return Voice{}, fmt.Errorf("reading the voice: %w", err)
	}
	return v, nil
}

// DeleteVoice forgets the user's voice sample and returns its blob key, for
// the caller to delete the audio.
func (s *Store) DeleteVoice(ctx context.Context, userID string) (blobKey string, err error) {
	err = s.db.QueryRow(ctx, `DELETE FROM voices WHERE user_id = $1 RETURNING blob_key`, userID).Scan(&blobKey)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", ErrNoVoice
	}
	if err != nil {
		return "", fmt.Errorf("deleting the voice: %w", err)
	}
	return blobKey, nil
}
