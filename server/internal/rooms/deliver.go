package rooms

import (
	"bytes"
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"io"

	"github.com/jackc/pgx/v5"

	"github.com/PelumiWeb/translator-app-go/server/internal/accounts"
	"github.com/PelumiWeb/translator-app-go/server/internal/provider"
)

// VoiceKeeper finds the voice a user keeps on the server (ADR 0016).
type VoiceKeeper interface {
	Voice(ctx context.Context, userID string) (accounts.Voice, error)
}

// VoiceFiles opens kept voice recordings.
type VoiceFiles interface {
	Open(key string) (io.ReadSeekCloser, error)
}

// MessageFiles stores the audio of messages.
type MessageFiles interface {
	Put(key string, r io.Reader) error
	Delete(key string) error
}

// Deliverer makes what a message becomes for the listeners of one language:
// the translation, then the translation spoken in the sender's voice. The
// queue's workers run it, one job per message and language, and retry it.
type Deliverer struct {
	Store *Store

	// Translator is nil when the server has no translation service; a
	// message then reaches only the listeners of its own language.
	Translator provider.Translator
	// Speaker is nil when the server has no voice engine; the listeners'
	// phones then read the translations aloud.
	Speaker provider.Synthesizer

	Voices   VoiceKeeper
	Files    VoiceFiles   // where kept voices are stored
	Messages MessageFiles // where message audio is stored
}

// Deliver translates the message and speaks it. A retry picks up where the
// last attempt stopped: a translation already made is not made again.
func (d *Deliverer) Deliver(ctx context.Context, messageID, lang string, attempt int) error {
	work, err := d.Store.delivery(ctx, messageID, lang)
	if errors.Is(err, ErrNoMessage) {
		// Deleted with its sender's account while the job waited.
		return provider.Permanent(err)
	}
	if err != nil {
		return err
	}
	if work.status == StatusReady {
		return nil // an earlier attempt finished; only marking the job done was lost
	}

	text := work.text
	if text == "" {
		if d.Translator == nil {
			return provider.Permanent(errors.New("this server has no translation service"))
		}
		text, err = d.Translator.Translate(ctx, work.transcript, work.lang, lang)
		if err != nil {
			return err
		}
		if err := d.Store.saveTranslation(ctx, messageID, lang, text); err != nil {
			return err
		}
	}

	if d.Speaker == nil {
		return d.Store.ready(ctx, messageID, lang, VoicePhone, "")
	}
	voice, err := d.Voices.Voice(ctx, work.senderID)
	if errors.Is(err, accounts.ErrNoVoice) {
		// They chose not to keep a voice, or deleted it: the listener's
		// phone reads the words instead.
		return d.Store.ready(ctx, messageID, lang, VoicePhone, "")
	}
	if err != nil {
		return fmt.Errorf("finding the sender's voice: %w", err)
	}
	sample, err := d.Files.Open(voice.BlobKey)
	if err != nil {
		return fmt.Errorf("opening the sender's voice: %w", err)
	}
	defer sample.Close()

	var speech bytes.Buffer
	opts := provider.SynthesisOptions{Text: text, Language: lang, Attempt: attempt}
	if err := d.Speaker.Synthesize(ctx, sample, opts, &speech); err != nil {
		return err
	}
	// A new name per attempt, as for synthesis jobs: a retry never
	// overwrites audio an earlier attempt may have recorded.
	key := rand.Text() + ".wav"
	if err := d.Messages.Put(key, &speech); err != nil {
		return fmt.Errorf("storing the speech: %w", err)
	}
	if err := d.Store.ready(ctx, messageID, lang, VoiceOwn, key); err != nil {
		_ = d.Messages.Delete(key) // nothing points at it
		return err
	}
	return nil
}

// DeliveryFailed records that a delivery has failed for good. If the
// translation was made and only the voice failed, the listeners still get the
// words, read by their phone; otherwise the message says it could not be
// translated, and its sender can try again.
func (d *Deliverer) DeliveryFailed(ctx context.Context, messageID, lang, _ string) error {
	return d.Store.failDelivery(ctx, messageID, lang)
}

// deliveryWork is what Deliver needs to know about a message and a language.
type deliveryWork struct {
	senderID   string
	lang       string // what the message was said in
	transcript string
	status     string // of the rendition
	text       string // the translation, if made
}

func (s *Store) delivery(ctx context.Context, messageID, lang string) (deliveryWork, error) {
	var w deliveryWork
	err := s.db.QueryRow(ctx, `
		SELECT m.sender_id, m.lang, m.transcript, r.status, coalesce(r.text, '')
		FROM messages m JOIN renditions r ON r.message_id = m.id AND r.lang = $2
		WHERE m.id = $1`, messageID, lang,
	).Scan(&w.senderID, &w.lang, &w.transcript, &w.status, &w.text)
	if errors.Is(err, pgx.ErrNoRows) {
		return deliveryWork{}, ErrNoMessage
	}
	if err != nil {
		return deliveryWork{}, fmt.Errorf("reading a delivery: %w", err)
	}
	return w, nil
}

func (s *Store) saveTranslation(ctx context.Context, messageID, lang, text string) error {
	return s.updateRendition(ctx, messageID, lang, `
		UPDATE renditions SET status = 'speaking', text = $3, updated_at = now()
		WHERE message_id = $1 AND lang = $2`, text)
}

func (s *Store) ready(ctx context.Context, messageID, lang, voice, audioKey string) error {
	return s.updateRendition(ctx, messageID, lang, `
		UPDATE renditions SET status = 'ready', voice = $3, audio_key = NULLIF($4, ''), updated_at = now()
		WHERE message_id = $1 AND lang = $2`, voice, audioKey)
}

func (s *Store) failDelivery(ctx context.Context, messageID, lang string) error {
	return s.updateRendition(ctx, messageID, lang, `
		UPDATE renditions SET
			status = CASE WHEN text IS NULL THEN 'failed' ELSE 'ready' END,
			voice  = CASE WHEN text IS NULL THEN NULL ELSE 'phone' END,
			updated_at = now()
		WHERE message_id = $1 AND lang = $2 AND status <> 'ready'`)
}

// updateRendition runs update, whose first two parameters are the message
// and the language, and announces the change, together.
func (s *Store) updateRendition(ctx context.Context, messageID, lang, update string, extra ...any) error {
	args := append([]any{messageID, lang}, extra...)
	err := pgx.BeginFunc(ctx, s.db, func(tx pgx.Tx) error {
		tag, err := tx.Exec(ctx, update, args...)
		if err != nil || tag.RowsAffected() == 0 {
			return err
		}
		return notifyMessage(ctx, tx, messageID)
	})
	if err != nil {
		return fmt.Errorf("updating a rendition: %w", err)
	}
	return nil
}
