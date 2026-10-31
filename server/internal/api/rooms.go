package api

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/PelumiWeb/translator-app-go/server/internal/accounts"
	"github.com/PelumiWeb/translator-app-go/server/internal/rooms"
)

// RoomStore is what the handlers need from the rooms store.
type RoomStore interface {
	OpenPair(ctx context.Context, userID, otherID string) (roomID string, created bool, err error)
	Rooms(ctx context.Context, userID string) ([]rooms.Room, error)
	Room(ctx context.Context, userID, roomID string) (rooms.Room, error)
	Messages(ctx context.Context, userID, roomID, before string, limit int) ([]rooms.Message, error)
	Message(ctx context.Context, userID, messageID string) (rooms.Message, error)
	AddMessage(ctx context.Context, m rooms.NewMessage) (message rooms.Message, created bool, err error)
	Retry(ctx context.Context, userID, messageID string) (rooms.Message, error)
	MarkRead(ctx context.Context, userID, roomID string) error
	ForgetSender(ctx context.Context, userID string) (audioKeys []string, err error)
}

// RoomChanges tells a user's open streams that something in their rooms changed.
type RoomChanges interface {
	Subscribe(userID string) (changes <-chan rooms.Change, unsubscribe func())
}

const (
	// A voice message is at most 30 seconds; the second spare covers
	// rounding on the phones.
	maxMessageMS = 31_000
	// Thirty seconds of speech is about 80 words. The limit keeps what the
	// translation and voice services are paid to process in check.
	maxTranscriptChars = 1000

	defaultPage = 50
	maxPage     = 100
)

type roomResponse struct {
	ID          string           `json:"id"`
	People      []userResponse   `json:"people"` // everyone in it but the one asking
	LastMessage *messageResponse `json:"last_message"`
	Unread      int              `json:"unread"`
	CreatedAt   time.Time        `json:"created_at"`
}

type messageResponse struct {
	ID         string              `json:"id"`
	RoomID     string              `json:"room_id"`
	SenderID   string              `json:"sender_id"`
	Mine       bool                `json:"mine"`
	Lang       string              `json:"lang"`
	Transcript string              `json:"transcript"`
	DurationMS int                 `json:"duration_ms"`
	CreatedAt  time.Time           `json:"created_at"`
	Renditions []renditionResponse `json:"renditions"`
}

type renditionResponse struct {
	Lang     string `json:"lang"`
	Status   string `json:"status"`
	Text     string `json:"text,omitempty"`
	Voice    string `json:"voice,omitempty"`
	AudioURL string `json:"audio_url,omitempty"`
}

func roomJSON(r rooms.Room, viewer accounts.User) roomResponse {
	response := roomResponse{ID: r.ID, People: []userResponse{}, Unread: r.Unread, CreatedAt: r.CreatedAt}
	for _, p := range r.People {
		response.People = append(response.People, userResponse{ID: p.ID, DisplayName: p.DisplayName, Handle: p.Handle, Language: p.Language})
	}
	if r.Last != nil {
		last := messageJSON(*r.Last, viewer)
		response.LastMessage = &last
	}
	return response
}

// messageJSON shows a message as viewer sees it. The sender sees what it
// became in every language; a listener sees only their own.
func messageJSON(m rooms.Message, viewer accounts.User) messageResponse {
	response := messageResponse{
		ID: m.ID, RoomID: m.RoomID, SenderID: m.SenderID, Mine: m.SenderID == viewer.ID,
		Lang: m.Lang, Transcript: m.Transcript, DurationMS: m.DurationMS, CreatedAt: m.CreatedAt,
		Renditions: []renditionResponse{},
	}
	for _, r := range m.Renditions {
		if !response.Mine && r.Lang != viewer.Language {
			continue
		}
		rendition := renditionResponse{Lang: r.Lang, Status: r.Status, Text: r.Text, Voice: r.Voice}
		if r.Status == rooms.StatusReady && (r.Voice == rooms.VoiceOwn || r.Voice == rooms.VoiceOriginal) {
			rendition.AudioURL = "/v1/messages/" + m.ID + "/audio?lang=" + r.Lang
		}
		response.Renditions = append(response.Renditions, rendition)
	}
	return response
}

