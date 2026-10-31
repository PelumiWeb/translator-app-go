package api

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/PelumiWeb/translator-app-go/server/internal/accounts"
	"github.com/PelumiWeb/translator-app-go/server/internal/auth"
)

// fakeAccounts keeps accounts in memory. The handle "taken" belongs to someone else.
type fakeAccounts struct {
	mu       sync.Mutex
	users    map[string]accounts.User  // by user ID
	bySub    map[string]string         // subject to user ID
	sessions map[string]string         // token to user ID
	voices   map[string]accounts.Voice // by user ID
}

func newFakeAccounts() *fakeAccounts {
	return &fakeAccounts{
		users:    map[string]accounts.User{},
		bySub:    map[string]string{},
		sessions: map[string]string{},
		voices:   map[string]accounts.Voice{},
	}
}

func (f *fakeAccounts) SignIn(_ context.Context, provider, subject, _, name string) (string, accounts.User, bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	id, known := f.bySub[subject]
	if !known {
		id = "user-" + subject
		f.bySub[subject] = id
		f.users[id] = accounts.User{ID: id, DisplayName: name}
	}
	token := "token-" + subject + "-" + string(rune('a'+len(f.sessions)))
	f.sessions[token] = id
	return token, f.users[id], !known, nil
}

func (f *fakeAccounts) UserForToken(_ context.Context, token string) (accounts.User, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	id, ok := f.sessions[token]
	if !ok {
		return accounts.User{}, accounts.ErrNoSession
	}
	return f.users[id], nil
}

func (f *fakeAccounts) SignOut(_ context.Context, token string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	delete(f.sessions, token)
	return nil
}

func (f *fakeAccounts) UpdateProfile(_ context.Context, userID string, update accounts.ProfileUpdate) (accounts.User, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	user := f.users[userID]
	if update.Handle != nil && *update.Handle == "taken" {
		return accounts.User{}, accounts.ErrHandleTaken
	}
	if update.DisplayName != nil {
		user.DisplayName = *update.DisplayName
	}
	if update.Handle != nil {
		user.Handle = *update.Handle
	}
	if update.Language != nil {
		user.Language = *update.Language
	}
	f.users[userID] = user
	return user, nil
}

func (f *fakeAccounts) DeleteUser(_ context.Context, userID string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	delete(f.users, userID)
	for token, id := range f.sessions {
		if id == userID {
			delete(f.sessions, token)
		}
	}
	return nil
}

func (f *fakeAccounts) SetVoice(_ context.Context, userID, blobKey, consentVersion string) (accounts.Voice, string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if _, ok := f.users[userID]; !ok {
		return accounts.Voice{}, "", accounts.ErrNoUser
	}
	replaced := f.voices[userID].BlobKey
	voice := accounts.Voice{BlobKey: blobKey, ConsentVersion: consentVersion, ConsentedAt: time.Unix(1_800_000_000, 0).UTC()}
	f.voices[userID] = voice
	return voice, replaced, nil
}

func (f *fakeAccounts) Voice(_ context.Context, userID string) (accounts.Voice, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	voice, ok := f.voices[userID]
	if !ok {
		return accounts.Voice{}, accounts.ErrNoVoice
	}
	return voice, nil
}

func (f *fakeAccounts) DeleteVoice(_ context.Context, userID string) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	voice, ok := f.voices[userID]
	if !ok {
		return "", accounts.ErrNoVoice
	}
	delete(f.voices, userID)
	return voice.BlobKey, nil
}

func (f *fakeAccounts) HandleFree(_ context.Context, userID, handle string) (bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if handle == "taken" {
		return false, nil
	}
	for id, user := range f.users {
		if user.Handle == handle && id != userID {
			return false, nil
		}
	}
	return true, nil
}

func (f *fakeAccounts) SuggestHandles(_ context.Context, handle, _ string, n int) ([]string, error) {
	return []string{handle + ".a", handle + "26", handle + "_26"}[:n], nil
}

