// Package rooms keeps rooms, their members and their voice messages, and
// delivers each message to the people in the room: translated into the
// language each speaks, then spoken in the sender's voice.
package rooms

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Sentinel errors: callers compare with errors.Is instead of matching text.
var (
	// ErrNoRoom is also the answer for a room the user is not in: to them,
	// it does not exist.
	ErrNoRoom    = errors.New("rooms: no such room")
	ErrNoMessage = errors.New("rooms: no such message")
	// ErrRoomClosed means everyone else has left the room, by deleting their
	// account, so a message would reach no one.
	ErrRoomClosed = errors.New("rooms: no one else is in the room")
)

// Where a rendition is, as stored in renditions.status.
const (
	StatusTranslating = "translating"
	StatusSpeaking    = "speaking"
	StatusReady       = "ready"
	StatusFailed      = "failed"
)

// Who speaks a ready rendition, as stored in renditions.voice.
const (
	VoiceOwn      = "own"      // the sender's voice, made for this language
	VoiceOriginal = "original" // the sender's own recording
	VoicePhone    = "phone"    // the listener's phone, reading the text
)

// Person is a member of a room, as the others see them.
type Person struct {
	ID          string
	DisplayName string
	Handle      string
	Language    string
}

// Room is a conversation as one of its members sees it.
type Room struct {
	ID        string
	People    []Person // everyone in it but the member asking
	Last      *Message // the newest message; nil before the first
	Unread    int      // messages from the others the member has not seen
	CreatedAt time.Time
}

// Message is a voice message as it was said, with what it has become for the
// listeners of each language.
type Message struct {
	ID         string
	RoomID     string
	SenderID   string
	Lang       string
	Transcript string
	AudioKey   string
	DurationMS int
	CreatedAt  time.Time
	Renditions []Rendition
}

// Rendition is a message for the listeners of one language.
type Rendition struct {
	Lang     string
	Status   string
	Text     string // the translation; empty until made
	Voice    string // once ready
	AudioKey string // for VoiceOwn
}

// NewMessage is a message to add to a room.
type NewMessage struct {
	RoomID         string
	SenderID       string
	Lang           string
	Transcript     string
	AudioKey       string
	DurationMS     int
	IdempotencyKey string // empty for none
}

// Enqueuer adds the jobs that deliver a message. It runs in the transaction
// that adds the message, so that neither can exist without the other.
type Enqueuer interface {
	EnqueueDeliveries(ctx context.Context, tx pgx.Tx, messageID string, langs []string) error
}

// Store keeps rooms in Postgres.
type Store struct {
	db   *pgxpool.Pool
	jobs Enqueuer
}

func NewStore(db *pgxpool.Pool, jobs Enqueuer) *Store {
	return &Store{db: db, jobs: jobs}
}

// OpenPair returns the room of two for userID and otherID, opening it if
// they have none. The same two people always get the same room.
func (s *Store) OpenPair(ctx context.Context, userID, otherID string) (roomID string, created bool, err error) {
	ids := []string{userID, otherID}
	slices.Sort(ids)
	pair := ids[0] + " " + ids[1]

	err = pgx.BeginFunc(ctx, s.db, func(tx pgx.Tx) error {
		err := tx.QueryRow(ctx, `
			INSERT INTO rooms (pair, created_by) VALUES ($1, $2)
			ON CONFLICT (pair) DO NOTHING
			RETURNING id`,
			pair, userID,
		).Scan(&roomID)
		if errors.Is(err, pgx.ErrNoRows) {
			// They have one. If the other of them was opening it at this
			// very moment, ON CONFLICT waited for that to commit, so this
			// read, a statement of its own, finds it.
			return tx.QueryRow(ctx, `SELECT id FROM rooms WHERE pair = $1`, pair).Scan(&roomID)
		}
		if err != nil {
			return err
		}
		created = true
		if _, err := tx.Exec(ctx,
			`INSERT INTO room_members (room_id, user_id) VALUES ($1, $2), ($1, $3)`, roomID, userID, otherID,
		); err != nil {
			return err
		}
		return notifyRoom(ctx, tx, roomID)
	})
	if err != nil {
		return "", false, fmt.Errorf("opening a room: %w", err)
	}
	return roomID, created, nil
}

