package api

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/PelumiWeb/translator-app-go/server/internal/accounts"
	"github.com/PelumiWeb/translator-app-go/server/internal/rooms"
)

// Block and report (milestone 16, ADR 0035).

const maxReportNoteChars = 500

type blockedResponse struct {
	userResponse
	BlockedAt time.Time `json:"blocked_at"`
}

// handleBlock blocks the person whose "user_id" is given: neither can send to
// the other until they are unblocked. Blocking twice is not an error.
func (a *API) handleBlock(w http.ResponseWriter, r *http.Request, user accounts.User, _ string) {
	var body struct {
		UserID string `json:"user_id"`
	}
	if !decodeJSON(w, r, &body) {
		return
	}
	if !uuidPattern.MatchString(body.UserID) {
		writeError(w, http.StatusBadRequest, "invalid_user_id", `"user_id" must be a user's id`)
		return
	}
	if body.UserID == user.ID {
		writeError(w, http.StatusBadRequest, "cannot_block_self", "that is you")
		return
	}
	err := a.Rooms.Block(r.Context(), user.ID, body.UserID)
	switch {
	case errors.Is(err, rooms.ErrNoPerson):
		writeError(w, http.StatusNotFound, "user_not_found", "no such person")
		return
	case err != nil:
		a.Logger.Error("blocking someone", "user_id", user.ID, "error", err)
		writeError(w, http.StatusInternalServerError, "internal", "could not block them")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// handleUnblock lifts a block. Lifting one that does not stand is not an error.
func (a *API) handleUnblock(w http.ResponseWriter, r *http.Request, user accounts.User, _ string) {
	otherID, ok := pathID(w, r, "user_not_found", "no such person")
	if !ok {
		return
	}
	if err := a.Rooms.Unblock(r.Context(), user.ID, otherID); err != nil {
		a.Logger.Error("unblocking someone", "user_id", user.ID, "error", err)
		writeError(w, http.StatusInternalServerError, "internal", "could not unblock them")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// handleListBlocked lists the people the user has blocked, the most recent
// first.
func (a *API) handleListBlocked(w http.ResponseWriter, r *http.Request, user accounts.User, _ string) {
	found, err := a.Rooms.Blocked(r.Context(), user.ID)
	if err != nil {
		a.Logger.Error("listing blocked people", "user_id", user.ID, "error", err)
		writeError(w, http.StatusInternalServerError, "internal", "could not list them")
		return
	}
	response := struct {
		Blocked []blockedResponse `json:"blocked"`
	}{Blocked: []blockedResponse{}}
	for _, p := range found {
		response.Blocked = append(response.Blocked, blockedResponse{
			userResponse: userResponse{ID: p.ID, DisplayName: p.DisplayName, Handle: p.Handle, Language: p.Language},
			BlockedAt:    p.BlockedAt,
		})
	}
	writeJSON(w, http.StatusOK, response)
}

// handleReport reports a person ("user_id") or one of their messages
// ("message_id"), with a "reason" and an optional "note". It also blocks
// them, unless "block" is false. A reported message is copied into the
// report, recording and all, so that it can be looked at even if its sender
// deletes it with their account; the copy goes when the report is closed.
func (a *API) handleReport(w http.ResponseWriter, r *http.Request, user accounts.User, _ string) {
	var body struct {
		UserID    string `json:"user_id"`
		MessageID string `json:"message_id"`
		Reason    string `json:"reason"`
		Note      string `json:"note"`
		Block     *bool  `json:"block"` // a pointer, since leaving it out means yes
	}
	if !decodeJSON(w, r, &body) {
		return
	}
	if !rooms.ValidReason(body.Reason) {
		writeError(w, http.StatusBadRequest, "invalid_reason",
			"reason must be spam, harassment, hate, sexual, violence, self_harm or other")
		return
	}
	note := strings.TrimSpace(body.Note)
	if utf8.RuneCountInString(note) > maxReportNoteChars {
		writeError(w, http.StatusBadRequest, "invalid_note", fmt.Sprintf("a note is at most %d characters", maxReportNoteChars))
		return
	}
	report := rooms.NewReport{
		ReporterID: user.ID, ReportedID: body.UserID, Reason: body.Reason, Note: note,
		Block: body.Block == nil || *body.Block,
	}

	switch {
	case body.MessageID != "":
		if !uuidPattern.MatchString(body.MessageID) {
			writeError(w, http.StatusNotFound, "message_not_found", "no such message")
			return
		}
		message, err := a.Rooms.Message(r.Context(), user.ID, body.MessageID)
		if !a.messageFound(w, err, user) {
			return
		}
		if message.SenderID == user.ID {
			writeError(w, http.StatusBadRequest, "cannot_report_self", "that message is yours")
			return
		}
		if body.UserID != "" && body.UserID != message.SenderID {
			writeError(w, http.StatusBadRequest, "invalid_request", `"user_id" is not who sent the message`)
			return
		}
		report.ReportedID = message.SenderID
		report.Message = reportedMessage(message, user)
	case !uuidPattern.MatchString(body.UserID):
		writeError(w, http.StatusBadRequest, "invalid_request", `give the "user_id" of the person, or the "message_id"`)
		return
	case body.UserID == user.ID:
		writeError(w, http.StatusBadRequest, "cannot_report_self", "that is you")
		return
	}

	if report.Message != nil && report.Message.AudioKey != "" {
		copied, err := a.copyToReports(report.Message.AudioKey)
		if err != nil {
			a.Logger.Error("copying a reported recording", "user_id", user.ID, "error", err)
			writeError(w, http.StatusInternalServerError, "internal", "could not send the report")
			return
		}
		report.Message.AudioKey = copied
	}
	id, err := a.Rooms.AddReport(r.Context(), report)
	if err != nil && report.Message != nil && report.Message.AudioKey != "" {
		a.deleteReportFile(report.Message.AudioKey) // nothing points at it
	}
	switch {
	case errors.Is(err, rooms.ErrNoPerson):
		writeError(w, http.StatusNotFound, "user_not_found", "no such person")
		return
	case err != nil:
		a.Logger.Error("adding a report", "user_id", user.ID, "error", err)
		writeError(w, http.StatusInternalServerError, "internal", "could not send the report")
		return
	}
	a.Wake() // the notice to the owner
	// IDs and the reason only: what was said stays out of the logs.
	a.Logger.Info("report received", "report_id", id, "reason", report.Reason, "about_message", report.Message != nil)
	writeJSON(w, http.StatusCreated, map[string]string{"id": id})
}

// reportedMessage is a message as the reporter saw it: their words, and the
// translation into their language when there is one. The recording's key is
// the message's own until copyToReports replaces it.
func reportedMessage(m rooms.Message, reporter accounts.User) *rooms.ReportedMessage {
	reported := &rooms.ReportedMessage{ID: m.ID, Kind: m.Kind, Lang: m.Lang, Transcript: m.Transcript, AudioKey: m.AudioKey}
	for _, rendition := range m.Renditions {
		if rendition.Lang == reporter.Language && rendition.Lang != m.Lang {
			reported.Translation = rendition.Text
		}
	}
	return reported
}

// copyToReports copies a message's recording into the reports' files, and
// returns the copy's key.
func (a *API) copyToReports(messageKey string) (string, error) {
	file, err := a.Messages.Open(messageKey)
	if err != nil {
		return "", err
	}
	defer file.Close()
	key := rand.Text() + ".wav"
	if err := a.Reports.Put(key, file); err != nil {
		return "", err
	}
	return key, nil
}

func (a *API) deleteReportFile(key string) {
	if err := a.Reports.Delete(key); err != nil {
		a.Logger.Warn("deleting a report file", "error", err)
	}
}

// revokeApple revokes the account's Sign in with Apple, as Apple asks of an
// account deleted (Guideline 5.1.1(v)): the person sees Shinar leave their
// Apple account's list of apps. It does not stop the deletion when it fails,
// which Apple's guidance allows; it logs its own errors.
func (a *API) revokeApple(ctx context.Context, user accounts.User) {
	if a.AppleTokens == nil {
		return
	}
	tokens, err := a.Accounts.AppleRefreshTokens(ctx, user.ID)
	if err != nil {
		a.Logger.Warn("reading Apple refresh tokens", "user_id", user.ID, "error", err)
		return
	}
	for _, token := range tokens {
		if err := a.AppleTokens.Revoke(ctx, token); err != nil {
			a.Logger.Warn("revoking Sign in with Apple", "user_id", user.ID, "error", err)
		}
	}
}

// sendRefused answers a send or a retry that a block stopped, and reports
// whether it did.
func sendRefused(w http.ResponseWriter, err error) bool {
	switch {
	case errors.Is(err, rooms.ErrBlocked):
		// No more than that: the person blocked is not told.
		writeError(w, http.StatusForbidden, "cannot_send", "this message cannot be sent")
	case errors.Is(err, rooms.ErrYouBlocked):
		writeError(w, http.StatusConflict, "you_blocked", "you blocked this person; unblock them to send")
	default:
		return false
	}
	return true
}

// DeleteAccount deletes an account, its voice, and everything it said in its
// rooms. It logs its own errors. Deleting one's own account uses it, and so
// does the owner's command for deleting an account on request.
func (a *API) DeleteAccount(ctx context.Context, user accounts.User) error {
	a.revokeApple(ctx, user)
	// The voice and the messages first: the database would drop their rows
	// with the user, but not their audio files, which only this knows to
	// delete.
	if err := a.deleteKeptVoice(ctx, user); err != nil {
		return err
	}
	if err := a.forgetMessages(ctx, user); err != nil {
		return err
	}
	if err := a.Accounts.DeleteUser(ctx, user.ID); err != nil && !errors.Is(err, accounts.ErrNoUser) {
		a.Logger.Error("deleting an account", "user_id", user.ID, "error", err)
		return err
	}
	a.Logger.Info("account deleted", "user_id", user.ID)
	return nil
}
