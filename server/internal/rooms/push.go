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
	return d.sendToAll(ctx, work.phones, n)
}

// NotifyReport tells the owner, on every phone they are signed in on, that a
// report came in, so that it is looked at within the day (ADR 0032). It says
// what was reported and why, never the words: the push services would see
// them.
func (d *Deliverer) NotifyReport(ctx context.Context, reportID string) error {
	if d.Pusher == nil || d.ReportsTo == "" {
		return nil
	}
	kind, reason, err := d.Store.reportSummary(ctx, reportID)
	if errors.Is(err, ErrNoReport) {
		return provider.Permanent(err)
	}
	if err != nil {
		return err
	}
	phones, err := d.Store.phonesOf(ctx, d.ReportsTo)
	if err != nil {
		return err
	}
	what := map[string]string{KindVoice: "A voice note", KindText: "A message"}[kind]
	if what == "" {
		what = "Someone"
	}
	n := push.Notification{
		Title:      "New report",
		Body:       what + " · " + reasonWords[reason],
		CollapseID: reportID,
		// No room: tapping it only opens the app. The report is read with
		// make reports-list.
		Data: map[string]string{"report_id": reportID},
	}
	return d.sendToAll(ctx, phones, n)
}

// reasonWords says each reason as the owner's notification shows it.
var reasonWords = map[string]string{
	ReasonSpam:       "spam",
	ReasonHarassment: "harassment",
	ReasonHate:       "hate",
	ReasonSexual:     "sexual content",
	ReasonViolence:   "violence",
	ReasonSelfHarm:   "someone may be at risk",
	ReasonOther:      "another reason",
}

// sendToAll sends n to each phone, forgetting those the push service no
// longer knows. A failure a retry may mend is returned as it is, so the job
// is retried; one it cannot, as permanent.
func (d *Deliverer) sendToAll(ctx context.Context, phones []push.Device, n push.Notification) error {
	var retry, failed []error
	for _, phone := range phones {
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

// reportSummary is what the owner's notification says of a report: whether
// it is about a voice note, a message or a person (kind is then empty), and
// why.
func (s *Store) reportSummary(ctx context.Context, reportID string) (kind, reason string, err error) {
	err = s.db.QueryRow(ctx, `SELECT coalesce(message_kind, ''), reason FROM reports WHERE id = $1`, reportID).Scan(&kind, &reason)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", "", ErrNoReport
	}
	if err != nil {
		return "", "", fmt.Errorf("reading a report: %w", err)
	}
	return kind, reason, nil
}

// phonesOf lists the phones the user with this handle is signed in on.
func (s *Store) phonesOf(ctx context.Context, handle string) ([]push.Device, error) {
	rows, err := s.db.Query(ctx, `
		SELECT p.service, p.token
		FROM users u
		JOIN sessions se ON se.user_id = u.id
		JOIN push_tokens p ON p.session_hash = se.token_hash
		WHERE u.handle = lower($1)`, handle)
	if err != nil {
		return nil, fmt.Errorf("finding the owner's phones: %w", err)
	}
	phones, err := pgx.CollectRows(rows, scanDevice)
	if err != nil {
		return nil, fmt.Errorf("finding the owner's phones: %w", err)
	}
	return phones, nil
}

func scanDevice(row pgx.CollectableRow) (push.Device, error) {
	var d push.Device
	err := row.Scan(&d.Service, &d.Token)
	return d, err
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
	w.phones, err = pgx.CollectRows(rows, scanDevice)
	if err != nil {
		return pushWork{}, fmt.Errorf("finding the listeners' phones: %w", err)
	}
	return w, nil
}