func (f *fakeAccounts) UserByHandle(_ context.Context, handle string) (accounts.User, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, user := range f.users {
		if user.Handle == handle && user.Language != "" {
			return user, nil
		}
	}
	return accounts.User{}, accounts.ErrNoUser
}

// noRooms is a user's rooms when they have none. Only what account deletion
// calls is implemented; the rest of RoomStore is left to panic, unused.
type noRooms struct{ RoomStore }

func (noRooms) ForgetSender(context.Context, string) ([]string, error) { return nil, nil }

// fakeVerifier accepts the token "good" with the nonce "n" as the person
// subject, treats "unreachable" as the provider being out of reach, and
// refuses anything else.
type fakeVerifier struct {
	subject, name string
}

func (f fakeVerifier) Verify(_ context.Context, token, nonce string) (auth.Identity, error) {
	switch {
	case token == "unreachable":
		return auth.Identity{}, errors.New("dial tcp: connection refused")
	case token == "good" && nonce == "n":
		return auth.Identity{Subject: f.subject, Name: f.name}, nil
	default:
		return auth.Identity{}, auth.ErrInvalidToken
	}
}

func accountsAPI() (*API, *fakeAccounts) {
	a := newTestAPI()
	store := newFakeAccounts()
	a.Accounts, a.Voices = store, &fakeAudio{}
	a.Rooms, a.Messages = noRooms{}, &fakeAudio{}
	a.Apple = fakeVerifier{subject: "apple-1"}
	a.Google = fakeVerifier{subject: "google-1", name: "Ada from Google"}
	return a, store
}

func send(a *API, method, path, token, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	rec := httptest.NewRecorder()
	a.Handler().ServeHTTP(rec, req)
	return rec
}

func TestSignInWithApple(t *testing.T) {
	tests := []struct {
		name       string
		body       string
		wantStatus int
		wantInBody string
	}{
		{"a first sign-in creates the account", `{"identity_token":"good","nonce":"n","name":"Ada"}`, http.StatusCreated, `"new_user":true`},
		{"a token Apple did not issue", `{"identity_token":"forged","nonce":"n"}`, http.StatusUnauthorized, `"code":"invalid_token"`},
		{"Apple out of reach", `{"identity_token":"unreachable","nonce":"n"}`, http.StatusServiceUnavailable, `"code":"sign_in_unavailable"`},
		{"no nonce", `{"identity_token":"good"}`, http.StatusBadRequest, `"code":"invalid_request"`},
		{"not JSON", `identity_token=good`, http.StatusBadRequest, `"code":"invalid_request"`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a, _ := accountsAPI()

			rec := send(a, http.MethodPost, "/v1/sessions/apple", "", tt.body)

			if rec.Code != tt.wantStatus || !strings.Contains(rec.Body.String(), tt.wantInBody) {
				t.Errorf("status %d, body %s; want %d with %s", rec.Code, rec.Body, tt.wantStatus, tt.wantInBody)
			}
		})
	}
}

func TestSignInWithGoogle(t *testing.T) {
	a, _ := accountsAPI()

	rec := send(a, http.MethodPost, "/v1/sessions/google", "", `{"identity_token":"good","nonce":"n"}`)

	// Google's token carries the name, so the phone need not send it.
	if rec.Code != http.StatusCreated || !strings.Contains(rec.Body.String(), `"display_name":"Ada from Google"`) {
		t.Errorf("status %d, body %s", rec.Code, rec.Body)
	}
}

func TestSignInWithGoogleOnAServerNotSetUpForIt(t *testing.T) {
	a, _ := accountsAPI()
	a.Google = nil

	rec := send(a, http.MethodPost, "/v1/sessions/google", "", `{"identity_token":"good","nonce":"n"}`)

	if rec.Code != http.StatusNotImplemented || !strings.Contains(rec.Body.String(), `"sign_in_not_configured"`) {
		t.Errorf("status %d, body %s; want 501 sign_in_not_configured", rec.Code, rec.Body)
	}
}