// handleOpenRoom opens a room with the person whose ID is given as "handle",
// or returns the one the two already have. It opens at once: there is no
// request for the other person to accept (the owner's choice, 2026-10-31).
func (a *API) handleOpenRoom(w http.ResponseWriter, r *http.Request, user accounts.User, _ string) {
	var body struct {
		Handle string `json:"handle"`
	}
	if !decodeJSON(w, r, &body) {
		return
	}
	if user.Language == "" {
		writeError(w, http.StatusConflict, "profile_incomplete", "choose the language you speak first")
		return
	}
	person, ok := a.findUser(w, r, body.Handle)
	if !ok {
		return
	}
	if person.ID == user.ID {
		writeError(w, http.StatusBadRequest, "cannot_add_self", "that is your own ID")
		return
	}

	roomID, created, err := a.Rooms.OpenPair(r.Context(), user.ID, person.ID)
	if err != nil {
		a.Logger.Error("opening a room", "user_id", user.ID, "error", err)
		writeError(w, http.StatusInternalServerError, "internal", "could not open the room")
		return
	}
	room, err := a.Rooms.Room(r.Context(), user.ID, roomID)
	if err != nil {
		a.Logger.Error("reading a room", "room_id", roomID, "error", err)
		writeError(w, http.StatusInternalServerError, "internal", "could not open the room")
		return
	}
	status := http.StatusOK
	if created {
		status = http.StatusCreated
	}
	writeJSON(w, status, roomJSON(room, user))
}

func (a *API) handleListRooms(w http.ResponseWriter, r *http.Request, user accounts.User, _ string) {
	found, err := a.Rooms.Rooms(r.Context(), user.ID)
	if err != nil {
		a.Logger.Error("listing rooms", "user_id", user.ID, "error", err)
		writeError(w, http.StatusInternalServerError, "internal", "could not list the rooms")
		return
	}
	response := struct {
		Rooms []roomResponse `json:"rooms"`
	}{Rooms: []roomResponse{}}
	for _, room := range found {
		response.Rooms = append(response.Rooms, roomJSON(room, user))
	}
	writeJSON(w, http.StatusOK, response)
}

func (a *API) handleGetRoom(w http.ResponseWriter, r *http.Request, user accounts.User, _ string) {
	roomID, ok := pathID(w, r, "room_not_found", "no such room")
	if !ok {
		return
	}
	room, err := a.Rooms.Room(r.Context(), user.ID, roomID)
	if !a.roomFound(w, err, user) {
		return
	}
	writeJSON(w, http.StatusOK, roomJSON(room, user))
}

// handleListMessages returns a room's messages, oldest first: the newest
// page, or with ?before=<message id>, the page before it.
func (a *API) handleListMessages(w http.ResponseWriter, r *http.Request, user accounts.User, _ string) {
	roomID, ok := pathID(w, r, "room_not_found", "no such room")
	if !ok {
		return
	}
	before := r.URL.Query().Get("before")
	if before != "" && !uuidPattern.MatchString(before) {
		writeError(w, http.StatusBadRequest, "invalid_before", "before must be a message id")
		return
	}
	limit := defaultPage
	if v := r.URL.Query().Get("limit"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 1 || n > maxPage {
			writeError(w, http.StatusBadRequest, "invalid_limit", fmt.Sprintf("limit must be 1 to %d", maxPage))
			return
		}
		limit = n
	}

	found, err := a.Rooms.Messages(r.Context(), user.ID, roomID, before, limit)
	if !a.roomFound(w, err, user) {
		return
	}
	response := struct {
		Messages []messageResponse `json:"messages"`
	}{Messages: []messageResponse{}}
	for _, m := range found {
		response.Messages = append(response.Messages, messageJSON(m, user))
	}
	writeJSON(w, http.StatusOK, response)
}

