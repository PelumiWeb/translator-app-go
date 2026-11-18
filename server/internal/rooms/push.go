package rooms

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"

	"github.com/PelumiWeb/translator-app-go/server/internal/provider"
	"github.com/PelumiWeb/translator-app-go/server/internal/push"
)

// Pusher sends a notification to one phone.
type Pusher interface {
	Push(ctx context.Context, to push.Device, n push.Notification) error
}

// PhoneBook forgets the address of a phone the push service no longer knows.
type PhoneBook interface {
	ForgetPushToken(ctx context.Context, pushToken string) error
}

// Push tells the listeners of one language, on every phone they are signed
// in on, that a message is ready for them.
//
// A retry sends to every phone again, also to those that had it: each
// notification carries the message's ID, which makes a repeat replace the
// first rather than show twice.
func (d *Deliverer) Push(ctx context.Context, messageID, lang string) error {
	if d.Pusher == nil {
		return nil
	}
	work, err := d.Store.pushWork(ctx, messageID, lang)
	if errors.Is(err, ErrNoMessage) {
		return provider.Permanent(err) // deleted with its sender's account
	}
	if err != nil {
		return err
	}

	// What was sent, never the words: the push services would see them.
	body := "Sent a voice note · " + clock(work.durationMS)
	if work.kind == KindText {
		body = "Sent a message"
	}
	n := push.Notification{
		Title:      work.sender,
		Body:       body,
		Thread:     work.roomID,
		CollapseID: messageID,
		Data:       map[string]string{"room_id": work.roomID, "message_id": messageID},
	}
	var retry, failed []error
	for _, phone := range work.phones {
		err := d.Pusher.Push(ctx, phone, n)
		switch {
		case err == nil:
		case errors.Is(err, push.ErrGone):
			if err := d.Phones.ForgetPushToken(ctx, phone.Token); err != nil {
				retry = append(retry, err)
			}
		case provider.IsPermanent(err):
			failed = append(failed, err)
		default:
			retry = append(retry, err)
		}
	}
	if len(retry) > 0 {
		return errors.Join(append(retry, failed...)...)
	}
	if len(failed) > 0 {
		// Only failures no retry can mend: the job fails, and the log says why.
		return provider.Permanent(errors.Join(failed...))
	}
	return nil
}

// clock writes a duration as a voice note's length is shown: 0:14, 1:05.
func clock(ms int) string {
	seconds := (ms + 500) / 1000
	return fmt.Sprintf("%d:%02d", seconds/60, seconds%60)
}

// pushWork is what Push needs: who sent the message, and the phones of the
// listeners of one language.
type pushWork struct {
	roomID     string
	sender     string // their name, or @handle if they have none
	kind       string
	durationMS int
	phones     []push.Device
}

func (s *Store) pushWork(ctx context.Context, messageID, lang string) (pushWork, error) {
	var w pushWork
	err := s.db.QueryRow(ctx, `
		SELECT m.room_id, coalesce(nullif(u.display_name, ''), '@' || u.handle, 'Someone'), m.kind, m.duration_ms
		FROM messages m JOIN users u ON u.id = m.sender_id
		WHERE m.id = $1`, messageID,
	).Scan(&w.roomID, &w.sender, &w.kind, &w.durationMS)
	if errors.Is(err, pgx.ErrNoRows) {
		return pushWork{}, ErrNoMessage
	}
	if err != nil {
		return pushWork{}, fmt.Errorf("reading a message to push: %w", err)
	}

	rows, err := s.db.Query(ctx, `
		SELECT p.service, p.token
		FROM messages m
		JOIN room_members rm ON rm.room_id = m.room_id AND rm.user_id <> m.sender_id
		JOIN users u ON u.id = rm.user_id AND u.language = $2
		JOIN sessions se ON se.user_id = rm.user_id
		JOIN push_tokens p ON p.session_hash = se.token_hash
		WHERE m.id = $1`, messageID, lang)
	if err != nil {
		return pushWork{}, fmt.Errorf("finding the listeners' phones: %w", err)
	}
	w.phones, err = pgx.CollectRows(rows, func(row pgx.CollectableRow) (push.Device, error) {
		var d push.Device
		err := row.Scan(&d.Service, &d.Token)
		return d, err
	})
	if err != nil {
		return pushWork{}, fmt.Errorf("finding the listeners' phones: %w", err)
	}
	return w, nil
}
