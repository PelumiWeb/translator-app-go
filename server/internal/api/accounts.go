package api

import (
	"cmp"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"unicode/utf8"

	"github.com/PelumiWeb/translator-app-go/server/internal/accounts"
	"github.com/PelumiWeb/translator-app-go/server/internal/auth"
)

// AccountStore is what the handlers need from the accounts store.
type AccountStore interface {
	SignIn(ctx context.Context, provider, subject, email, name string) (token string, user accounts.User, created bool, err error)
	UserForToken(ctx context.Context, token string) (accounts.User, error)
	SignOut(ctx context.Context, token string) error
	UpdateProfile(ctx context.Context, userID string, update accounts.ProfileUpdate) (accounts.User, error)
	DeleteUser(ctx context.Context, userID string) error

	SetVoice(ctx context.Context, userID, blobKey, consentVersion string) (voice accounts.Voice, replaced string, err error)
	Voice(ctx context.Context, userID string) (accounts.Voice, error)
	DeleteVoice(ctx context.Context, userID string) (blobKey string, err error)

	HandleFree(ctx context.Context, userID, handle string) (bool, error)
	SuggestHandles(ctx context.Context, handle, name string, n int) ([]string, error)
	UserByHandle(ctx context.Context, handle string) (accounts.User, error)
}

// TokenVerifier checks an identity token from Sign in with Apple or Google.
type TokenVerifier interface {
	Verify(ctx context.Context, token, nonce string) (auth.Identity, error)
}

const maxNameChars = 50

type userResponse struct {
	ID          string `json:"id"`
	DisplayName string `json:"display_name"`
	Handle      string `json:"handle"`
	Language    string `json:"language"`
}

func userJSON(u accounts.User) userResponse {
	return userResponse{ID: u.ID, DisplayName: u.DisplayName, Handle: u.Handle, Language: u.Language}
}

type signInResponse struct {
	Token   string       `json:"token"`
	User    userResponse `json:"user"`
	NewUser bool         `json:"new_user"`
}

// handleSignIn signs in with an identity token from provider, checked by
// verifier, creating the account on its first sign-in, and answers with a
// session token for the phone to keep. A nil verifier means this server is
// not set up for that provider.
//
// The body is JSON: the "identity_token" the provider gave the phone, the
// "nonce" the phone made for this sign-in, and optionally the person's
// "name". Apple tells the phone the name once, at the first sign-in, and
// never puts it in the token; Google's tokens carry it.
func (a *API) handleSignIn(provider string, verifier TokenVerifier) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if verifier == nil {
			writeError(w, http.StatusNotImplemented, "sign_in_not_configured", "this server is not set up for this way of signing in")
			return
		}
		var body struct {
			IdentityToken string `json:"identity_token"`
			Nonce         string `json:"nonce"`
			Name          string `json:"name"`
		}
		if !decodeJSON(w, r, &body) {
			return
		}
		if body.IdentityToken == "" || body.Nonce == "" {
			writeError(w, http.StatusBadRequest, "invalid_request", `"identity_token" and "nonce" are required`)
			return
		}

		identity, err := verifier.Verify(r.Context(), body.IdentityToken, body.Nonce)
		switch {
		case errors.Is(err, auth.ErrInvalidToken):
			writeError(w, http.StatusUnauthorized, "invalid_token", "the sign-in could not be verified")
			return
		case err != nil:
			a.Logger.Error("verifying a sign-in", "provider", provider, "error", err)
			writeError(w, http.StatusServiceUnavailable, "sign_in_unavailable", "the sign-in could not be checked just now; try again")
			return
		}

		name := cmp.Or(cutName(body.Name), cutName(identity.Name))
		token, user, created, err := a.Accounts.SignIn(r.Context(), provider, identity.Subject, identity.Email, name)
		if err != nil {
			a.Logger.Error("signing in", "provider", provider, "error", err)
			writeError(w, http.StatusInternalServerError, "internal", "could not sign in")
			return
		}
		// The user's ID only: names and emails stay out of the logs.
		a.Logger.Info("signed in", "provider", provider, "user_id", user.ID, "new_user", created)
		status := http.StatusOK
		if created {
			status = http.StatusCreated
		}
		writeJSON(w, status, signInResponse{Token: token, User: userJSON(user), NewUser: created})
	}
}