// Rooms lists the rooms userID is in, the most recently active first. A room
// with no messages yet is listed only for the member who opened it.
func (s *Store) Rooms(ctx context.Context, userID string) ([]Room, error) {
	return s.rooms(ctx, userID, "")
}

// Room returns one room userID is in, or ErrNoRoom.
func (s *Store) Room(ctx context.Context, userID, roomID string) (Room, error) {
	found, err := s.rooms(ctx, userID, roomID)
	if err != nil {
		return Room{}, err
	}
	if len(found) == 0 {
		return Room{}, ErrNoRoom
	}
	return found[0], nil
}

// rooms lists userID's rooms, or only roomID when it is not empty.
func (s *Store) rooms(ctx context.Context, userID, roomID string) ([]Room, error) {
	rows, err := s.db.Query(ctx, `
		SELECT r.id, r.created_at, coalesce(last.id::text, ''),
		       (SELECT count(*) FROM messages m
		        WHERE m.room_id = r.id AND m.sender_id <> $1 AND m.created_at > me.read_at)
		FROM room_members me
		JOIN rooms r ON r.id = me.room_id
		LEFT JOIN LATERAL (
			SELECT id, created_at FROM messages WHERE room_id = r.id ORDER BY created_at DESC LIMIT 1
		) last ON true
		WHERE me.user_id = $1
		  AND CASE WHEN $2 = '' THEN last.id IS NOT NULL OR r.created_by = $1
		           ELSE r.id::text = $2 END
		ORDER BY coalesce(last.created_at, r.created_at) DESC`,
		userID, roomID,
	)
	if err != nil {
		return nil, fmt.Errorf("listing rooms: %w", err)
	}
	var found []Room
	var lastIDs []string
	lastOf := map[string]string{} // room ID to its last message's ID
	for rows.Next() {
		var room Room
		var lastID string
		if err := rows.Scan(&room.ID, &room.CreatedAt, &lastID, &room.Unread); err != nil {
			rows.Close()
			return nil, fmt.Errorf("listing rooms: %w", err)
		}
		found = append(found, room)
		if lastID != "" {
			lastIDs = append(lastIDs, lastID)
			lastOf[room.ID] = lastID
		}
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("listing rooms: %w", err)
	}
	if len(found) == 0 {
		return nil, nil
	}

	roomIDs := make([]string, len(found))
	for i, room := range found {
		roomIDs[i] = room.ID
	}
	people, err := s.people(ctx, userID, roomIDs)
	if err != nil {
		return nil, err
	}
	last, err := s.messagesWhere(ctx, `m.id = ANY($1)`, lastIDs)
	if err != nil {
		return nil, err
	}
	byID := map[string]*Message{}
	for i := range last {
		byID[last[i].ID] = &last[i]
	}
	for i := range found {
		found[i].People = people[found[i].ID]
		found[i].Last = byID[lastOf[found[i].ID]]
	}
	return found, nil
}

// people returns the members of each room other than userID.
func (s *Store) people(ctx context.Context, userID string, roomIDs []string) (map[string][]Person, error) {
	rows, err := s.db.Query(ctx, `
		SELECT rm.room_id, u.id, u.display_name, coalesce(u.handle, ''), u.language
		FROM room_members rm JOIN users u ON u.id = rm.user_id
		WHERE rm.room_id = ANY($1) AND rm.user_id <> $2
		ORDER BY rm.joined_at, u.id`,
		roomIDs, userID,
	)
	if err != nil {
		return nil, fmt.Errorf("reading room members: %w", err)
	}
	defer rows.Close()
	people := map[string][]Person{}
	for rows.Next() {
		var roomID string
		var p Person
		if err := rows.Scan(&roomID, &p.ID, &p.DisplayName, &p.Handle, &p.Language); err != nil {
			return nil, fmt.Errorf("reading room members: %w", err)
		}
		people[roomID] = append(people[roomID], p)
	}
	return people, rows.Err()
}

