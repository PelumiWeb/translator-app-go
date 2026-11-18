package rooms

import (
	"context"
	"errors"
	"slices"
	"testing"

	"github.com/PelumiWeb/translator-app-go/server/internal/push"
)

// sendText has sender type transcript into room.
func (w world) sendText(t *testing.T, room, sender, lang, text string) Message {
	t.Helper()
	m, _, err := w.rooms.AddMessage(context.Background(), NewMessage{
		RoomID: room, SenderID: sender, Kind: KindText, Lang: lang, Transcript: text,
	})
	if err != nil {
		t.Fatal(err)
	}
	return m
}

// textWorld is deliveryWorld with a text message from Ada in place of the
// voice note.
func textWorld(t *testing.T) (world, *Deliverer, Message) {
	t.Helper()
	w, d, voiceNote := deliveryWorld(t)
	return w, d, w.sendText(t, voiceNote.RoomID, voiceNote.SenderID, "en", "see you at eight")
}

func TestATextMessageIsDeliveredAsWords(t *testing.T) {
	w, d, m := textWorld(t)
	if m.Kind != KindText || m.AudioKey != "" || m.DurationMS != 0 {
		t.Errorf("message = %+v", m)
	}

	if err := d.Deliver(context.Background(), m.ID, "es", 1); err != nil {
		t.Fatal(err)
	}
	r := w.rendition(t, m.ID)
	if r.Status != StatusReady || r.Text != "(es) see you at eight" || r.Voice != "" || r.AudioKey != "" {
		t.Errorf("rendition = %+v, want the words, with no voice yet", r)
	}
	if got := d.Messages.(*memFiles).keys(); len(got) != 0 {
		t.Errorf("message files = %v, want none: nobody asked to hear it", got)
	}
	if got := w.pushesFor(t, m.ID); !slices.Equal(got, []string{"es"}) {
		t.Errorf("pushes = %v, want one, once the words are ready", got)
	}
}

func TestATextMessageInTheListenersLanguageIsReadyAtOnce(t *testing.T) {
	w := newWorld(t)
	ada, ben := w.user(t, "ada", "en"), w.user(t, "ben", "en")
	m := w.sendText(t, w.open(t, ada, ben), ada, "en", "hello")

	if len(m.Renditions) != 1 || m.Renditions[0] != (Rendition{Lang: "en", Status: StatusReady, Text: "hello"}) {
		t.Errorf("renditions = %+v, want the words, ready, with no voice", m.Renditions)
	}
	if got := w.jobsFor(t, m.ID); len(got) != 0 {
		t.Errorf("deliveries = %v, want none", got)
	}
}

func TestATextMessageIsSpokenWhenAskedFor(t *testing.T) {
	w, d, m := textWorld(t)
	ctx := context.Background()
	if err := d.Deliver(ctx, m.ID, "es", 1); err != nil {
		t.Fatal(err)
	}
	rosa := w.otherMember(t, m)

	asked, err := w.rooms.RequestSpeech(ctx, rosa, m.ID, "es")
	if err != nil {
		t.Fatal(err)
	}
	if r := asked.Renditions[0]; r.Status != StatusSpeaking || r.Text != "(es) see you at eight" {
		t.Errorf("after asking: %+v, want speaking, the words kept", r)
	}
	// Asking again while it is made starts nothing more.
	if _, err := w.rooms.RequestSpeech(ctx, rosa, m.ID, "es"); err != nil {
		t.Fatal(err)
	}
	if got := w.jobsFor(t, m.ID); !slices.Equal(got, []string{"es", "es"}) {
		t.Errorf("deliveries = %v, want the translation and one to speak it", got)
	}

	if err := d.Deliver(ctx, m.ID, "es", 1); err != nil {
		t.Fatal(err)
	}
	r := w.rendition(t, m.ID)
	if r.Status != StatusReady || r.Voice != VoiceOwn || r.AudioKey == "" {
		t.Errorf("rendition = %+v, want it in Ada's voice", r)
	}
	if calls := d.Translator.(*countingTranslator).calls.Load(); calls != 1 {
		t.Errorf("translated %d times, want once", calls)
	}
	if got := w.pushesFor(t, m.ID); len(got) != 1 {
		t.Errorf("pushes = %v, want only the one for the words", got)
	}
	// Once made, the voice is kept: asking again changes nothing.
	if again, err := w.rooms.RequestSpeech(ctx, rosa, m.ID, "es"); err != nil || again.Renditions[0].AudioKey != r.AudioKey {
		t.Errorf("asking again: %+v, %v", again.Renditions, err)
	}
	if got := w.jobsFor(t, m.ID); len(got) != 2 {
		t.Errorf("deliveries = %v, want no more", got)
	}
}

