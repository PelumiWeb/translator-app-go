package api

import (
	"errors"
	"net/http"
	"regexp"

	"github.com/PelumiWeb/translator-app-go/server/internal/accounts"
	"github.com/PelumiWeb/translator-app-go/server/internal/push"
)

// The shapes of the push addresses the services give phones. Checking them
// matters beyond tidiness: an APNs token becomes part of the URL the server
// calls at Apple.
var pushTokenPatterns = map[string]*regexp.Regexp{
	push.ServiceAPNs:        regexp.MustCompile(`^[0-9a-fA-F]{32,200}$`),
	push.ServiceAPNsSandbox: regexp.MustCompile(`^[0-9a-fA-F]{32,200}$`),
	push.ServiceFCM:         regexp.MustCompile(`^[A-Za-z0-9_:.\-]{20,}$`),
}

// maxPushToken is far beyond any real token; Go's regexps cannot count that high.
const maxPushToken = 4096

// handlePutPush records where the phone signed in with this session receives
// notifications. The body is JSON: the "service" ("apns", "apns-sandbox" or
// "fcm") and the "token" it gave the phone. Signing out forgets it.
func (a *API) handlePutPush(w http.ResponseWriter, r *http.Request, _ accounts.User, session string) {
	var body struct {
		Service string `json:"service"`
		Token   string `json:"token"`
	}
	if !decodeJSON(w, r, &body) {
		return
	}
	pattern, ok := pushTokenPatterns[body.Service]
	if !ok {
		writeError(w, http.StatusBadRequest, "invalid_request", `"service" must be "apns", "apns-sandbox" or "fcm"`)
		return
	}
	if len(body.Token) > maxPushToken || !pattern.MatchString(body.Token) {
		writeError(w, http.StatusBadRequest, "invalid_request", `"token" is not a push token of that service`)
		return
	}
	err := a.Accounts.SetPushToken(r.Context(), session, body.Service, body.Token)
	if errors.Is(err, accounts.ErrNoSession) {
		a.writeSessionError(w, err)
		return
	}
	if err != nil {
		a.Logger.Error("keeping a push token", "error", err)
		writeError(w, http.StatusInternalServerError, "internal", "could not keep the push token")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
