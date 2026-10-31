package api

import (
	"context"
	"net/http"
	"strings"
	"testing"

	"github.com/PelumiWeb/translator-app-go/server/internal/accounts"
)

// peopleAPI has a signed-in user and Rosa, who has an ID and a language.
func peopleAPI(t *testing.T) (*API, string) {
	t.Helper()
	a, store := accountsAPI()
	ctx := context.Background()
	token, _, _, _ := store.SignIn(ctx, accounts.ProviderApple, "apple-1", "", "Ada")
	_, rosa, _, _ := store.SignIn(ctx, accounts.ProviderApple, "rosa", "", "Rosa López")
	if _, err := store.UpdateProfile(ctx, rosa.ID, accounts.ProfileUpdate{Handle: ptr("rosa.lm"), Language: ptr("es")}); err != nil {
		t.Fatal(err)
	}
	return a, token
}

func ptr(s string) *string { return &s }

func TestCheckHandle(t *testing.T) {
	tests := []struct {
		name       string
		path       string
		wantStatus int
		wantInBody string
	}{
		{"free", "/v1/handles/ada", http.StatusOK, `{"handle":"ada","available":true,"suggestions":[]}`},
		{"compared in lower case", "/v1/handles/ADA", http.StatusOK, `"handle":"ada","available":true`},
		{"someone else's, with suggestions", "/v1/handles/rosa.lm", http.StatusOK, `"available":false,"suggestions":["rosa.lm.a","rosa.lm26","rosa.lm_26"]`},
		{"too short", "/v1/handles/ab", http.StatusBadRequest, `"code":"invalid_handle"`},
		{"a character outside the rule", "/v1/handles/ada-l", http.StatusBadRequest, `"code":"invalid_handle"`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a, token := peopleAPI(t)
			rec := send(a, http.MethodGet, tt.path, token, "")
			if rec.Code != tt.wantStatus || !strings.Contains(rec.Body.String(), tt.wantInBody) {
				t.Errorf("status %d, body %s; want %d containing %s", rec.Code, rec.Body, tt.wantStatus, tt.wantInBody)
			}
		})
	}
}

func TestFindUser(t *testing.T) {
	tests := []struct {
		name       string
		path       string
		wantStatus int
		wantInBody string
	}{
		{"by ID", "/v1/users/rosa.lm", http.StatusOK, `"display_name":"Rosa López","handle":"rosa.lm","language":"es"`},
		{"with the @ and in capitals", "/v1/users/@Rosa.LM", http.StatusOK, `"handle":"rosa.lm"`},
		{"no one has it", "/v1/users/nobody", http.StatusNotFound, `"code":"user_not_found"`},
		{"not a possible ID", "/v1/users/x", http.StatusNotFound, `"code":"user_not_found"`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a, token := peopleAPI(t)
			rec := send(a, http.MethodGet, tt.path, token, "")
			if rec.Code != tt.wantStatus || !strings.Contains(rec.Body.String(), tt.wantInBody) {
				t.Errorf("status %d, body %s; want %d containing %s", rec.Code, rec.Body, tt.wantStatus, tt.wantInBody)
			}
		})
	}
}

func TestPeopleNeedASession(t *testing.T) {
	a, _ := peopleAPI(t)
	for _, path := range []string{"/v1/handles/ada", "/v1/users/rosa.lm"} {
		if rec := send(a, http.MethodGet, path, "", ""); rec.Code != http.StatusUnauthorized {
			t.Errorf("%s without a session: status %d", path, rec.Code)
		}
	}
}
