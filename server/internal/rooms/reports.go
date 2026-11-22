package rooms

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"time"

	"github.com/jackc/pgx/v5"
)

// Reasons a report can give, as stored in reports.reason.
const (
	ReasonSpam       = "spam"
	ReasonHarassment = "harassment"
	ReasonHate       = "hate"
	ReasonSexual     = "sexual"
	ReasonViolence   = "violence"
	ReasonSelfHarm   = "self_harm" // someone may be at risk
	ReasonOther      = "other"
)

var reasons = []string{ReasonSpam, ReasonHarassment, ReasonHate, ReasonSexual, ReasonViolence, ReasonSelfHarm, ReasonOther}

// ValidReason reports whether reason is one a report can give.
func ValidReason(reason string) bool { return slices.Contains(reasons, reason) }

// NewReport is a report about a person, or about one of their messages.
type NewReport struct {
	ReporterID string
	ReportedID string
	Reason     string
	Note       string
	// Message is the reported message as the reporter saw it; nil for a
	// report about the person.
	Message *ReportedMessage
	// Block blocks the reported person as well, in the same transaction.
	Block bool
}

// ReportedMessage is a reported message, copied into the report so that it
// outlives the message itself.
type ReportedMessage struct {
	ID          string
	Kind        string
	Lang        string
	Transcript  string
	Translation string // in the reporter's language; empty when not translated
	AudioKey    string // the report's own copy of the recording; empty for text
}

// Report is a report as the owner reads it.
type Report struct {
	ID         string
	ReporterID string // empty once the reporter's account is deleted
	Reporter   string // "@handle Name", as they are now; empty once deleted
	ReportedID string // empty once the reported person's account is deleted
	Reported   string // "@handle Name", as when the report was made
	Reason     string
	Note       string
	Message    *ReportedMessage // nil for a report about a person
	CreatedAt  time.Time
}

// AddReport records a report, blocks the reported person when asked, and
// queues the notice that tells the owner. People can report only someone they
// share a room with: there is no reporting a stranger whose ID one guessed.
func (s *Store) AddReport(ctx context.Context, r NewReport) (reportID string, err error) {
	m := r.Message
	if m == nil {
		m = &ReportedMessage{}
	}
	err = pgx.BeginFunc(ctx, s.db, func(tx pgx.Tx) error {
		var shared bool
		if err := tx.QueryRow(ctx, `
			SELECT EXISTS (SELECT 1 FROM room_members a JOIN room_members b ON b.room_id = a.room_id
			               WHERE a.user_id = $1 AND b.user_id = $2)`, r.ReporterID, r.ReportedID,
		).Scan(&shared); err != nil {
			return err
		}
		if !shared || r.ReporterID == r.ReportedID {
			return ErrNoPerson
		}
		if err := tx.QueryRow(ctx, `
			INSERT INTO reports (reporter_id, reported_id, reported, reason, note,
			                     message_id, message_kind, message_lang, transcript, translation, audio_key)
			SELECT $1, u.id, trim(coalesce('@' || u.handle, '') || ' ' || u.display_name), $3, $4,
			       NULLIF($5, '')::uuid, NULLIF($6, ''), NULLIF($7, ''), NULLIF($8, ''), NULLIF($9, ''), NULLIF($10, '')
			FROM users u WHERE u.id = $2
			RETURNING id`,
			r.ReporterID, r.ReportedID, r.Reason, r.Note,
			m.ID, m.Kind, m.Lang, m.Transcript, m.Translation, m.AudioKey,
		).Scan(&reportID); err != nil {
			return err
		}
		if r.Block {
			if err := block(ctx, tx, r.ReporterID, r.ReportedID); err != nil {
				return err
			}
		}
		return s.jobs.EnqueueReportNotice(ctx, tx, reportID)
	})
	if errors.Is(err, ErrNoPerson) {
		return "", err
	}
	if err != nil {
		return "", fmt.Errorf("adding a report: %w", err)
	}
	return reportID, nil
}

// OpenReports lists the reports not yet closed, the oldest first: the order
// to answer them in.
func (s *Store) OpenReports(ctx context.Context) ([]Report, error) {
	rows, err := s.db.Query(ctx, `
		SELECT r.id, coalesce(r.reporter_id::text, ''),
		       coalesce(trim(coalesce('@' || u.handle, '') || ' ' || u.display_name), ''),
		       coalesce(r.reported_id::text, ''), r.reported, r.reason, r.note,
		       coalesce(r.message_id::text, ''), coalesce(r.message_kind, ''), coalesce(r.message_lang, ''),
		       coalesce(r.transcript, ''), coalesce(r.translation, ''), coalesce(r.audio_key, ''), r.created_at
		FROM reports r LEFT JOIN users u ON u.id = r.reporter_id
		WHERE r.closed_at IS NULL
		ORDER BY r.created_at, r.id`)
	if err != nil {
		return nil, fmt.Errorf("listing reports: %w", err)
	}
	found, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (Report, error) {
		var r Report
		var m ReportedMessage
		err := row.Scan(&r.ID, &r.ReporterID, &r.Reporter, &r.ReportedID, &r.Reported, &r.Reason, &r.Note,
			&m.ID, &m.Kind, &m.Lang, &m.Transcript, &m.Translation, &m.AudioKey, &r.CreatedAt)
		if m.ID != "" {
			r.Message = &m
		}
		return r, err
	})
	if err != nil {
		return nil, fmt.Errorf("listing reports: %w", err)
	}
	return found, nil
}

// CloseReport closes an open report with what the owner did about it, and
// clears its copy of the reported message: kept only while it was needed. It
// returns the copy of the recording, for the caller to delete.
func (s *Store) CloseReport(ctx context.Context, reportID, outcome string) (audioKey string, err error) {
	// The subquery reads the row as it was, so the copy's key can be returned
	// by the statement that clears it.
	err = s.db.QueryRow(ctx, `
		UPDATE reports r
		SET closed_at = now(), outcome = $2, transcript = NULL, translation = NULL, audio_key = NULL
		FROM (SELECT id, audio_key FROM reports WHERE id = $1 AND closed_at IS NULL FOR UPDATE) prior
		WHERE r.id = prior.id
		RETURNING coalesce(prior.audio_key, '')`, reportID, outcome,
	).Scan(&audioKey)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", ErrNoReport
	}
	if err != nil {
		return "", fmt.Errorf("closing a report: %w", err)
	}
	return audioKey, nil
}
