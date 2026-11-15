package accounts

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
)

// SetPushToken records where the phone signed in with this session receives
// notifications: its address at a push service (APNs or FCM). It replaces any
// address the session gave before. An address another session held moves to
// this one: it is the same phone, signed in again.
func (s *Store) SetPushToken(ctx context.Context, sessionToken, service, pushToken string) error {
	hash := hashToken(sessionToken)
	err := pgx.BeginFunc(ctx, s.db, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx,
			`DELETE FROM push_tokens WHERE token = $1 AND session_hash <> $2`, pushToken, hash,
		); err != nil {
			return err
		}
		tag, err := tx.Exec(ctx, `
			INSERT INTO push_tokens (session_hash, service, token)
			SELECT token_hash, $2, $3 FROM sessions WHERE token_hash = $1
			ON CONFLICT (session_hash) DO UPDATE SET
				service    = excluded.service,
				token      = excluded.token,
				updated_at = now()`,
			hash, service, pushToken)
		if err != nil {
			return err
		}
		if tag.RowsAffected() == 0 {
			return ErrNoSession // signed out meanwhile
		}
		return nil
	})
	if errors.Is(err, ErrNoSession) {
		return err
	}
	if err != nil {
		return fmt.Errorf("keeping a push token: %w", err)
	}
	return nil
}

// ForgetPushToken drops an address the push service no longer knows: the
// app was deleted from the phone, or the phone was given a new address.
func (s *Store) ForgetPushToken(ctx context.Context, pushToken string) error {
	if _, err := s.db.Exec(ctx, `DELETE FROM push_tokens WHERE token = $1`, pushToken); err != nil {
		return fmt.Errorf("forgetting a push token: %w", err)
	}
	return nil
}
