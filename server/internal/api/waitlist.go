package api

import (
	"context"
	"net/http"
	"net/mail"
	"strings"

	"github.com/PelumiWeb/translator-app-go/server/internal/waitlist"
)

// WaitlistStore is what the waitlist handler needs from the waitlist.
type WaitlistStore interface {
	Join(ctx context.Context, e waitlist.Entry) error
}

// maxEmailChars is the longest address mail servers accept (RFC 5321).
const maxEmailChars = 254

// handleJoinWaitlist puts someone on the waitlist from the website: JSON
// "email", "phone" ("iphone" or "android") and an optional first "name".
// Anyone may call it, signed in or not (ADR 0030).
//
// The answer is the same whether the address is new or already on the list,
// so the endpoint cannot be used to find out who has joined.
func (a *API) handleJoinWaitlist(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Email string `json:"email"`
		Name  string `json:"name"`
		Phone string `json:"phone"`
		// A field the page hides from people. Form-filling bots fill in
		// every field they find, so one filled in here is taken for a bot
		// and quietly not kept.
		Company string `json:"company"`
	}
	if !decodeJSON(w, r, &body) {
		return
	}
	email := strings.TrimSpace(body.Email)
	if !plainAddress(email) {
		writeError(w, http.StatusBadRequest, "invalid_email", "that does not look like an email address")
		return
	}
	if body.Phone != waitlist.PhoneIPhone && body.Phone != waitlist.PhoneAndroid {
		writeError(w, http.StatusBadRequest, "invalid_phone", `phone must be "iphone" or "android"`)
		return
	}
	if body.Company == "" {
		entry := waitlist.Entry{Email: email, Name: cutName(body.Name), Phone: body.Phone}
		if err := a.Waitlist.Join(r.Context(), entry); err != nil {
			a.Logger.Error("joining the waitlist", "error", err)
			writeError(w, http.StatusInternalServerError, "internal", "could not add you to the list")
			return
		}
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "joined"})
}

// plainAddress reports whether s is a bare email address, such as
// "ada@example.com": no display name, no angle brackets, a dot in the domain.
// net/mail accepts far more ("Ada <ada@example>"), which is right for mail
// headers and wrong for a form field.
func plainAddress(s string) bool {
	if s == "" || len(s) > maxEmailChars {
		return false
	}
	parsed, err := mail.ParseAddress(s)
	if err != nil || parsed.Address != s || parsed.Name != "" {
		return false
	}
	at := strings.LastIndexByte(s, '@')
	domain := s[at+1:]
	return strings.Contains(domain, ".") && !strings.HasPrefix(domain, ".") && !strings.HasSuffix(domain, ".")
}
