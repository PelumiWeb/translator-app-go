package rooms

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// Blocking (milestone 16, ADR 0035). In a room of two, a block either way
// stops the room: neither member can send into it until the one who blocked
// unblocks. The person blocked is not told; their phone says only that a note
// could not be sent. Group rooms (milestone 20) will need a rule of their own.

// BlockedPerson is someone a user has blocked.
type BlockedPerson struct {
	Person
	BlockedAt time.Time
}

// Block stops blockerID and blockedID sending to each other until blockerID
// unblocks. Blocking someone already blocked changes nothing.
func (s *Store) Block(ctx context.Context, blockerID, blockedID string) error {
	err := pgx.BeginFunc(ctx, s.db, func(tx pgx.Tx) error {
		return block(ctx, tx, blockerID, blockedID)
	})
	if errors.Is(err, ErrNoPerson) {
		return err
	}
	if err != nil {
		return fmt.Errorf("blocking someone: %w", err)
	}
	return nil
}

func block(ctx context.Context, tx pgx.Tx, blockerID, blockedID string) error {
	if blockerID == blockedID {
		return ErrNoPerson
	}
	_, err := tx.Exec(ctx,
		`INSERT INTO blocks (blocker_id, blocked_id) VALUES ($1, $2) ON CONFLICT DO NOTHING`, blockerID, blockedID)
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23503" { // foreign key: no such user
		return ErrNoPerson
	}
	if err != nil {
		return err
	}
	return notifyBlocker(ctx, tx, blockerID, blockedID)
}

// Unblock lifts a block. Lifting one that does not stand is not an error.
func (s *Store) Unblock(ctx context.Context, blockerID, blockedID string) error {
	err := pgx.BeginFunc(ctx, s.db, func(tx pgx.Tx) error {
		tag, err := tx.Exec(ctx, `DELETE FROM blocks WHERE blocker_id = $1 AND blocked_id = $2`, blockerID, blockedID)
		if err != nil || tag.RowsAffected() == 0 {
			return err
		}
		return notifyBlocker(ctx, tx, blockerID, blockedID)
	})
	if err != nil {
		return fmt.Errorf("unblocking someone: %w", err)
	}
	return nil
}

// Blocked lists the people blockerID has blocked, the most recent first.
func (s *Store) Blocked(ctx context.Context, blockerID string) ([]BlockedPerson, error) {
	rows, err := s.db.Query(ctx, `
		SELECT u.id, u.display_name, coalesce(u.handle, ''), u.language, b.created_at
		FROM blocks b JOIN users u ON u.id = b.blocked_id
		WHERE b.blocker_id = $1
		ORDER BY b.created_at DESC, u.id`, blockerID)
	if err != nil {
		return nil, fmt.Errorf("listing blocked people: %w", err)
	}
	found, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (BlockedPerson, error) {
		var p BlockedPerson
		err := row.Scan(&p.ID, &p.DisplayName, &p.Handle, &p.Language, &p.BlockedAt)
		return p, err
	})
	if err != nil {
		return nil, fmt.Errorf("listing blocked people: %w", err)
	}
	return found, nil
}

// checkBlocks refuses a send into a room where a block stands between the
// sender and another member.
func checkBlocks(ctx context.Context, tx pgx.Tx, roomID, senderID string) error {
	var youBlocked, blocked bool
	err := tx.QueryRow(ctx, `
		SELECT coalesce(bool_or(b.blocker_id = $2), false), coalesce(bool_or(b.blocked_id = $2), false)
		FROM room_members o
		JOIN blocks b ON (b.blocker_id = $2 AND b.blocked_id = o.user_id)
		              OR (b.blocked_id = $2 AND b.blocker_id = o.user_id)
		WHERE o.room_id = $1 AND o.user_id <> $2`, roomID, senderID,
	).Scan(&youBlocked, &blocked)
	switch {
	case err != nil:
		return err
	case youBlocked:
		return ErrYouBlocked // told first: unblocking is in their hands
	case blocked:
		return ErrBlocked
	}
	return nil
}

// notifyBlocker tells only the blocker's open streams that their room with
// the other person changed. The other's phones hear nothing of it, so the
// stream does not give the block away.
func notifyBlocker(ctx context.Context, q querier, blockerID, otherID string) error {
	_, err := q.Exec(ctx, `
		SELECT pg_notify($1, json_build_object('room_id', r.id, 'users', json_build_array($2::text))::text)
		FROM rooms r WHERE r.pair = $3`, changesChannel, blockerID, pairOf(blockerID, otherID))
	return err
}
