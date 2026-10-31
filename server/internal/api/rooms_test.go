package api

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/PelumiWeb/translator-app-go/server/internal/accounts"
	"github.com/PelumiWeb/translator-app-go/server/internal/queue"
	"github.com/PelumiWeb/translator-app-go/server/internal/rooms"
	"github.com/PelumiWeb/translator-app-go/server/internal/testdb"
)

// roomsWorld is an API on a real database: rooms are SQL through and
// through, so a fake would test only itself.
type roomsWorld struct {
	api    *API
	tokens map[string]string // by name
}

// newRoomsWorld signs up each person, name to the language they speak, with
// the ID "<name>.id". An empty language leaves the profile unfinished.
func newRoomsWorld(t *testing.T, people map[string]string) roomsWorld {
	t.Helper()
	db := testdb.New(t)
	store := accounts.NewStore(db)
	a := newTestAPI()
	a.Accounts, a.Voices, a.Messages = store, &fakeAudio{}, &fakeAudio{}
	a.Rooms = rooms.NewStore(db, queue.NewStore(db))

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	hub := rooms.NewHub(db, slog.New(slog.NewTextHandler(io.Discard, nil)))
	go hub.Run(ctx)
	<-hub.Ready()
	a.Changes = hub

	w := roomsWorld{api: a, tokens: map[string]string{}}
	for name, lang := range people {
		token, user, _, err := store.SignIn(ctx, accounts.ProviderApple, name, "", name)
		if err != nil {
			t.Fatal(err)
		}
		handle := name + ".id"
		update := accounts.ProfileUpdate{Handle: &handle}
		if lang != "" {
			update.Language = &lang
		}
		if _, err := store.UpdateProfile(ctx, user.ID, update); err != nil {
			t.Fatal(err)
		}
		w.tokens[name] = token
	}
	return w
}

func (w roomsWorld) do(method, path, who, body string) *httptest.ResponseRecorder {
	return send(w.api, method, path, w.tokens[who], body)
}

// open has who open a room with the person named, and returns its ID.
func (w roomsWorld) open(t *testing.T, who, with string) string {
	t.Helper()
	rec := w.do(http.MethodPost, "/v1/rooms", who, `{"handle":"`+with+`.id"}`)
	var room roomResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &room); err != nil || room.ID == "" {
		t.Fatalf("opening a room: status %d, body %s", rec.Code, rec.Body)
	}
	return room.ID
}

// messageRequest builds a multipart message. Empty values leave parts out.
func messageRequest(t *testing.T, path, token, transcript, duration string, audio []byte) *http.Request {
	t.Helper()
	var body bytes.Buffer
	form := multipart.NewWriter(&body)
	for field, value := range map[string]string{"transcript": transcript, "duration_ms": duration} {
		if value != "" {
			if err := form.WriteField(field, value); err != nil {
				t.Fatal(err)
			}
		}
	}
	if audio != nil {
		part, err := form.CreateFormFile("audio", "message.wav")
		if err != nil {
			t.Fatal(err)
		}
		part.Write(audio)
	}
	form.Close()
	req := httptest.NewRequest(http.MethodPost, path, &body)
	req.Header.Set("Content-Type", form.FormDataContentType())
	req.Header.Set("Authorization", "Bearer "+token)
	return req
}

func (w roomsWorld) sendMessage(t *testing.T, room, who, transcript string) messageResponse {
	t.Helper()
	rec := serve(w.api, messageRequest(t, "/v1/rooms/"+room+"/messages", w.tokens[who], transcript, "2400", wavBytes(200)))
	var m messageResponse
	if rec.Code != http.StatusCreated || json.Unmarshal(rec.Body.Bytes(), &m) != nil {
		t.Fatalf("sending: status %d, body %s", rec.Code, rec.Body)
	}
	return m
}

