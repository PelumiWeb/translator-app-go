package api

import (
	"errors"
	"net/http"
	"strings"

	"github.com/PelumiWeb/translator-app-go/server/internal/accounts"
)

// suggestionCount is how many free IDs are offered when the one asked for is taken.
const suggestionCount = 3

type handleResponse struct {
	Handle      string   `json:"handle"`
	Available   bool     `json:"available"`
	Suggestions []string `json:"suggestions"`
}

// handleCheckHandle says whether the user may take an ID, for the phones to
// check it while it is typed. When it is taken, it suggests free ones. The
// answer can be out of date a moment later; PATCH /v1/me has the last word.
func (a *API) handleCheckHandle(w http.ResponseWriter, r *http.Request, user accounts.User, _ string) {
	handle := strings.ToLower(r.PathValue("handle"))
	if !accounts.ValidHandle(handle) {
		writeError(w, http.StatusBadRequest, "invalid_handle", "an ID is 3 to 20 letters, digits, dots or underscores")
		return
	}
	free, err := a.Accounts.HandleFree(r.Context(), user.ID, handle)
	if err != nil {
		a.Logger.Error("checking a handle", "error", err)
		writeError(w, http.StatusInternalServerError, "internal", "could not check the ID")
		return
	}
	// An empty list, not null, so the phones need not tell the two apart.
	response := handleResponse{Handle: handle, Available: free, Suggestions: []string{}}
	if !free {
		response.Suggestions, err = a.Accounts.SuggestHandles(r.Context(), handle, user.DisplayName, suggestionCount)
		if err != nil {
			a.Logger.Error("suggesting handles", "error", err)
			writeError(w, http.StatusInternalServerError, "internal", "could not check the ID")
			return
		}
	}
	writeJSON(w, http.StatusOK, response)
}

// handleFindUser finds someone by their ID, for adding them.
func (a *API) handleFindUser(w http.ResponseWriter, r *http.Request, _ accounts.User, _ string) {
	person, ok := a.findUser(w, r, r.PathValue("handle"))
	if !ok {
		return
	}
	writeJSON(w, http.StatusOK, userJSON(person))
}

// findUser looks up the user with handle, or answers 404 itself and returns false.
func (a *API) findUser(w http.ResponseWriter, r *http.Request, handle string) (accounts.User, bool) {
	handle = strings.ToLower(strings.TrimPrefix(strings.TrimSpace(handle), "@"))
	if !accounts.ValidHandle(handle) {
		writeError(w, http.StatusNotFound, "user_not_found", "no one has that ID")
		return accounts.User{}, false
	}
	person, err := a.Accounts.UserByHandle(r.Context(), handle)
	switch {
	case errors.Is(err, accounts.ErrNoUser):
		writeError(w, http.StatusNotFound, "user_not_found", "no one has that ID")
		return accounts.User{}, false
	case err != nil:
		a.Logger.Error("finding a user", "error", err)
		writeError(w, http.StatusInternalServerError, "internal", "could not look up the ID")
		return accounts.User{}, false
	}
	return person, true
}