// Messages returns up to limit messages of a room userID is in, oldest
// first: the newest ones, or with before, the ones sent before that message.
func (s *Store) Messages(ctx context.Context, userID, roomID, before string, limit int) ([]Message, error) {
	if err := s.checkMember(ctx, userID, roomID); err != nil {
		return nil, err
	}
	var found []Message
	var err error
	if before == "" {
		found, err = s.messagesWhere(ctx, `m.room_id = $1 ORDER BY m.created_at DESC LIMIT $2`, roomID, limit)
	} else {
		found, err = s.messagesWhere(ctx, `m.room_id = $1
			AND m.created_at < (SELECT created_at FROM messages WHERE id = $3 AND room_id = $1)
			ORDER BY m.created_at DESC LIMIT $2`, roomID, limit, before)
	}
	if err != nil {
		return nil, err
	}
	slices.Reverse(found)
	return found, nil
}

// Message returns a message in a room userID is in, or ErrNoMessage.
func (s *Store) Message(ctx context.Context, userID, messageID string) (Message, error) {
	found, err := s.messagesWhere(ctx, `m.id = $1
		AND EXISTS (SELECT 1 FROM room_members WHERE room_id = m.room_id AND user_id = $2)`, messageID, userID)
	if err != nil {
		return Message{}, err
	}
	if len(found) == 0 {
		return Message{}, ErrNoMessage
	}
	return found[0], nil
}

// AddMessage adds a message to a room its sender is in, with a rendition for
// each language the others speak, and the jobs that make them. A message with
// an idempotency key already used by this sender is not added again: the
// earlier one is returned, and created is false.
func (s *Store) AddMessage(ctx context.Context, m NewMessage) (message Message, created bool, err error) {
	var id string
	err = pgx.BeginFunc(ctx, s.db, func(tx pgx.Tx) error {
		var member bool
		if err := tx.QueryRow(ctx,
			`SELECT EXISTS (SELECT 1 FROM room_members WHERE room_id = $1 AND user_id = $2)`, m.RoomID, m.SenderID,
		).Scan(&member); err != nil {
			return err
		}
		if !member {
			return ErrNoRoom
		}
		rows, err := tx.Query(ctx, `
			SELECT DISTINCT u.language FROM room_members rm JOIN users u ON u.id = rm.user_id
			WHERE rm.room_id = $1 AND rm.user_id <> $2 AND u.language <> ''`,
			m.RoomID, m.SenderID,
		)
		if err != nil {
			return err
		}
		langs, err := pgx.CollectRows(rows, pgx.RowTo[string])
		if err != nil {
			return err
		}
		if len(langs) == 0 {
			return ErrRoomClosed
		}

		err = tx.QueryRow(ctx, `
			INSERT INTO messages (room_id, sender_id, lang, transcript, audio_key, duration_ms, idempotency_key)
			VALUES ($1, $2, $3, $4, $5, $6, NULLIF($7, ''))
			ON CONFLICT (sender_id, idempotency_key) WHERE idempotency_key IS NOT NULL DO NOTHING
			RETURNING id`,
			m.RoomID, m.SenderID, m.Lang, m.Transcript, m.AudioKey, m.DurationMS, m.IdempotencyKey,
		).Scan(&id)
		if errors.Is(err, pgx.ErrNoRows) {
			// Sent before. As in OpenPair, a statement of its own sees it.
			return tx.QueryRow(ctx,
				`SELECT id FROM messages WHERE sender_id = $1 AND idempotency_key = $2`, m.SenderID, m.IdempotencyKey,
			).Scan(&id)
		}
		if err != nil {
			return err
		}
		created = true

		// Listeners of the language it was said in hear the sender's own
		// recording; the others wait for a translation.
		var toTranslate []string
		for _, lang := range langs {
			if lang == m.Lang {
				_, err = tx.Exec(ctx, `
					INSERT INTO renditions (message_id, lang, status, text, voice)
					VALUES ($1, $2, 'ready', $3, 'original')`, id, lang, m.Transcript)
			} else {
				toTranslate = append(toTranslate, lang)
				_, err = tx.Exec(ctx,
					`INSERT INTO renditions (message_id, lang, status) VALUES ($1, $2, 'translating')`, id, lang)
			}
			if err != nil {
				return err
			}
		}
		if err := s.jobs.EnqueueDeliveries(ctx, tx, id, toTranslate); err != nil {
			return err
		}
		return notifyMessage(ctx, tx, id)
	})
	if errors.Is(err, ErrNoRoom) || errors.Is(err, ErrRoomClosed) {
		return Message{}, false, err
	}
	if err != nil {
		return Message{}, false, fmt.Errorf("adding a message: %w", err)
	}
	message, err = s.Message(ctx, m.SenderID, id)
	return message, created, err
}

