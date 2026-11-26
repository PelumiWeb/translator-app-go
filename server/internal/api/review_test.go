package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"slices"
	"sync"
	"testing"
)

// fakeAppleTokens trades "code-…" for "refresh-…" and records what it revokes.
type fakeAppleTokens struct {
	mu       sync.Mutex
	exchange error
	revoked  []string
}

func (f *fakeAppleTokens) Exchange(_ context.Context, code string) (string, error) {
	if f.exchange != nil {
		return "", f.exchange
	}
	return "refresh-" + code, nil
}

func (f *fakeAppleTokens) Revoke(_ context.Context, token string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.revoked = append(f.revoked, token)
	return nil
}

func TestDeletingAnAccountRevokesItsSignInWithApple(t *testing.T) {
	a, store := accountsAPI()
	tokens := &fakeAppleTokens{}
	a.AppleTokens = tokens

	rec := send(a, http.MethodPost, "/v1/sessions/apple", "", `{"identity_token":"good","nonce":"n","authorization_code":"c1"}`)
	var signIn signInResponse
	if rec.Code != http.StatusCreated || json.Unmarshal(rec.Body.Bytes(), &signIn) != nil {
		t.Fatalf("sign-in: status %d, body %s", rec.Code, rec.Body)
	}
	if got, _ := store.AppleRefreshTokens(context.Background(), signIn.User.ID); !slices.Equal(got, []string{"refresh-c1"}) {
		t.Errorf("kept = %v, want the token traded for the code", got)
	}

	if rec := send(a, http.MethodDelete, "/v1/me", signIn.Token, ""); rec.Code != http.StatusNoContent {
		t.Fatalf("delete: status %d, body %s", rec.Code, rec.Body)
	}
	if !slices.Equal(tokens.revoked, []string{"refresh-c1"}) {
		t.Errorf("revoked = %v, want refresh-c1", tokens.revoked)
	}
}

func TestASignInStandsWhenAppleWillNotTradeTheCode(t *testing.T) {
	a, _ := accountsAPI()
	a.AppleTokens = &fakeAppleTokens{exchange: errors.New("invalid_grant")}

	rec := send(a, http.MethodPost, "/v1/sessions/apple", "", `{"identity_token":"good","nonce":"n","authorization_code":"old"}`)
	if rec.Code != http.StatusCreated {
		t.Errorf("status %d, body %s; want signed in all the same", rec.Code, rec.Body)
	}
}

func TestAgreeingToTheServicesThatHelp(t *testing.T) {
	w := newRoomsWorld(t, map[string]string{"ada": "en"})

	if rec := w.do(http.MethodPatch, "/v1/me", "ada", `{"services_consent":"2026-11-23"}`); rec.Code != http.StatusOK {
		t.Fatalf("agreeing: status %d, body %s", rec.Code, rec.Body)
	}
	var me userResponse
	if err := json.Unmarshal(w.do(http.MethodGet, "/v1/me", "ada", "").Body.Bytes(), &me); err != nil || me.ServicesConsent != "2026-11-23" {
		t.Errorf("profile = %+v, %v; want the version agreed to", me, err)
	}
	if rec := w.do(http.MethodPatch, "/v1/me", "ada", `{"services_consent":""}`); rec.Code != http.StatusBadRequest {
		t.Errorf("an empty version: status %d, want 400", rec.Code)
	}
}
