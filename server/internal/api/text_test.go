package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// jsonRequest is a request with a JSON body, as the phones send a text message.
func jsonRequest(method, path, token, body string) *http.Request {
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json; charset=utf-8")
	req.Header.Set("Authorization", "Bearer "+token)
	return req
}

func (w roomsWorld) sendText(t *testing.T, room, who, text string) messageResponse {
	t.Helper()
	rec := serve(w.api, jsonRequest(http.MethodPost, "/v1/rooms/"+room+"/messages", w.tokens[who], `{"text":`+quote(text)+`}`))
	var m messageResponse
	if rec.Code != http.StatusCreated || json.Unmarshal(rec.Body.Bytes(), &m) != nil {
		t.Fatalf("sending: status %d, body %s", rec.Code, rec.Body)
	}
	return m
}

func quote(s string) string {
	b, _ := json.Marshal(s)
	return string(b)
}

func TestATextMessageTravelsFromOneToTheOther(t *testing.T) {
	w := newRoomsWorld(t, map[string]string{"ada": "en", "rosa": "es"})
	room := w.open(t, "ada", "rosa")

	sent := w.sendText(t, room, "ada", "  see you at eight ")
	if !sent.Mine || sent.Kind != "text" || sent.Transcript != "see you at eight" || sent.Lang != "en" || sent.DurationMS != 0 {
		t.Errorf("sent = %+v", sent)
	}
	if len(sent.Renditions) != 1 || sent.Renditions[0] != (renditionResponse{Lang: "es", Status: "translating"}) {
		t.Errorf("renditions = %+v, want Spanish on its way", sent.Renditions)
	}
	rec := w.do(http.MethodGet, "/v1/rooms/"+room+"/messages", "rosa", "")
	if !strings.Contains(rec.Body.String(), `"kind":"text"`) {
		t.Errorf("Rosa's messages: %s", rec.Body)
	}
	// Voice notes say what they are too.
	if note := w.sendMessage(t, room, "ada", "and a note"); note.Kind != "voice" {
		t.Errorf("a voice note's kind = %q", note.Kind)
	}
}

func TestSendTextRefusesBadInput(t *testing.T) {
	tests := []struct {
		name, body, wantCode string
	}{
		{"no text", `{"text":"   "}`, "invalid_text"},
		{"text too long", `{"text":"` + strings.Repeat("a", 1001) + `"}`, "invalid_text"},
		{"a made-up language", `{"text":"hello","lang":"not a language"}`, "invalid_lang"},
		{"not JSON", `text=hello`, "invalid_request"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := newRoomsWorld(t, map[string]string{"ada": "en", "rosa": "es"})
			room := w.open(t, "ada", "rosa")
			req := jsonRequest(http.MethodPost, "/v1/rooms/"+room+"/messages", w.tokens["ada"], tt.body)
			rec := serve(w.api, req)
			if rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), tt.wantCode) {
				t.Errorf("status %d, body %s; want 400 %s", rec.Code, rec.Body, tt.wantCode)
			}
		})
	}
}

func TestATextMessageSentTwiceWithItsKeyIsKeptOnce(t *testing.T) {
	w := newRoomsWorld(t, map[string]string{"ada": "en", "rosa": "es"})
	room := w.open(t, "ada", "rosa")
	var statuses []int
	for range 2 {
		req := jsonRequest(http.MethodPost, "/v1/rooms/"+room+"/messages", w.tokens["ada"], `{"text":"hello"}`)
		req.Header.Set("Idempotency-Key", "text-1")
		statuses = append(statuses, serve(w.api, req).Code)
	}
	if statuses[0] != http.StatusCreated || statuses[1] != http.StatusOK {
		t.Errorf("statuses = %v, want 201 then 200", statuses)
	}
}

func TestRequestSpeech(t *testing.T) {
	w := newRoomsWorld(t, map[string]string{"ada": "en", "sam": "en", "eve": "fr"})
	room := w.open(t, "ada", "sam")
	text := w.sendText(t, room, "ada", "hello Sam")
	note := w.sendMessage(t, room, "ada", "a voice note")
	speech := func(id, lang string) string { return "/v1/messages/" + id + "/speech?lang=" + lang }

	tests := []struct {
		name, who, path string
		wantStatus      int
		wantInBody      string
	}{
		// Sam reads Ada in English: the words are ready at once, so the voice can be asked for.
		{"the listener, in their language", "sam", speech(text.ID, "en"), http.StatusOK, `"status":"speaking"`},
		{"again, while it is made", "sam", speech(text.ID, "en"), http.StatusOK, `"status":"speaking"`},
		{"the sender, in the listener's language", "ada", speech(text.ID, "en"), http.StatusOK, `"status":"speaking"`},
		{"a language nobody here reads", "ada", speech(text.ID, "fr"), http.StatusNotFound, `"no_rendition"`},
		{"a voice note", "sam", speech(note.ID, "en"), http.StatusConflict, `"not_text"`},
		{"someone outside the room", "eve", speech(text.ID, "en"), http.StatusNotFound, `"message_not_found"`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := w.do(http.MethodPost, tt.path, tt.who, "")
			if rec.Code != tt.wantStatus || !strings.Contains(rec.Body.String(), tt.wantInBody) {
				t.Errorf("status %d, body %s; want %d with %s", rec.Code, rec.Body, tt.wantStatus, tt.wantInBody)
			}
		})
	}
}

func TestRequestSpeechBeforeTheWordsAreReady(t *testing.T) {
	w := newRoomsWorld(t, map[string]string{"ada": "en", "rosa": "es"})
	text := w.sendText(t, w.open(t, "ada", "rosa"), "ada", "hello")

	rec := w.do(http.MethodPost, "/v1/messages/"+text.ID+"/speech?lang=es", "rosa", "")

	if rec.Code != http.StatusConflict || !strings.Contains(rec.Body.String(), `"not_ready"`) {
		t.Errorf("status %d, body %s; want 409 not_ready", rec.Code, rec.Body)
	}
}

func TestDeletingAnAccountWithTextMessages(t *testing.T) {
	w := newRoomsWorld(t, map[string]string{"ada": "en", "rosa": "es"})
	room := w.open(t, "ada", "rosa")
	w.sendText(t, room, "ada", "goodbye")
	w.sendMessage(t, room, "ada", "and a note")

	if rec := w.do(http.MethodDelete, "/v1/me", "ada", ""); rec.Code != http.StatusNoContent {
		t.Fatalf("delete: status %d, body %s", rec.Code, rec.Body)
	}
	if kept := w.api.Messages.(*fakeAudio).keys(); len(kept) != 0 {
		t.Errorf("files kept = %v, want none", kept)
	}
}