func TestOpenRoom(t *testing.T) {
	tests := []struct {
		name       string
		who, body  string
		wantStatus int
		wantInBody string
	}{
		{"with someone by their ID", "ada", `{"handle":"@Rosa.id"}`, http.StatusCreated, `"handle":"rosa.id","language":"es"`},
		{"with yourself", "ada", `{"handle":"ada.id"}`, http.StatusBadRequest, `"code":"cannot_add_self"`},
		{"with an ID no one has", "ada", `{"handle":"nobody"}`, http.StatusNotFound, `"code":"user_not_found"`},
		{"before choosing your language", "new", `{"handle":"rosa.id"}`, http.StatusConflict, `"code":"profile_incomplete"`},
		{"with someone who has not chosen theirs", "ada", `{"handle":"new.id"}`, http.StatusNotFound, `"code":"user_not_found"`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := newRoomsWorld(t, map[string]string{"ada": "en", "rosa": "es", "new": ""})
			rec := w.do(http.MethodPost, "/v1/rooms", tt.who, tt.body)
			if rec.Code != tt.wantStatus || !strings.Contains(rec.Body.String(), tt.wantInBody) {
				t.Errorf("status %d, body %s; want %d containing %s", rec.Code, rec.Body, tt.wantStatus, tt.wantInBody)
			}
		})
	}
}

func TestOpeningARoomAgainFindsIt(t *testing.T) {
	w := newRoomsWorld(t, map[string]string{"ada": "en", "rosa": "es"})
	first := w.open(t, "ada", "rosa")
	rec := w.do(http.MethodPost, "/v1/rooms", "rosa", `{"handle":"ada.id"}`)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"id":"`+first+`"`) {
		t.Errorf("status %d, body %s; want 200 and the same room", rec.Code, rec.Body)
	}
}

func TestAMessageTravelsFromOneToTheOther(t *testing.T) {
	w := newRoomsWorld(t, map[string]string{"ada": "en", "rosa": "es"})
	room := w.open(t, "ada", "rosa")

	sent := w.sendMessage(t, room, "ada", "  I'm on my way ")
	if !sent.Mine || sent.Transcript != "I'm on my way" || sent.Lang != "en" || sent.DurationMS != 2400 {
		t.Errorf("sent = %+v", sent)
	}
	if len(sent.Renditions) != 1 || sent.Renditions[0] != (renditionResponse{Lang: "es", Status: "translating"}) {
		t.Errorf("renditions = %+v, want Spanish on its way", sent.Renditions)
	}

	rec := w.do(http.MethodGet, "/v1/rooms", "rosa", "")
	var list struct{ Rooms []roomResponse }
	if err := json.Unmarshal(rec.Body.Bytes(), &list); err != nil || len(list.Rooms) != 1 {
		t.Fatalf("Rosa's rooms: status %d, body %s", rec.Code, rec.Body)
	}
	got := list.Rooms[0]
	if got.Unread != 1 || len(got.People) != 1 || got.People[0].DisplayName != "ada" || got.LastMessage == nil || got.LastMessage.Mine {
		t.Errorf("Rosa's room = %+v", got)
	}

	rec = w.do(http.MethodGet, "/v1/rooms/"+room+"/messages", "rosa", "")
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"transcript":"I'm on my way"`) {
		t.Errorf("messages: status %d, body %s", rec.Code, rec.Body)
	}
	if rec := w.do(http.MethodPost, "/v1/rooms/"+room+"/read", "rosa", ""); rec.Code != http.StatusNoContent {
		t.Errorf("read: status %d", rec.Code)
	}
	if rec := w.do(http.MethodGet, "/v1/rooms/"+room, "rosa", ""); !strings.Contains(rec.Body.String(), `"unread":0`) {
		t.Errorf("after reading: %s", rec.Body)
	}
}

