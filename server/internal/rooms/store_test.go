package rooms

import (
	"bytes"
	"context"
	"errors"
	"io"
	"io/fs"
	"slices"
	"sync"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/PelumiWeb/translator-app-go/server/internal/accounts"
	"github.com/PelumiWeb/translator-app-go/server/internal/queue"
	"github.com/PelumiWeb/translator-app-go/server/internal/testdb"
)

// world is a database with the stores rooms works with.
type world struct {
	db       *pgxpool.Pool
	accounts *accounts.Store
	rooms    *Store
}

func newWorld(t *testing.T) world {
	t.Helper()
	db := testdb.New(t)
	return world{db: db, accounts: accounts.NewStore(db), rooms: NewStore(db, queue.NewStore(db))}
}

// user signs up someone who speaks lang and returns their ID.
func (w world) user(t *testing.T, name, lang string) string {
	t.Helper()
	ctx := context.Background()
	_, u, _, err := w.accounts.SignIn(ctx, accounts.ProviderApple, name, "", name)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := w.accounts.UpdateProfile(ctx, u.ID, accounts.ProfileUpdate{Language: &lang}); err != nil {
		t.Fatal(err)
	}
	return u.ID
}

func (w world) open(t *testing.T, a, b string) string {
	t.Helper()
	id, _, err := w.rooms.OpenPair(context.Background(), a, b)
	if err != nil {
		t.Fatal(err)
	}
	return id
}

func (w world) send(t *testing.T, room, sender, lang, transcript string) Message {
	t.Helper()
	m, _, err := w.rooms.AddMessage(context.Background(), NewMessage{
		RoomID: room, SenderID: sender, Lang: lang, Transcript: transcript, AudioKey: transcript + ".wav", DurationMS: 2000,
	})
	if err != nil {
		t.Fatal(err)
	}
	return m
}

func (w world) jobsFor(t *testing.T, messageID string) []string {
	t.Helper()
	rows, err := w.db.Query(context.Background(),
		`SELECT target_lang FROM jobs WHERE message_id = $1 AND kind = 'deliver' ORDER BY target_lang`, messageID)
	if err != nil {
		t.Fatal(err)
	}
	var langs []string
	for rows.Next() {
		var lang string
		if err := rows.Scan(&lang); err != nil {
			t.Fatal(err)
		}
		langs = append(langs, lang)
	}
	return langs
}

func TestOpenPairGivesTwoPeopleOneRoom(t *testing.T) {
	w := newWorld(t)
	ctx := context.Background()
	ada, rosa := w.user(t, "ada", "en"), w.user(t, "rosa", "es")

	first, created, err := w.rooms.OpenPair(ctx, ada, rosa)
	if err != nil || !created {
		t.Fatalf("first OpenPair: created = %v, err = %v", created, err)
	}
	again, created, err := w.rooms.OpenPair(ctx, rosa, ada)
	if err != nil || created || again != first {
		t.Errorf("OpenPair the other way: %q, created = %v, err = %v; want %q", again, created, err, first)
	}

	// Both at once still make one room.
	kenji := w.user(t, "kenji", "ja")
	ids := make([]string, 6)
	var wg sync.WaitGroup
	for i := range ids {
		wg.Go(func() {
			a, b := ada, kenji
			if i%2 == 1 {
				a, b = b, a
			}
			id, _, err := w.rooms.OpenPair(ctx, a, b)
			if err != nil {
				t.Error(err)
			}
			ids[i] = id
		})
	}
	wg.Wait()
	slices.Sort(ids)
	if len(slices.Compact(ids)) != 1 {
		t.Errorf("rooms opened at once: %v, want one", ids)
	}
}

func TestAnEmptyRoomIsListedOnlyForWhoOpenedIt(t *testing.T) {
	w := newWorld(t)
	ctx := context.Background()
	ada, rosa := w.user(t, "ada", "en"), w.user(t, "rosa", "es")
	room := w.open(t, ada, rosa)

	if got, _ := w.rooms.Rooms(ctx, ada); len(got) != 1 || got[0].ID != room || got[0].Last != nil {
		t.Errorf("Ada's rooms = %+v, want the empty room", got)
	}
	if got, _ := w.rooms.Rooms(ctx, rosa); len(got) != 0 {
		t.Errorf("Rosa's rooms = %+v, want none before a message", got)
	}

	w.send(t, room, ada, "en", "hello")
	got, err := w.rooms.Rooms(ctx, rosa)
	if err != nil || len(got) != 1 {
		t.Fatalf("Rosa's rooms = %+v, %v", got, err)
	}
	r := got[0]
	if len(r.People) != 1 || r.People[0].ID != ada || r.People[0].Language != "en" {
		t.Errorf("people = %+v, want Ada alone", r.People)
	}
	if r.Last == nil || r.Last.Transcript != "hello" || r.Unread != 1 {
		t.Errorf("last = %+v, unread = %d", r.Last, r.Unread)
	}

	if err := w.rooms.MarkRead(ctx, rosa, room); err != nil {
		t.Fatal(err)
	}
	if got, _ := w.rooms.Room(ctx, rosa, room); got.Unread != 0 {
		t.Errorf("unread after MarkRead = %d", got.Unread)
	}
	// Your own messages are never unread.
	if got, _ := w.rooms.Room(ctx, ada, room); got.Unread != 0 {
		t.Errorf("Ada's unread = %d", got.Unread)
	}
}

