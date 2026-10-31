package rooms

import (
	"bytes"
	"context"
	"errors"
	"io"
	"slices"
	"sync/atomic"
	"testing"

	"github.com/PelumiWeb/translator-app-go/server/internal/provider"
	"github.com/PelumiWeb/translator-app-go/server/internal/provider/fake"
)

// countingTranslator is the fake translator, counting its calls.
type countingTranslator struct {
	fake.Translator
	calls atomic.Int32
}

func (c *countingTranslator) Translate(ctx context.Context, text, from, to string) (string, error) {
	c.calls.Add(1)
	return c.Translator.Translate(ctx, text, from, to)
}

// flakySpeaker fails its first attempt, then speaks like the fake engine.
type flakySpeaker struct{ fake.Synthesizer }

func (f flakySpeaker) Synthesize(ctx context.Context, voice io.Reader, opts provider.SynthesisOptions, out io.Writer) error {
	if opts.Attempt == 1 {
		return errors.New("engine busy")
	}
	return f.Synthesizer.Synthesize(ctx, voice, opts, out)
}

// deliveryWorld has Ada (English, with a kept voice) and Rosa (Spanish) in
// a room, and a message from Ada waiting to reach Rosa.
func deliveryWorld(t *testing.T) (world, *Deliverer, Message) {
	t.Helper()
	w := newWorld(t)
	ctx := context.Background()
	ada, rosa := w.user(t, "ada", "en"), w.user(t, "rosa", "es")

	files := &memFiles{}
	// Two seconds of silence: the fake engine wants at least one.
	if err := files.Put("ada-voice.wav", bytes.NewReader(make([]byte, 64_000))); err != nil {
		t.Fatal(err)
	}
	if _, _, err := w.accounts.SetVoice(ctx, ada, "ada-voice.wav", "2026-10-31"); err != nil {
		t.Fatal(err)
	}
	d := &Deliverer{
		Store:      w.rooms,
		Translator: &countingTranslator{},
		Speaker:    fake.Synthesizer{},
		Voices:     w.accounts,
		Files:      files,
		Messages:   &memFiles{},
	}
	return w, d, w.send(t, w.open(t, ada, rosa), ada, "en", "hello")
}

func (w world) rendition(t *testing.T, messageID string) Rendition {
	t.Helper()
	found, err := w.rooms.messagesWhere(context.Background(), `m.id = $1`, messageID)
	if err != nil || len(found) != 1 || len(found[0].Renditions) != 1 {
		t.Fatalf("reading the message: %+v, %v", found, err)
	}
	return found[0].Renditions[0]
}

func TestDeliverTranslatesAndSpeaksInTheSendersVoice(t *testing.T) {
	w, d, m := deliveryWorld(t)

	if err := d.Deliver(context.Background(), m.ID, "es", 1); err != nil {
		t.Fatal(err)
	}
	r := w.rendition(t, m.ID)
	if r.Status != StatusReady || r.Text != "(es) hello" || r.Voice != VoiceOwn || r.AudioKey == "" {
		t.Errorf("rendition = %+v", r)
	}
	if got := d.Messages.(*memFiles).keys(); !slices.Equal(got, []string{r.AudioKey}) {
		t.Errorf("message files = %v, want the speech", got)
	}
}

func TestDeliverFallsBackToThePhonesVoice(t *testing.T) {
	tests := []struct {
		name  string
		setUp func(t *testing.T, w world, d *Deliverer, m Message)
	}{
		{"the sender keeps no voice", func(t *testing.T, w world, d *Deliverer, m Message) {
			if _, err := w.accounts.DeleteVoice(context.Background(), m.SenderID); err != nil {
				t.Fatal(err)
			}
		}},
		{"the server has no voice engine", func(t *testing.T, w world, d *Deliverer, m Message) {
			d.Speaker = nil
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w, d, m := deliveryWorld(t)
			tt.setUp(t, w, d, m)

			if err := d.Deliver(context.Background(), m.ID, "es", 1); err != nil {
				t.Fatal(err)
			}
			if r := w.rendition(t, m.ID); r.Status != StatusReady || r.Text != "(es) hello" || r.Voice != VoicePhone || r.AudioKey != "" {
				t.Errorf("rendition = %+v, want the words for the phone to read", r)
			}
		})
	}
}

func TestDeliverWithoutATranslatorFailsForGood(t *testing.T) {
	_, d, m := deliveryWorld(t)
	d.Translator = nil
	if err := d.Deliver(context.Background(), m.ID, "es", 1); !provider.IsPermanent(err) {
		t.Errorf("err = %v, want a permanent error", err)
	}
}

func TestARetryKeepsTheTranslation(t *testing.T) {
	w, d, m := deliveryWorld(t)
	d.Speaker = flakySpeaker{}
	ctx := context.Background()

	if err := d.Deliver(ctx, m.ID, "es", 1); err == nil {
		t.Fatal("first attempt: want the engine's error")
	}
	if r := w.rendition(t, m.ID); r.Status != StatusSpeaking || r.Text != "(es) hello" {
		t.Errorf("after the first attempt: %+v, want the translation kept, speaking", r)
	}
	if err := d.Deliver(ctx, m.ID, "es", 2); err != nil {
		t.Fatal(err)
	}
	if calls := d.Translator.(*countingTranslator).calls.Load(); calls != 1 {
		t.Errorf("translated %d times, want once", calls)
	}
	if r := w.rendition(t, m.ID); r.Status != StatusReady || r.Voice != VoiceOwn {
		t.Errorf("after the retry: %+v", r)
	}

	// A third run, as after a lost "done", changes nothing.
	if err := d.Deliver(ctx, m.ID, "es", 3); err != nil {
		t.Errorf("delivering a ready message: %v", err)
	}
	if got := d.Messages.(*memFiles).keys(); len(got) != 1 {
		t.Errorf("message files = %v, want one", got)
	}
}

func TestDeliveryFailed(t *testing.T) {
	t.Run("before the translation, the message failed", func(t *testing.T) {
		w, d, m := deliveryWorld(t)
		if err := d.DeliveryFailed(context.Background(), m.ID, "es", "no such language"); err != nil {
			t.Fatal(err)
		}
		if r := w.rendition(t, m.ID); r.Status != StatusFailed || r.Voice != "" {
			t.Errorf("rendition = %+v", r)
		}
	})
	t.Run("after it, the phone reads the words", func(t *testing.T) {
		w, d, m := deliveryWorld(t)
		ctx := context.Background()
		if err := w.rooms.saveTranslation(ctx, m.ID, "es", "hola"); err != nil {
			t.Fatal(err)
		}
		if err := d.DeliveryFailed(ctx, m.ID, "es", "engine down"); err != nil {
			t.Fatal(err)
		}
		if r := w.rendition(t, m.ID); r.Status != StatusReady || r.Voice != VoicePhone || r.Text != "hola" {
			t.Errorf("rendition = %+v", r)
		}
	})
}