func TestAListenerOfTheSameLanguageHearsTheRecording(t *testing.T) {
	w := newRoomsWorld(t, map[string]string{"ada": "en", "sam": "en"})
	room := w.open(t, "ada", "sam")
	sent := w.sendMessage(t, room, "ada", "hello Sam")

	rec := w.do(http.MethodGet, "/v1/messages/"+sent.ID, "sam", "")
	var m messageResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &m); err != nil || len(m.Renditions) != 1 {
		t.Fatalf("status %d, body %s", rec.Code, rec.Body)
	}
	want := renditionResponse{Lang: "en", Status: "ready", Text: "hello Sam", Voice: "original", AudioURL: "/v1/messages/" + sent.ID + "/audio?lang=en"}
	if m.Renditions[0] != want {
		t.Errorf("rendition = %+v, want %+v", m.Renditions[0], want)
	}

	rec = w.do(http.MethodGet, want.AudioURL, "sam", "")
	if rec.Code != http.StatusOK || !bytes.Equal(rec.Body.Bytes(), wavBytes(200)) {
		t.Errorf("audio: status %d, %d bytes", rec.Code, rec.Body.Len())
	}
}

func TestATranslationNotYetSpokenHasNoAudio(t *testing.T) {
	w := newRoomsWorld(t, map[string]string{"ada": "en", "rosa": "es"})
	sent := w.sendMessage(t, w.open(t, "ada", "rosa"), "ada", "hello")
	if rec := w.do(http.MethodGet, "/v1/messages/"+sent.ID+"/audio?lang=es", "rosa", ""); rec.Code != http.StatusNotFound {
		t.Errorf("status %d, body %s; want 404", rec.Code, rec.Body)
	}
	// The sender's own recording is there from the start.
	if rec := w.do(http.MethodGet, "/v1/messages/"+sent.ID+"/audio", "rosa", ""); rec.Code != http.StatusOK {
		t.Errorf("original: status %d", rec.Code)
	}
}

func TestSendMessageRefusesBadInput(t *testing.T) {
	tests := []struct {
		name                 string
		transcript, duration string
		audio                []byte
		wantCode             string
	}{
		{"no transcript", "", "2000", wavBytes(10), "invalid_transcript"},
		{"a transcript too long", strings.Repeat("a", 1001), "2000", wavBytes(10), "invalid_transcript"},
		{"longer than 30 seconds", "hello", "45000", wavBytes(10), "invalid_duration"},
		{"no duration", "hello", "", wavBytes(10), "invalid_duration"},
		{"no audio", "hello", "2000", nil, "missing_audio"},
		{"not a WAV", "hello", "2000", []byte("ID3 an mp3"), "unsupported_audio"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := newRoomsWorld(t, map[string]string{"ada": "en", "rosa": "es"})
			room := w.open(t, "ada", "rosa")
			rec := serve(w.api, messageRequest(t, "/v1/rooms/"+room+"/messages", w.tokens["ada"], tt.transcript, tt.duration, tt.audio))
			if rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), tt.wantCode) {
				t.Errorf("status %d, body %s; want 400 %s", rec.Code, rec.Body, tt.wantCode)
			}
			if kept := w.api.Messages.(*fakeAudio).keys(); len(kept) != 0 {
				t.Errorf("files kept = %v", kept)
			}
		})
	}
}

func TestOutsidersSeeNothing(t *testing.T) {
	w := newRoomsWorld(t, map[string]string{"ada": "en", "rosa": "es", "eve": "fr"})
	room := w.open(t, "ada", "rosa")
	sent := w.sendMessage(t, room, "ada", "a secret")

	for _, path := range []string{"/v1/rooms/" + room, "/v1/rooms/" + room + "/messages", "/v1/messages/" + sent.ID, "/v1/messages/" + sent.ID + "/audio"} {
		if rec := w.do(http.MethodGet, path, "eve", ""); rec.Code != http.StatusNotFound {
			t.Errorf("GET %s by an outsider: status %d", path, rec.Code)
		}
	}
	rec := serve(w.api, messageRequest(t, "/v1/rooms/"+room+"/messages", w.tokens["eve"], "hi", "1000", wavBytes(10)))
	if rec.Code != http.StatusNotFound {
		t.Errorf("sending by an outsider: status %d", rec.Code)
	}
	if rec := w.do(http.MethodPost, "/v1/messages/"+sent.ID+"/retry", "rosa", ""); rec.Code != http.StatusNotFound {
		t.Errorf("retry by the listener: status %d", rec.Code)
	}
	if rec := w.do(http.MethodGet, "/v1/rooms/not-a-uuid", "ada", ""); rec.Code != http.StatusNotFound {
		t.Errorf("a malformed room id: status %d", rec.Code)
	}
}