func TestTheNewestRoomComesFirst(t *testing.T) {
	w := newWorld(t)
	ctx := context.Background()
	ada, rosa, kenji := w.user(t, "ada", "en"), w.user(t, "rosa", "es"), w.user(t, "kenji", "ja")
	withRosa, withKenji := w.open(t, ada, rosa), w.open(t, ada, kenji)
	w.send(t, withKenji, kenji, "ja", "konnichiwa")
	w.send(t, withRosa, rosa, "es", "hola")

	got, err := w.rooms.Rooms(ctx, ada)
	if err != nil || len(got) != 2 || got[0].ID != withRosa || got[1].ID != withKenji {
		t.Errorf("rooms = %+v, %v; want Rosa's, then Kenji's", got, err)
	}
}

func TestAMessageGetsARenditionForEachLanguage(t *testing.T) {
	w := newWorld(t)
	ada, rosa, sam := w.user(t, "ada", "en"), w.user(t, "rosa", "es"), w.user(t, "sam", "en")

	toRosa := w.send(t, w.open(t, ada, rosa), ada, "en", "hello")
	want := []Rendition{{Lang: "es", Status: StatusTranslating}}
	if !slices.Equal(toRosa.Renditions, want) {
		t.Errorf("renditions = %+v, want %+v", toRosa.Renditions, want)
	}
	if got := w.jobsFor(t, toRosa.ID); !slices.Equal(got, []string{"es"}) {
		t.Errorf("delivery jobs = %v, want one for es", got)
	}

	// Sam speaks the language it was said in: he hears Ada's recording, at once.
	toSam := w.send(t, w.open(t, ada, sam), ada, "en", "hi Sam")
	want = []Rendition{{Lang: "en", Status: StatusReady, Text: "hi Sam", Voice: VoiceOriginal}}
	if !slices.Equal(toSam.Renditions, want) {
		t.Errorf("renditions = %+v, want %+v", toSam.Renditions, want)
	}
	if got := w.jobsFor(t, toSam.ID); len(got) != 0 {
		t.Errorf("delivery jobs = %v, want none", got)
	}
}

func TestAMessageSentTwiceIsKeptOnce(t *testing.T) {
	w := newWorld(t)
	ctx := context.Background()
	ada, rosa := w.user(t, "ada", "en"), w.user(t, "rosa", "es")
	room := w.open(t, ada, rosa)

	m := NewMessage{RoomID: room, SenderID: ada, Lang: "en", Transcript: "hello", AudioKey: "a.wav", DurationMS: 900, IdempotencyKey: "k1"}
	first, created, err := w.rooms.AddMessage(ctx, m)
	if err != nil || !created {
		t.Fatalf("first: created = %v, err = %v", created, err)
	}
	m.AudioKey = "b.wav"
	again, created, err := w.rooms.AddMessage(ctx, m)
	if err != nil || created || again.ID != first.ID || again.AudioKey != "a.wav" {
		t.Errorf("again: %+v, created = %v, err = %v; want the first message", again, created, err)
	}
	if got := w.jobsFor(t, first.ID); len(got) != 1 {
		t.Errorf("delivery jobs = %v, want one", got)
	}
}

func TestOnlyMembersSendAndRead(t *testing.T) {
	w := newWorld(t)
	ctx := context.Background()
	ada, rosa, eve := w.user(t, "ada", "en"), w.user(t, "rosa", "es"), w.user(t, "eve", "fr")
	room := w.open(t, ada, rosa)
	m := w.send(t, room, ada, "en", "hello")

	if _, _, err := w.rooms.AddMessage(ctx, NewMessage{RoomID: room, SenderID: eve, Lang: "fr", Transcript: "x", AudioKey: "x", DurationMS: 1}); !errors.Is(err, ErrNoRoom) {
		t.Errorf("AddMessage by an outsider: err = %v, want ErrNoRoom", err)
	}
	if _, err := w.rooms.Messages(ctx, eve, room, "", 10); !errors.Is(err, ErrNoRoom) {
		t.Errorf("Messages for an outsider: err = %v, want ErrNoRoom", err)
	}
	if _, err := w.rooms.Message(ctx, eve, m.ID); !errors.Is(err, ErrNoMessage) {
		t.Errorf("Message for an outsider: err = %v, want ErrNoMessage", err)
	}
	if _, err := w.rooms.Room(ctx, eve, room); !errors.Is(err, ErrNoRoom) {
		t.Errorf("Room for an outsider: err = %v, want ErrNoRoom", err)
	}
	if err := w.rooms.MarkRead(ctx, eve, room); !errors.Is(err, ErrNoRoom) {
		t.Errorf("MarkRead by an outsider: err = %v, want ErrNoRoom", err)
	}
}