func TestSigningInAgainFindsTheSameAccount(t *testing.T) {
	a, _ := accountsAPI()
	send(a, http.MethodPost, "/v1/sessions/apple", "", `{"identity_token":"good","nonce":"n","name":"Ada"}`)

	rec := send(a, http.MethodPost, "/v1/sessions/apple", "", `{"identity_token":"good","nonce":"n"}`)

	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"new_user":false`) || !strings.Contains(rec.Body.String(), `"display_name":"Ada"`) {
		t.Errorf("status %d, body %s", rec.Code, rec.Body)
	}
}

func TestTheProfileNeedsASession(t *testing.T) {
	a, store := accountsAPI()
	token, _, _, _ := store.SignIn(context.Background(), accounts.ProviderApple, "apple-1", "", "Ada")

	tests := []struct {
		name       string
		token      string
		wantStatus int
	}{
		{"no token", "", http.StatusUnauthorized},
		{"a token from no session", "made-up", http.StatusUnauthorized},
		{"a session's token", token, http.StatusOK},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := send(a, http.MethodGet, "/v1/me", tt.token, "")
			if rec.Code != tt.wantStatus {
				t.Errorf("status %d, body %s; want %d", rec.Code, rec.Body, tt.wantStatus)
			}
		})
	}
}

func TestUpdateMe(t *testing.T) {
	tests := []struct {
		name       string
		body       string
		wantStatus int
		wantInBody string
	}{
		{"a name", `{"display_name":"  Ada L.  "}`, http.StatusOK, `"display_name":"Ada L."`},
		{"an empty name", `{"display_name":"  "}`, http.StatusBadRequest, `"code":"invalid_name"`},
		{"an ID, in any case", `{"handle":" Ada_L "}`, http.StatusOK, `"handle":"ada_l"`},
		{"an ID with a space", `{"handle":"ada l"}`, http.StatusBadRequest, `"code":"invalid_handle"`},
		{"an ID too short", `{"handle":"ad"}`, http.StatusBadRequest, `"code":"invalid_handle"`},
		{"an ID someone has", `{"handle":"taken"}`, http.StatusConflict, `"code":"handle_taken"`},
		{"a language", `{"language":"yo"}`, http.StatusOK, `"language":"yo"`},
		{"not a language", `{"language":"yoruba!"}`, http.StatusBadRequest, `"code":"invalid_lang"`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a, store := accountsAPI()
			token, _, _, _ := store.SignIn(context.Background(), accounts.ProviderApple, "apple-1", "", "Ada")

			rec := send(a, http.MethodPatch, "/v1/me", token, tt.body)

			if rec.Code != tt.wantStatus || !strings.Contains(rec.Body.String(), tt.wantInBody) {
				t.Errorf("status %d, body %s; want %d with %s", rec.Code, rec.Body, tt.wantStatus, tt.wantInBody)
			}
		})
	}
}

func TestSignOutAndDeleteEndTheSession(t *testing.T) {
	for _, path := range []string{"/v1/sessions/current", "/v1/me"} {
		t.Run(path, func(t *testing.T) {
			a, store := accountsAPI()
			token, _, _, _ := store.SignIn(context.Background(), accounts.ProviderApple, "apple-1", "", "Ada")

			if rec := send(a, http.MethodDelete, path, token, ""); rec.Code != http.StatusNoContent {
				t.Fatalf("status %d, body %s; want 204", rec.Code, rec.Body)
			}
			if rec := send(a, http.MethodGet, "/v1/me", token, ""); rec.Code != http.StatusUnauthorized {
				t.Errorf("afterwards, GET /v1/me: status %d, want 401", rec.Code)
			}
		})
	}
}