func TestAMessageSentTwiceWithItsKeyIsKeptOnce(t *testing.T) {
	w := newRoomsWorld(t, map[string]string{"ada": "en", "rosa": "es"})
	room := w.open(t, "ada", "rosa")
	var statuses []int
	for range 2 {
		req := messageRequest(t, "/v1/rooms/"+room+"/messages", w.tokens["ada"], "hello", "1000", wavBytes(10))
		req.Header.Set("Idempotency-Key", "note-1")
		statuses = append(statuses, serve(w.api, req).Code)
	}
	if statuses[0] != http.StatusCreated || statuses[1] != http.StatusOK {
		t.Errorf("statuses = %v, want 201 then 200", statuses)
	}
	if kept := w.api.Messages.(*fakeAudio).keys(); len(kept) != 1 {
		t.Errorf("files kept = %v, want one", kept)
	}
}

func TestDeletingAnAccountDeletesWhatItSaid(t *testing.T) {
	w := newRoomsWorld(t, map[string]string{"ada": "en", "rosa": "es"})
	room := w.open(t, "ada", "rosa")
	w.sendMessage(t, room, "ada", "goodbye")

	if rec := w.do(http.MethodDelete, "/v1/me", "ada", ""); rec.Code != http.StatusNoContent {
		t.Fatalf("delete: status %d, body %s", rec.Code, rec.Body)
	}
	if kept := w.api.Messages.(*fakeAudio).keys(); len(kept) != 0 {
		t.Errorf("files kept = %v, want none", kept)
	}
	rec := w.do(http.MethodGet, "/v1/rooms/"+room, "rosa", "")
	if !strings.Contains(rec.Body.String(), `"people":[],"last_message":null`) {
		t.Errorf("Rosa's room = %s, want no one else and nothing said", rec.Body)
	}
	rec = serve(w.api, messageRequest(t, "/v1/rooms/"+room+"/messages", w.tokens["rosa"], "hola?", "1000", wavBytes(10)))
	if rec.Code != http.StatusConflict || !strings.Contains(rec.Body.String(), "room_closed") {
		t.Errorf("sending to an empty room: status %d, body %s", rec.Code, rec.Body)
	}
}

func TestRoomChangesStreamToTheMembers(t *testing.T) {
	w := newRoomsWorld(t, map[string]string{"ada": "en", "rosa": "es"})
	server := httptest.NewServer(w.api.Handler())
	defer server.Close()

	req, _ := http.NewRequest(http.MethodGet, server.URL+"/v1/me/events", nil)
	req.Header.Set("Authorization", "Bearer "+w.tokens["rosa"])
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK || resp.Header.Get("Content-Type") != "text/event-stream" {
		t.Fatalf("status %d, type %q", resp.StatusCode, resp.Header.Get("Content-Type"))
	}

	lines := make(chan string)
	go func() {
		scanner := bufio.NewScanner(resp.Body)
		for scanner.Scan() {
			lines <- scanner.Text()
		}
		close(lines)
	}()
	next := func() string {
		t.Helper()
		select {
		case line := <-lines:
			return line
		case <-time.After(5 * time.Second):
			t.Fatal("no line within 5s")
			return ""
		}
	}
	if got := next(); got != "event: ready" {
		t.Fatalf("first line = %q, want the ready event", got)
	}
	next() // its data
	next() // the blank line

	room := w.open(t, "ada", "rosa")
	if got := next(); got != "event: room" {
		t.Fatalf("line = %q, want a room event", got)
	}
	if got := next(); got != `data: {"room_id":"`+room+`"}` {
		t.Errorf("data = %q", got)
	}
}