// handleSendMessage adds a voice message to a room and starts delivering it.
//
// It accepts multipart/form-data with the "audio" (a WAV recording of up to
// 30 seconds), its "transcript", made on the phone, and "duration_ms";
// optionally the "lang" it was said in, which defaults to the sender's. An
// Idempotency-Key header, as for jobs, makes a resend safe.
func (a *API) handleSendMessage(w http.ResponseWriter, r *http.Request, user accounts.User, _ string) {
	roomID, ok := pathID(w, r, "room_not_found", "no such room")
	if !ok {
		return
	}
	idempotencyKey := r.Header.Get("Idempotency-Key")
	if idempotencyKey != "" && !idempotencyKeyPattern.MatchString(idempotencyKey) {
		writeError(w, http.StatusBadRequest, "invalid_idempotency_key", "Idempotency-Key must be 1 to 128 printable ASCII characters")
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, a.MaxUploadBytes)
	if err := r.ParseMultipartForm(formMemory); err != nil {
		var tooLarge *http.MaxBytesError
		if errors.As(err, &tooLarge) {
			writeError(w, http.StatusRequestEntityTooLarge, "audio_too_large", "the upload exceeds the size limit")
			return
		}
		writeError(w, http.StatusBadRequest, "invalid_form", "expected multipart/form-data")
		return
	}
	defer r.MultipartForm.RemoveAll()

	transcript := strings.TrimSpace(r.FormValue("transcript"))
	if transcript == "" || utf8.RuneCountInString(transcript) > maxTranscriptChars {
		writeError(w, http.StatusBadRequest, "invalid_transcript", fmt.Sprintf("the transcript is 1 to %d characters", maxTranscriptChars))
		return
	}
	durationMS, err := strconv.Atoi(r.FormValue("duration_ms"))
	if err != nil || durationMS < 1 || durationMS > maxMessageMS {
		writeError(w, http.StatusBadRequest, "invalid_duration", "a voice message is at most 30 seconds")
		return
	}
	lang := r.FormValue("lang")
	if lang == "" {
		lang = user.Language
	}
	if !languageTag.MatchString(lang) {
		writeError(w, http.StatusBadRequest, "invalid_lang", `lang must be a language tag such as "es"`)
		return
	}
	file, _, err := r.FormFile("audio")
	if err != nil {
		writeError(w, http.StatusBadRequest, "missing_audio", `the "audio" file is required`)
		return
	}
	defer file.Close()
	if !isWAV(file) {
		writeError(w, http.StatusBadRequest, "unsupported_audio", "the audio must be a WAV file")
		return
	}

	key := rand.Text() + ".wav"
	if err := a.Messages.Put(key, file); err != nil {
		a.Logger.Error("storing a message", "user_id", user.ID, "error", err)
		writeError(w, http.StatusInternalServerError, "internal", "could not store the message")
		return
	}
	message, created, err := a.Rooms.AddMessage(r.Context(), rooms.NewMessage{
		RoomID: roomID, SenderID: user.ID, Lang: lang, Transcript: transcript,
		AudioKey: key, DurationMS: durationMS, IdempotencyKey: idempotencyKey,
	})
	if err != nil || !created {
		// Not kept, or the same message was kept before, with its own copy.
		a.deleteMessageFile(key)
	}
	switch {
	case errors.Is(err, rooms.ErrNoRoom):
		writeError(w, http.StatusNotFound, "room_not_found", "no such room")
		return
	case errors.Is(err, rooms.ErrRoomClosed):
		writeError(w, http.StatusConflict, "room_closed", "no one else is in this room any more")
		return
	case err != nil:
		a.Logger.Error("adding a message", "user_id", user.ID, "room_id", roomID, "error", err)
		writeError(w, http.StatusInternalServerError, "internal", "could not send the message")
		return
	}
	status := http.StatusOK
	if created {
		a.Wake()
		status = http.StatusCreated
	}
	writeJSON(w, status, messageJSON(message, user))
}