// Retry tries again to deliver the renditions of a message that failed. Only
// its sender may ask.
func (s *Store) Retry(ctx context.Context, userID, messageID string) (Message, error) {
	err := pgx.BeginFunc(ctx, s.db, func(tx pgx.Tx) error {
		var mine bool
		if err := tx.QueryRow(ctx,
			`SELECT EXISTS (SELECT 1 FROM messages WHERE id = $1 AND sender_id = $2)`, messageID, userID,
		).Scan(&mine); err != nil {
			return err
		}
		if !mine {
			return ErrNoMessage
		}
		rows, err := tx.Query(ctx, `
			UPDATE renditions SET status = 'translating', updated_at = now()
			WHERE message_id = $1 AND status = 'failed'
			RETURNING lang`, messageID)
		if err != nil {
			return err
		}
		langs, err := pgx.CollectRows(rows, pgx.RowTo[string])
		if err != nil || len(langs) == 0 {
			return err
		}
		if err := s.jobs.EnqueueDeliveries(ctx, tx, messageID, langs); err != nil {
			return err
		}
		return notifyMessage(ctx, tx, messageID)
	})
	if errors.Is(err, ErrNoMessage) {
		return Message{}, err
	}
	if err != nil {
		return Message{}, fmt.Errorf("retrying a message: %w", err)
	}
	return s.Message(ctx, userID, messageID)
}