func TestARoomClosesWhenTheOtherPersonLeaves(t *testing.T) {
	w := newWorld(t)
	ctx := context.Background()
	ada, rosa := w.user(t, "ada", "en"), w.user(t, "rosa", "es")
	room := w.open(t, ada, rosa)
	w.send(t, room, rosa, "es", "hola")

	if err := w.accounts.DeleteUser(ctx, rosa); err != nil {
		t.Fatal(err)
	}
	_, _, err := w.rooms.AddMessage(ctx, NewMessage{RoomID: room, SenderID: ada, Lang: "en", Transcript: "hello?", AudioKey: "a.wav", DurationMS: 1})
	if !errors.Is(err, ErrRoomClosed) {
		t.Errorf("err = %v, want ErrRoomClosed", err)
	}
	// What Rosa said went with her account.
	if got, _ := w.rooms.Room(ctx, ada, room); len(got.People) != 0 || got.Last != nil {
		t.Errorf("room = %+v, want no one else and no messages", got)
	}
}

func TestMessagesPageBackwards(t *testing.T) {
	w := newWorld(t)
	ctx := context.Background()
	ada, rosa := w.user(t, "ada", "en"), w.user(t, "rosa", "es")
	room := w.open(t, ada, rosa)
	for _, text := range []string{"one", "two", "three"} {
		w.send(t, room, ada, "en", text)
	}
	transcripts := func(ms []Message) []string {
		var out []string
		for _, m := range ms {
			out = append(out, m.Transcript)
		}
		return out
	}

	newest, err := w.rooms.Messages(ctx, rosa, room, "", 2)
	if err != nil || !slices.Equal(transcripts(newest), []string{"two", "three"}) {
		t.Fatalf("newest = %v, %v", transcripts(newest), err)
	}
	older, err := w.rooms.Messages(ctx, rosa, room, newest[0].ID, 2)
	if err != nil || !slices.Equal(transcripts(older), []string{"one"}) {
		t.Errorf("older = %v, %v", transcripts(older), err)
	}
}

func TestRetryDeliversAFailedMessageAgain(t *testing.T) {
	w := newWorld(t)
	ctx := context.Background()
	ada, rosa := w.user(t, "ada", "en"), w.user(t, "rosa", "es")
	m := w.send(t, w.open(t, ada, rosa), ada, "en", "hello")
	if err := w.rooms.failDelivery(ctx, m.ID, "es"); err != nil {
		t.Fatal(err)
	}

	if _, err := w.rooms.Retry(ctx, rosa, m.ID); !errors.Is(err, ErrNoMessage) {
		t.Errorf("Retry by the listener: err = %v, want ErrNoMessage", err)
	}
	retried, err := w.rooms.Retry(ctx, ada, m.ID)
	if err != nil || retried.Renditions[0].Status != StatusTranslating {
		t.Fatalf("Retry = %+v, %v", retried, err)
	}
	if got := w.jobsFor(t, m.ID); !slices.Equal(got, []string{"es", "es"}) {
		t.Errorf("delivery jobs = %v, want a second for es", got)
	}
}

func TestForgetSenderReturnsTheirAudio(t *testing.T) {
	w := newWorld(t)
	ctx := context.Background()
	ada, rosa := w.user(t, "ada", "en"), w.user(t, "rosa", "es")
	room := w.open(t, ada, rosa)
	m := w.send(t, room, ada, "en", "hello")
	if err := w.rooms.ready(ctx, m.ID, "es", VoiceOwn, "spoken.wav"); err != nil {
		t.Fatal(err)
	}
	w.send(t, room, rosa, "es", "hola")

	keys, err := w.rooms.ForgetSender(ctx, ada)
	slices.Sort(keys)
	if err != nil || !slices.Equal(keys, []string{"hello.wav", "spoken.wav"}) {
		t.Errorf("ForgetSender = %v, %v", keys, err)
	}
	left, _ := w.rooms.Messages(ctx, rosa, room, "", 10)
	if len(left) != 1 || left[0].Transcript != "hola" {
		t.Errorf("messages left = %+v, want only Rosa's", left)
	}
}

// memFiles keeps files in memory.
type memFiles struct {
	mu    sync.Mutex
	files map[string][]byte
}

func (m *memFiles) Put(key string, r io.Reader) error {
	b, err := io.ReadAll(r)
	if err != nil {
		return err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.files == nil {
		m.files = map[string][]byte{}
	}
	m.files[key] = b
	return nil
}

func (m *memFiles) Open(key string) (io.ReadSeekCloser, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	b, ok := m.files[key]
	if !ok {
		return nil, fs.ErrNotExist
	}
	return nopCloser{bytes.NewReader(b)}, nil
}

func (m *memFiles) Delete(key string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.files, key)
	return nil
}

func (m *memFiles) keys() []string {
	m.mu.Lock()
	defer m.mu.Unlock()
	var keys []string
	for k := range m.files {
		keys = append(keys, k)
	}
	slices.Sort(keys)
	return keys
}

type nopCloser struct{ *bytes.Reader }

func (nopCloser) Close() error { return nil }