func (a *API) handleMarkRead(w http.ResponseWriter, r *http.Request, user accounts.User, _ string) {
	roomID, ok := pathID(w, r, "room_not_found", "no such room")
	if !ok {
		return
	}
	if !a.roomFound(w, a.Rooms.MarkRead(r.Context(), user.ID, roomID), user) {
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (a *API) handleGetMessage(w http.ResponseWriter, r *http.Request, user accounts.User, _ string) {
	messageID, ok := pathID(w, r, "message_not_found", "no such message")
	if !ok {
		return
	}
	message, err := a.Rooms.Message(r.Context(), user.ID, messageID)
	if !a.messageFound(w, err, user) {
		return
	}
	writeJSON(w, http.StatusOK, messageJSON(message, user))
}

// handleRetryMessage tries again to deliver a message whose translation
// failed. Only its sender may ask.
func (a *API) handleRetryMessage(w http.ResponseWriter, r *http.Request, user accounts.User, _ string) {
	messageID, ok := pathID(w, r, "message_not_found", "no such message")
	if !ok {
		return
	}
	message, err := a.Rooms.Retry(r.Context(), user.ID, messageID)
	if !a.messageFound(w, err, user) {
		return
	}
	a.Wake()
	writeJSON(w, http.StatusOK, messageJSON(message, user))
}

// handleMessageAudio sends a message's audio in one language (?lang=): the
// sender's recording for the language it was said in, or for another, the
// translation spoken in the sender's voice, once it is ready.
func (a *API) handleMessageAudio(w http.ResponseWriter, r *http.Request, user accounts.User, _ string) {
	messageID, ok := pathID(w, r, "message_not_found", "no such message")
	if !ok {
		return
	}
	message, err := a.Rooms.Message(r.Context(), user.ID, messageID)
	if !a.messageFound(w, err, user) {
		return
	}
	lang := r.URL.Query().Get("lang")
	key := ""
	if lang == "" || lang == message.Lang {
		key = message.AudioKey
	}
	for _, rendition := range message.Renditions {
		if rendition.Lang != lang || rendition.Status != rooms.StatusReady {
			continue
		}
		switch rendition.Voice {
		case rooms.VoiceOwn:
			key = rendition.AudioKey
		case rooms.VoiceOriginal:
			key = message.AudioKey
		}
	}
	if key == "" {
		writeError(w, http.StatusNotFound, "no_audio", "there is no audio of this message in that language, or not yet")
		return
	}
	file, err := a.Messages.Open(key)
	if err != nil {
		a.Logger.Error("opening message audio", "message_id", messageID, "error", err)
		writeError(w, http.StatusInternalServerError, "internal", "could not read the audio")
		return
	}
	defer file.Close()
	w.Header().Set("Content-Type", "audio/wav")
	w.Header().Set("Cache-Control", "private, no-store")
	http.ServeContent(w, r, "message.wav", time.Time{}, file)
}

// handleRoomChanges streams a "room" event, as Server-Sent Events, whenever
// something in one of the user's rooms changes. The events carry only which
// room and message; the phone fetches what it shows. They are not kept, so a
// phone that reconnects fetches everything again instead of replaying.
func (a *API) handleRoomChanges(w http.ResponseWriter, r *http.Request, user accounts.User, _ string) {
	changes, unsubscribe := a.Changes.Subscribe(user.ID)
	defer unsubscribe()

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.WriteHeader(http.StatusOK)
	flusher := http.NewResponseController(w)

	// "ready" says the subscription is in place: anything the phone fetches
	// after it is at least as new as what the stream will report.
	if _, err := fmt.Fprint(w, "event: ready\ndata: {}\n\n"); err != nil {
		return
	}
	if err := flusher.Flush(); err != nil {
		return
	}

	heartbeat := time.NewTicker(a.Heartbeat)
	defer heartbeat.Stop()
	for {
		select {
		case <-r.Context().Done():
			return
		case <-a.Stopping:
			return
		case change, ok := <-changes:
			if !ok {
				return // cut off for falling behind; the phone reconnects
			}
			data, err := json.Marshal(change)
			if err != nil {
				return
			}
			if _, err := fmt.Fprintf(w, "event: room\ndata: %s\n\n", data); err != nil {
				return
			}
			if err := flusher.Flush(); err != nil {
				return
			}
		case <-heartbeat.C:
			if _, err := fmt.Fprint(w, ": keep-alive\n\n"); err != nil {
				return
			}
			if err := flusher.Flush(); err != nil {
				return
			}
		}
	}
}

// forgetMessages deletes what the user said in their rooms, and its audio,
// before their account goes. It logs its own errors.
func (a *API) forgetMessages(ctx context.Context, user accounts.User) error {
	keys, err := a.Rooms.ForgetSender(ctx, user.ID)
	if err != nil {
		a.Logger.Error("deleting a user's messages", "user_id", user.ID, "error", err)
		return err
	}
	for _, key := range keys {
		a.deleteMessageFile(key)
	}
	return nil
}

func (a *API) deleteMessageFile(key string) {
	if err := a.Messages.Delete(key); err != nil {
		a.Logger.Warn("deleting a message file", "error", err)
	}
}

// pathID returns the {id} in the path, or answers 404 itself when it is not
// a uuid, which Postgres would reject with an error.
func pathID(w http.ResponseWriter, r *http.Request, code, message string) (string, bool) {
	id := r.PathValue("id")
	if !uuidPattern.MatchString(id) {
		writeError(w, http.StatusNotFound, code, message)
		return "", false
	}
	return id, true
}

// roomFound answers the request itself when err is not nil, and reports
// whether the caller should go on.
func (a *API) roomFound(w http.ResponseWriter, err error, user accounts.User) bool {
	switch {
	case err == nil:
		return true
	case errors.Is(err, rooms.ErrNoRoom):
		writeError(w, http.StatusNotFound, "room_not_found", "no such room")
	default:
		a.Logger.Error("reading a room", "user_id", user.ID, "error", err)
		writeError(w, http.StatusInternalServerError, "internal", "could not read the room")
	}
	return false
}

// messageFound is roomFound for a message.
func (a *API) messageFound(w http.ResponseWriter, err error, user accounts.User) bool {
	switch {
	case err == nil:
		return true
	case errors.Is(err, rooms.ErrNoMessage):
		writeError(w, http.StatusNotFound, "message_not_found", "no such message")
	default:
		a.Logger.Error("reading a message", "user_id", user.ID, "error", err)
		writeError(w, http.StatusInternalServerError, "internal", "could not read the message")
	}
	return false
}
