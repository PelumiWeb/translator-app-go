package api

import (
	"context"
	"encoding/json"
	"net/http"
	"slices"
	"strings"
	"testing"

	"github.com/PelumiWeb/translator-app-go/server/internal/accounts"
)

// id is the user ID of the person named.
func (w roomsWorld) id(t *testing.T, who string) string {
	t.Helper()
	var me userResponse
	if err := json.Unmarshal(w.do(http.MethodGet, "/v1/me", who, "").Body.Bytes(), &me); err != nil || me.ID == "" {
		t.Fatalf("reading %s's profile: %v", who, err)
	}
	return me.ID
}

func (w roomsWorld) room(t *testing.T, room, who string) roomResponse {
	t.Helper()
	var r roomResponse
	if err := json.Unmarshal(w.do(http.MethodGet, "/v1/rooms/"+room, who, "").Body.Bytes(), &r); err != nil {
		t.Fatal(err)
	}
	return r
}

func errorCode(body []byte) string {
	var e errorBody
	_ = json.Unmarshal(body, &e)
	return e.Error.Code
}

func TestBlockingStopsSendingBothWays(t *testing.T) {
	w := newRoomsWorld(t, map[string]string{"ada": "en", "rosa": "es"})
	room := w.open(t, "ada", "rosa")
	rosa := w.id(t, "rosa")

	if rec := w.do(http.MethodPost, "/v1/blocks", "ada", `{"user_id":"`+rosa+`"}`); rec.Code != http.StatusNoContent {
		t.Fatalf("block: status %d, body %s", rec.Code, rec.Body)
	}
	tests := []struct {
		who        string
		wantStatus int
		wantCode   string
	}{
		{"rosa", http.StatusForbidden, "cannot_send"}, // and no more: she is not told why
		{"ada", http.StatusConflict, "you_blocked"},
	}
	for _, tt := range tests {
		rec := serve(w.api, jsonRequest(http.MethodPost, "/v1/rooms/"+room+"/messages", w.tokens[tt.who], `{"text":"hello"}`))
		if rec.Code != tt.wantStatus || errorCode(rec.Body.Bytes()) != tt.wantCode {
			t.Errorf("%s sending: status %d, body %s; want %d %s", tt.who, rec.Code, rec.Body, tt.wantStatus, tt.wantCode)
		}
	}
	if !w.room(t, room, "ada").YouBlocked || w.room(t, room, "rosa").YouBlocked {
		t.Error("you_blocked should be true for ada, who blocked, and false for rosa")
	}

	var list struct {
		Blocked []blockedResponse `json:"blocked"`
	}
	if err := json.Unmarshal(w.do(http.MethodGet, "/v1/me/blocks", "ada", "").Body.Bytes(), &list); err != nil {
		t.Fatal(err)
	}
	if len(list.Blocked) != 1 || list.Blocked[0].ID != rosa || list.Blocked[0].Handle != "rosa.id" || list.Blocked[0].BlockedAt.IsZero() {
		t.Errorf("blocked = %+v, want rosa", list.Blocked)
	}

	if rec := w.do(http.MethodDelete, "/v1/blocks/"+rosa, "ada", ""); rec.Code != http.StatusNoContent {
		t.Fatalf("unblock: status %d, body %s", rec.Code, rec.Body)
	}
	w.sendText(t, room, "rosa", "hola")
	w.sendText(t, room, "ada", "hello")
}

func TestBlockRefusesBadInput(t *testing.T) {
	w := newRoomsWorld(t, map[string]string{"ada": "en"})
	ada := w.id(t, "ada")
	tests := []struct {
		name, body string
		wantStatus int
		wantCode   string
	}{
		{"no id", `{}`, http.StatusBadRequest, "invalid_user_id"},
		{"not an id", `{"user_id":"rosa"}`, http.StatusBadRequest, "invalid_user_id"},
		{"themselves", `{"user_id":"` + ada + `"}`, http.StatusBadRequest, "cannot_block_self"},
		{"no one", `{"user_id":"00000000-0000-0000-0000-000000000000"}`, http.StatusNotFound, "user_not_found"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := w.do(http.MethodPost, "/v1/blocks", "ada", tt.body)
			if rec.Code != tt.wantStatus || errorCode(rec.Body.Bytes()) != tt.wantCode {
				t.Errorf("status %d, body %s; want %d %s", rec.Code, rec.Body, tt.wantStatus, tt.wantCode)
			}
		})
	}
}

func TestReportingAVoiceNoteKeepsACopyAndBlocks(t *testing.T) {
	w := newRoomsWorld(t, map[string]string{"ada": "en", "rosa": "es"})
	room := w.open(t, "ada", "rosa")
	note := w.sendMessage(t, room, "rosa", "tú otra vez")

	rec := w.do(http.MethodPost, "/v1/reports", "ada", `{"message_id":"`+note.ID+`","reason":"harassment","note":"  every day  "}`)
	var created struct {
		ID string `json:"id"`
	}
	if rec.Code != http.StatusCreated || json.Unmarshal(rec.Body.Bytes(), &created) != nil || created.ID == "" {
		t.Fatalf("report: status %d, body %s", rec.Code, rec.Body)
	}

	reports := w.api.Reports.(*fakeAudio).keys()
	if len(reports) != 1 {
		t.Fatalf("report files = %v, want one copy of the recording", reports)
	}
	messages := w.api.Messages.(*fakeAudio)
	if copied, original := reports[0], messages.keys()[0]; copied == original || messages.blobs[original] != w.api.Reports.(*fakeAudio).blobs[copied] {
		t.Errorf("the copy %s should hold the recording %s under a name of its own", copied, original)
	}
	// Reporting blocks too, unless asked not to.
	rec = serve(w.api, jsonRequest(http.MethodPost, "/v1/rooms/"+room+"/messages", w.tokens["rosa"], `{"text":"hola"}`))
	if rec.Code != http.StatusForbidden {
		t.Errorf("rosa sending after the report: status %d, want 403", rec.Code)
	}
}