// MarkRead notes that userID has seen everything in the room so far.
func (s *Store) MarkRead(ctx context.Context, userID, roomID string) error {
	tag, err := s.db.Exec(ctx,
		`UPDATE room_members SET read_at = now() WHERE room_id = $1 AND user_id = $2`, roomID, userID)
	if err != nil {
		return fmt.Errorf("marking a room read: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNoRoom
	}
	return nil
}

// ForgetSender deletes every message userID sent, before their account is
// deleted, and returns the audio to delete with them: their recordings and
// the copies spoken in their voice.
func (s *Store) ForgetSender(ctx context.Context, userID string) ([]string, error) {
	var keys []string
	err := pgx.BeginFunc(ctx, s.db, func(tx pgx.Tx) error {
		// Read first, delete second: the renditions go with their messages,
		// and one statement could not both delete them and report them.
		rows, err := tx.Query(ctx, `
			SELECT audio_key FROM messages WHERE sender_id = $1
			UNION ALL
			SELECT r.audio_key FROM renditions r JOIN messages m ON m.id = r.message_id
			WHERE m.sender_id = $1 AND r.audio_key IS NOT NULL`, userID)
		if err != nil {
			return err
		}
		if keys, err = pgx.CollectRows(rows, pgx.RowTo[string]); err != nil {
			return err
		}
		_, err = tx.Exec(ctx, `DELETE FROM messages WHERE sender_id = $1`, userID)
		return err
	})
	if err != nil {
		return nil, fmt.Errorf("deleting a sender's messages: %w", err)
	}
	return keys, nil
}

func (s *Store) checkMember(ctx context.Context, userID, roomID string) error {
	var member bool
	err := s.db.QueryRow(ctx,
		`SELECT EXISTS (SELECT 1 FROM room_members WHERE room_id = $1 AND user_id = $2)`, roomID, userID,
	).Scan(&member)
	if err != nil {
		return fmt.Errorf("checking a room member: %w", err)
	}
	if !member {
		return ErrNoRoom
	}
	return nil
}

// messagesWhere reads the messages matching where, which may end in ORDER BY
// and LIMIT, with their renditions.
func (s *Store) messagesWhere(ctx context.Context, where string, args ...any) ([]Message, error) {
	rows, err := s.db.Query(ctx, `
		SELECT m.id, m.room_id, m.sender_id, m.lang, m.transcript, m.audio_key, m.duration_ms, m.created_at
		FROM messages m WHERE `+where, args...)
	if err != nil {
		return nil, fmt.Errorf("reading messages: %w", err)
	}
	found, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (Message, error) {
		var m Message
		err := row.Scan(&m.ID, &m.RoomID, &m.SenderID, &m.Lang, &m.Transcript, &m.AudioKey, &m.DurationMS, &m.CreatedAt)
		return m, err
	})
	if err != nil {
		return nil, fmt.Errorf("reading messages: %w", err)
	}
	if len(found) == 0 {
		return nil, nil
	}

	ids := make([]string, len(found))
	index := map[string]int{}
	for i, m := range found {
		ids[i] = m.ID
		index[m.ID] = i
	}
	rows, err = s.db.Query(ctx, `
		SELECT message_id, lang, status, coalesce(text, ''), coalesce(voice, ''), coalesce(audio_key, '')
		FROM renditions WHERE message_id = ANY($1) ORDER BY lang`, ids)
	if err != nil {
		return nil, fmt.Errorf("reading renditions: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var messageID string
		var r Rendition
		if err := rows.Scan(&messageID, &r.Lang, &r.Status, &r.Text, &r.Voice, &r.AudioKey); err != nil {
			return nil, fmt.Errorf("reading renditions: %w", err)
		}
		i := index[messageID]
		found[i].Renditions = append(found[i].Renditions, r)
	}
	return found, rows.Err()
}

// changesChannel is the Postgres NOTIFY channel on which every change to a
// room is announced, for the Hub to pass on to the phones of its members.
const changesChannel = "room_changes"

// querier is satisfied by *pgxpool.Pool and pgx.Tx alike.
type querier interface {
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
}

// notifyRoom announces that something in a room changed. Inside a
// transaction, Postgres sends the notification only if it commits, so no one
// hears of a change that did not happen.
func notifyRoom(ctx context.Context, q querier, roomID string) error {
	_, err := q.Exec(ctx, `
		SELECT pg_notify($1, json_build_object(
			'room_id', $2::text,
			'users', (SELECT json_agg(user_id) FROM room_members WHERE room_id = $2::uuid)
		)::text)`, changesChannel, roomID)
	return err
}

// notifyMessage announces that a message, or what it became for some
// language, changed.
func notifyMessage(ctx context.Context, q querier, messageID string) error {
	_, err := q.Exec(ctx, `
		SELECT pg_notify($1, json_build_object(
			'room_id', m.room_id,
			'message_id', m.id,
			'users', (SELECT json_agg(user_id) FROM room_members WHERE room_id = m.room_id)
		)::text)
		FROM messages m WHERE m.id = $2`, changesChannel, messageID)
	return err
}
