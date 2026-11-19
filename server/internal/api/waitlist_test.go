package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"testing"

	"github.com/PelumiWeb/translator-app-go/server/internal/waitlist"
)

type fakeWaitlist struct {
	joined []waitlist.Entry
	err    error
}

func (f *fakeWaitlist) Join(_ context.Context, e waitlist.Entry) error {
	if f.err != nil {
		return f.err
	}
	f.joined = append(f.joined, e)
	return nil
}

func TestJoinWaitlist(t *testing.T) {
	tests := []struct {
		name       string
		body       string
		storeErr   error
		wantStatus int
		wantCode   string
		wantKept   *waitlist.Entry
	}{
		{
			name:       "an iPhone, with a name",
			body:       `{"email":" ada@example.com ","name":"  Ada ","phone":"iphone"}`,
			wantStatus: http.StatusOK,
			wantKept:   &waitlist.Entry{Email: "ada@example.com", Name: "Ada", Phone: "iphone"},
		},
		{
			name:       "an Android phone, no name",
			body:       `{"email":"kenji@example.co.jp","phone":"android"}`,
			wantStatus: http.StatusOK,
			wantKept:   &waitlist.Entry{Email: "kenji@example.co.jp", Phone: "android"},
		},
		{
			name:       "a name longer than a profile's is cut",
			body:       `{"email":"ada@example.com","name":"` + strings.Repeat("a", 80) + `","phone":"iphone"}`,
			wantStatus: http.StatusOK,
			wantKept:   &waitlist.Entry{Email: "ada@example.com", Name: strings.Repeat("a", maxNameChars), Phone: "iphone"},
		},
		{
			name:       "the field hidden from people is filled in: answered, not kept",
			body:       `{"email":"bot@example.com","phone":"iphone","company":"Acme"}`,
			wantStatus: http.StatusOK,
		},
		{name: "no address", body: `{"phone":"iphone"}`, wantStatus: http.StatusBadRequest, wantCode: "invalid_email"},
		{name: "not an address", body: `{"email":"ada","phone":"iphone"}`, wantStatus: http.StatusBadRequest, wantCode: "invalid_email"},
		{name: "no dot in the domain", body: `{"email":"ada@localhost","phone":"iphone"}`, wantStatus: http.StatusBadRequest, wantCode: "invalid_email"},
		{name: "a display name", body: `{"email":"Ada <ada@example.com>","phone":"iphone"}`, wantStatus: http.StatusBadRequest, wantCode: "invalid_email"},
		{name: "two addresses", body: `{"email":"ada@example.com, rosa@example.com","phone":"iphone"}`, wantStatus: http.StatusBadRequest, wantCode: "invalid_email"},
		{name: "too long", body: `{"email":"` + strings.Repeat("a", 250) + `@example.com","phone":"iphone"}`, wantStatus: http.StatusBadRequest, wantCode: "invalid_email"},
		{name: "no phone", body: `{"email":"ada@example.com"}`, wantStatus: http.StatusBadRequest, wantCode: "invalid_phone"},
		{name: "another phone", body: `{"email":"ada@example.com","phone":"blackberry"}`, wantStatus: http.StatusBadRequest, wantCode: "invalid_phone"},
		{name: "not JSON", body: `email=ada@example.com`, wantStatus: http.StatusBadRequest, wantCode: "invalid_request"},
		{
			name:       "the database fails",
			body:       `{"email":"ada@example.com","phone":"iphone"}`,
			storeErr:   errors.New("connection refused"),
			wantStatus: http.StatusInternalServerError,
			wantCode:   "internal",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := newTestAPI()
			store := &fakeWaitlist{err: tt.storeErr}
			a.Waitlist = store

			// No session: the website's visitors have no account.
			rec := send(a, http.MethodPost, "/v1/waitlist", "", tt.body)

			if rec.Code != tt.wantStatus {
				t.Fatalf("status %d, want %d; body %s", rec.Code, tt.wantStatus, rec.Body)
			}
			if tt.wantCode != "" {
				var body errorBody
				if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil || body.Error.Code != tt.wantCode {
					t.Errorf("body %s, want code %q", rec.Body, tt.wantCode)
				}
			}
			switch {
			case tt.wantKept == nil && len(store.joined) != 0:
				t.Errorf("kept %+v, want nothing", store.joined)
			case tt.wantKept != nil && (len(store.joined) != 1 || store.joined[0] != *tt.wantKept):
				t.Errorf("kept %+v, want %+v", store.joined, *tt.wantKept)
			}
		})
	}
}