func TestATextMessageFromSomeoneWithNoVoiceIsReadByThePhone(t *testing.T) {
	w, d, m := textWorld(t)
	ctx := context.Background()
	if _, err := w.accounts.DeleteVoice(ctx, m.SenderID); err != nil {
		t.Fatal(err)
	}
	if err := d.Deliver(ctx, m.ID, "es", 1); err != nil {
		t.Fatal(err)
	}
	if _, err := w.rooms.RequestSpeech(ctx, w.otherMember(t, m), m.ID, "es"); err != nil {
		t.Fatal(err)
	}
	if err := d.Deliver(ctx, m.ID, "es", 1); err != nil {
		t.Fatal(err)
	}
	if r := w.rendition(t, m.ID); r.Status != StatusReady || r.Voice != VoicePhone {
		t.Errorf("rendition = %+v, want it for the phone to read", r)
	}
}

func TestASpokenTextThatFailsIsReadByThePhone(t *testing.T) {
	w, d, m := textWorld(t)
	ctx := context.Background()
	if err := d.Deliver(ctx, m.ID, "es", 1); err != nil {
		t.Fatal(err)
	}
	if _, err := w.rooms.RequestSpeech(ctx, w.otherMember(t, m), m.ID, "es"); err != nil {
		t.Fatal(err)
	}
	if err := d.DeliveryFailed(ctx, m.ID, "es", "engine down"); err != nil {
		t.Fatal(err)
	}
	if r := w.rendition(t, m.ID); r.Status != StatusReady || r.Voice != VoicePhone || r.Text == "" {
		t.Errorf("rendition = %+v", r)
	}
	if got := w.pushesFor(t, m.ID); len(got) != 1 {
		t.Errorf("pushes = %v, want only the one for the words", got)
	}
}

func TestRequestSpeechRefuses(t *testing.T) {
	tests := []struct {
		name    string
		message func(t *testing.T, w world, m Message) Message
		who     func(t *testing.T, w world, m Message) string
		want    error
	}{
		{
			"a voice note, already spoken",
			func(t *testing.T, w world, m Message) Message { return w.send(t, m.RoomID, m.SenderID, "en", "a note") },
			func(t *testing.T, w world, m Message) string { return w.otherMember(t, m) },
			ErrNotText,
		},
		{
			"words still being translated",
			func(t *testing.T, w world, m Message) Message { return m },
			func(t *testing.T, w world, m Message) string { return w.otherMember(t, m) },
			ErrNotReady,
		},
		{
			"someone outside the room",
			func(t *testing.T, w world, m Message) Message { return m },
			func(t *testing.T, w world, m Message) string { return w.user(t, "eve", "es") },
			ErrNoMessage,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w, _, text := textWorld(t)
			m := tt.message(t, w, text)
			_, err := w.rooms.RequestSpeech(context.Background(), tt.who(t, w, m), m.ID, "es")
			if !errors.Is(err, tt.want) {
				t.Errorf("err = %v, want %v", err, tt.want)
			}
		})
	}
}

func TestATextMessagesNotificationKeepsItsWordsOut(t *testing.T) {
	w, d, m := textWorld(t)
	pusher := &fakePusher{}
	d.Pusher, d.Phones = pusher, w.accounts
	w.phone(t, "rosa", push.ServiceFCM, "rosa-android")

	if err := d.Push(context.Background(), m.ID, "es"); err != nil {
		t.Fatal(err)
	}
	if n := pusher.last; n.Title != "ada" || n.Body != "Sent a message" {
		t.Errorf("notification = %+v", n)
	}
}

func TestForgetSenderSkipsTextMessages(t *testing.T) {
	w, _, m := textWorld(t)
	keys, err := w.rooms.ForgetSender(context.Background(), m.SenderID)
	if err != nil {
		t.Fatal(err)
	}
	// The voice note's recording from deliveryWorld; nothing for the text.
	if len(keys) != 1 {
		t.Errorf("keys = %v, want only the voice note's", keys)
	}
}

// otherMember is the member of m's room who did not send it.
func (w world) otherMember(t *testing.T, m Message) string {
	t.Helper()
	var id string
	if err := w.db.QueryRow(context.Background(),
		`SELECT user_id FROM room_members WHERE room_id = $1 AND user_id <> $2`, m.RoomID, m.SenderID,
	).Scan(&id); err != nil {
		t.Fatal(err)
	}
	return id
}