func (a *API) handleSignOut(w http.ResponseWriter, r *http.Request, _ accounts.User, token string) {
	if err := a.Accounts.SignOut(r.Context(), token); err != nil {
		a.Logger.Error("signing out", "error", err)
		writeError(w, http.StatusInternalServerError, "internal", "could not sign out")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (a *API) handleGetMe(w http.ResponseWriter, _ *http.Request, user accounts.User, _ string) {
	writeJSON(w, http.StatusOK, userJSON(user))
}

// handleUpdateMe changes the profile: any of "display_name", "handle" and
// "language", leaving out what is not given.
func (a *API) handleUpdateMe(w http.ResponseWriter, r *http.Request, user accounts.User, _ string) {
	var body struct {
		DisplayName *string `json:"display_name"`
		Handle      *string `json:"handle"`
		Language    *string `json:"language"`
	}
	if !decodeJSON(w, r, &body) {
		return
	}

	var update accounts.ProfileUpdate
	if body.DisplayName != nil {
		name := strings.TrimSpace(*body.DisplayName)
		if name == "" || utf8.RuneCountInString(name) > maxNameChars {
			writeError(w, http.StatusBadRequest, "invalid_name", "a name is 1 to 50 characters")
			return
		}
		update.DisplayName = &name
	}
	if body.Handle != nil {
		handle := strings.ToLower(strings.TrimSpace(*body.Handle))
		if !accounts.ValidHandle(handle) {
			writeError(w, http.StatusBadRequest, "invalid_handle", "an ID is 3 to 20 letters, digits, dots or underscores")
			return
		}
		update.Handle = &handle
	}
	if body.Language != nil {
		if !languageTag.MatchString(*body.Language) {
			writeError(w, http.StatusBadRequest, "invalid_lang", `language must be a language tag such as "es"`)
			return
		}
		update.Language = body.Language
	}

	updated, err := a.Accounts.UpdateProfile(r.Context(), user.ID, update)
	switch {
	case errors.Is(err, accounts.ErrHandleTaken):
		writeError(w, http.StatusConflict, "handle_taken", "that ID is taken; try another")
		return
	case errors.Is(err, accounts.ErrNoUser):
		writeError(w, http.StatusUnauthorized, "unauthenticated", "the account no longer exists")
		return
	case err != nil:
		a.Logger.Error("updating a profile", "user_id", user.ID, "error", err)
		writeError(w, http.StatusInternalServerError, "internal", "could not update the profile")
		return
	}
	writeJSON(w, http.StatusOK, userJSON(updated))
}

// handleDeleteMe deletes the account, its sign-ins, its sessions, its voice,
// and everything it said in its rooms.
func (a *API) handleDeleteMe(w http.ResponseWriter, r *http.Request, user accounts.User, _ string) {
	// The voice and the messages first: the database would drop their rows
	// with the user, but not their audio files, which only this knows to
	// delete.
	if err := a.deleteKeptVoice(r.Context(), user); err != nil {
		writeError(w, http.StatusInternalServerError, "internal", "could not delete the account")
		return
	}
	if err := a.forgetMessages(r.Context(), user); err != nil {
		writeError(w, http.StatusInternalServerError, "internal", "could not delete the account")
		return
	}
	if err := a.Accounts.DeleteUser(r.Context(), user.ID); err != nil && !errors.Is(err, accounts.ErrNoUser) {
		a.Logger.Error("deleting an account", "user_id", user.ID, "error", err)
		writeError(w, http.StatusInternalServerError, "internal", "could not delete the account")
		return
	}
	a.Logger.Info("account deleted", "user_id", user.ID)
	w.WriteHeader(http.StatusNoContent)
}

// signedIn runs next with the user whose session token the request carries,
// as "Authorization: Bearer <token>", or answers 401 when there is none.
func (a *API) signedIn(next func(w http.ResponseWriter, r *http.Request, user accounts.User, token string)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user, token, err := a.sessionUser(r)
		if err != nil {
			a.writeSessionError(w, err)
			return
		}
		next(w, r, user, token)
	}
}

// errNoToken means the request carries no session token at all.
var errNoToken = errors.New("no session token")

// sessionUser returns the user whose session token the request carries.
func (a *API) sessionUser(r *http.Request) (accounts.User, string, error) {
	token, ok := strings.CutPrefix(r.Header.Get("Authorization"), "Bearer ")
	if !ok || token == "" {
		return accounts.User{}, "", errNoToken
	}
	user, err := a.Accounts.UserForToken(r.Context(), token)
	return user, token, err
}

// writeSessionError answers a request whose user sessionUser could not find.
func (a *API) writeSessionError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, errNoToken):
		writeError(w, http.StatusUnauthorized, "unauthenticated", "sign in first")
	case errors.Is(err, accounts.ErrNoSession):
		writeError(w, http.StatusUnauthorized, "unauthenticated", "the session has ended; sign in again")
	default:
		a.Logger.Error("checking a session", "error", err)
		writeError(w, http.StatusInternalServerError, "internal", "could not check the session")
	}
}

// decodeJSON reads a small JSON body into v, answering 400 itself when it
// cannot, in which case it returns false.
func decodeJSON(w http.ResponseWriter, r *http.Request, v any) bool {
	// These bodies are a few hundred bytes; the limit stops a client from
	// streaming megabytes into the decoder.
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 64<<10)).Decode(v); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "expected a JSON body")
		return false
	}
	return true
}

// cutName keeps a name within the limit, counting characters, not bytes.
func cutName(name string) string {
	name = strings.TrimSpace(name)
	if utf8.RuneCountInString(name) > maxNameChars {
		name = string([]rune(name)[:maxNameChars])
	}
	return name
}