func TestReportingWithoutBlocking(t *testing.T) {
	w := newRoomsWorld(t, map[string]string{"ada": "en", "rosa": "es"})
	room := w.open(t, "ada", "rosa")
	w.sendText(t, room, "rosa", "hola")
	rosa := w.id(t, "rosa")

	rec := w.do(http.MethodPost, "/v1/reports", "ada", `{"user_id":"`+rosa+`","reason":"spam","block":false}`)
	if rec.Code != http.StatusCreated {
		t.Fatalf("report: status %d, body %s", rec.Code, rec.Body)
	}
	if keys := w.api.Reports.(*fakeAudio).keys(); len(keys) != 0 {
		t.Errorf("report files = %v, want none for a report about a person", keys)
	}
	w.sendText(t, room, "rosa", "hola otra vez")
}

func TestReportRefusesBadInput(t *testing.T) {
	w := newRoomsWorld(t, map[string]string{"ada": "en", "rosa": "es", "kenji": "ja"})
	room := w.open(t, "ada", "rosa")
	mine := w.sendText(t, room, "ada", "hello")
	theirs := w.sendText(t, room, "rosa", "hola")
	ada, kenji := w.id(t, "ada"), w.id(t, "kenji")

	tests := []struct {
		name, body string
		wantStatus int
		wantCode   string
	}{
		{"no reason", `{"message_id":"` + theirs.ID + `"}`, http.StatusBadRequest, "invalid_reason"},
		{"an unknown reason", `{"message_id":"` + theirs.ID + `","reason":"rude"}`, http.StatusBadRequest, "invalid_reason"},
		{"a note too long", `{"message_id":"` + theirs.ID + `","reason":"other","note":"` + strings.Repeat("é", 501) + `"}`, http.StatusBadRequest, "invalid_note"},
		{"nothing to report", `{"reason":"spam"}`, http.StatusBadRequest, "invalid_request"},
		{"their own message", `{"message_id":"` + mine.ID + `","reason":"spam"}`, http.StatusBadRequest, "cannot_report_self"},
		{"themselves", `{"user_id":"` + ada + `","reason":"spam"}`, http.StatusBadRequest, "cannot_report_self"},
		{"a message and someone else", `{"message_id":"` + theirs.ID + `","user_id":"` + kenji + `","reason":"spam"}`, http.StatusBadRequest, "invalid_request"},
		{"a message they cannot see", `{"message_id":"00000000-0000-0000-0000-000000000000","reason":"spam"}`, http.StatusNotFound, "message_not_found"},
		{"a stranger", `{"user_id":"` + kenji + `","reason":"spam"}`, http.StatusNotFound, "user_not_found"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := w.do(http.MethodPost, "/v1/reports", "ada", tt.body)
			if rec.Code != tt.wantStatus || errorCode(rec.Body.Bytes()) != tt.wantCode {
				t.Errorf("status %d, body %s; want %d %s", rec.Code, rec.Body, tt.wantStatus, tt.wantCode)
			}
		})
	}
	// None of them blocked anyone.
	w.sendText(t, room, "rosa", "sigo aquí")
}

func TestASuspendedAccountCannotSignIn(t *testing.T) {
	w := newRoomsWorld(t, map[string]string{"rosa": "es"})
	w.api.Apple = fakeVerifier{subject: "rosa"} // the subject the world signed her up with
	if err := w.api.Accounts.(*accounts.Store).Suspend(context.Background(), w.id(t, "rosa")); err != nil {
		t.Fatal(err)
	}

	rec := send(w.api, http.MethodPost, "/v1/sessions/apple", "", `{"identity_token":"good","nonce":"n"}`)
	if rec.Code != http.StatusForbidden || errorCode(rec.Body.Bytes()) != "account_suspended" {
		t.Errorf("sign-in: status %d, body %s; want 403 account_suspended", rec.Code, rec.Body)
	}
	if rec := w.do(http.MethodGet, "/v1/me", "rosa", ""); rec.Code != http.StatusUnauthorized {
		t.Errorf("the old session: status %d, want 401", rec.Code)
	}
}

func TestDeletingAnAccountLeavesReportsAboutIt(t *testing.T) {
	w := newRoomsWorld(t, map[string]string{"ada": "en", "rosa": "es"})
	room := w.open(t, "ada", "rosa")
	note := w.sendMessage(t, room, "rosa", "tú otra vez")
	if rec := w.do(http.MethodPost, "/v1/reports", "ada", `{"message_id":"`+note.ID+`","reason":"harassment"}`); rec.Code != http.StatusCreated {
		t.Fatalf("report: status %d, body %s", rec.Code, rec.Body)
	}
	copies := w.api.Reports.(*fakeAudio).keys()

	if rec := w.do(http.MethodDelete, "/v1/me", "rosa", ""); rec.Code != http.StatusNoContent {
		t.Fatalf("deleting: status %d, body %s", rec.Code, rec.Body)
	}
	if got := w.api.Messages.(*fakeAudio).keys(); len(got) != 0 {
		t.Errorf("message files = %v, want hers gone", got)
	}
	if got := w.api.Reports.(*fakeAudio).keys(); !slices.Equal(got, copies) {
		t.Errorf("report files = %v, want the copy %v kept until the report is closed", got, copies)
	}
}
