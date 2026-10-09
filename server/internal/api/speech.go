package api

import (
	"crypto/rand"
	"errors"
	"net/http"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/PelumiWeb/translator-app-go/server/internal/queue"
)

// maxSpeechChars caps the text of one request. A 30 second recording is a
// few hundred characters; this leaves room and stops abuse.
const maxSpeechChars = 2000

type createSpeechResponse struct {
	ID        string `json:"id"`
	Status    string `json:"status"`
	EventsURL string `json:"events_url"`
	AudioURL  string `json:"audio_url"`
}

// handleCreateSpeech asks for a text to be spoken in a given voice.
//
// It accepts multipart/form-data with a "voice" file (a WAV sample of the
// voice to imitate), the "text" to say, and its language as "lang". Like a
// transcription, it is queued and answered at once with 202 Accepted: follow
// events_url for progress, then fetch audio_url.
//
// The voice sample is used for this one request and deleted when the job
// ends. The server keeps no voices.
func (a *API) handleCreateSpeech(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, a.MaxUploadBytes)

	idempotencyKey := r.Header.Get("Idempotency-Key")
	if idempotencyKey != "" && !idempotencyKeyPattern.MatchString(idempotencyKey) {
		writeError(w, http.StatusBadRequest, "invalid_idempotency_key", "Idempotency-Key must be 1 to 128 visible ASCII characters")
		return
	}

	if err := r.ParseMultipartForm(formMemory); err != nil {
		var tooLarge *http.MaxBytesError
		if errors.As(err, &tooLarge) {
			writeError(w, http.StatusRequestEntityTooLarge, "voice_too_large", "the upload exceeds the size limit")
			return
		}
		writeError(w, http.StatusBadRequest, "invalid_form", "expected multipart/form-data")
		return
	}
	defer r.MultipartForm.RemoveAll()

	lang := r.FormValue("lang")
	if !languageTag.MatchString(lang) {
		writeError(w, http.StatusBadRequest, "invalid_lang", `lang must be a language tag such as "es"`)
		return
	}

	text := strings.TrimSpace(r.FormValue("text"))
	if text == "" {
		writeError(w, http.StatusBadRequest, "missing_text", `the "text" to speak is required`)
		return
	}
	// Counted in characters, not bytes: one accented or non-Latin letter is
	// several bytes, and should not count for more than a plain one.
	if utf8.RuneCountInString(text) > maxSpeechChars {
		writeError(w, http.StatusBadRequest, "text_too_long", "the text is too long to speak in one request")
		return
	}

	file, _, err := r.FormFile("voice")
	if err != nil {
		writeError(w, http.StatusBadRequest, "missing_voice", `the "voice" sample is required`)
		return
	}
	defer file.Close()
	if !isWAV(file) {
		writeError(w, http.StatusBadRequest, "unsupported_audio", "the voice sample must be a WAV file")
		return
	}

	key := rand.Text() + ".wav"
	if err := a.Audio.Put(key, file); err != nil {
		a.Logger.Error("storing voice sample", "error", err)
		writeError(w, http.StatusInternalServerError, "internal", "could not store the voice sample")
		return
	}

	id, created, err := a.Jobs.EnqueueSynthesis(r.Context(), lang, text, key, idempotencyKey)
	if err != nil {
		a.Logger.Error("enqueueing synthesis", "error", err)
		a.deleteAudio(key)
		writeError(w, http.StatusInternalServerError, "internal", "could not create the job")
		return
	}

	status := queue.StatusQueued
	if created {
		a.Wake()
		// The text is not logged: it is what the user said.
		a.Logger.Info("synthesis enqueued", "job_id", id, "lang", lang, "chars", utf8.RuneCountInString(text))
	} else {
		a.deleteAudio(key)
		if current, err := a.Jobs.Status(r.Context(), id); err == nil {
			status = current
		}
		a.Logger.Info("repeated request matched an existing job", "job_id", id)
	}

	writeJSON(w, http.StatusAccepted, createSpeechResponse{
		ID:        id,
		Status:    status,
		EventsURL: "/v1/jobs/" + id + "/events",
		AudioURL:  "/v1/jobs/" + id + "/audio",
	})
}

// handleJobAudio sends the audio a synthesis job produced.
func (a *API) handleJobAudio(w http.ResponseWriter, r *http.Request) {
	jobID := r.PathValue("id")
	if !uuidPattern.MatchString(jobID) {
		writeError(w, http.StatusNotFound, "job_not_found", "no such job")
		return
	}

	key, err := a.Jobs.ResultAudio(r.Context(), jobID)
	switch {
	case errors.Is(err, queue.ErrJobNotFound):
		writeError(w, http.StatusNotFound, "job_not_found", "no such speech job")
		return
	case errors.Is(err, queue.ErrNotReady):
		// 409 Conflict: the request is fine, the job is just not in a
		// state that has audio. The events stream says when it is.
		writeError(w, http.StatusConflict, "not_ready", "the audio is not available: the job is unfinished, failed, or its audio has expired")
		return
	case err != nil:
		a.Logger.Error("reading job result", "job_id", jobID, "error", err)
		writeError(w, http.StatusInternalServerError, "internal", "could not read the job")
		return
	}

	file, err := a.Audio.Open(key)
	if err != nil {
		a.Logger.Error("opening synthesized audio", "job_id", jobID, "error", err)
		writeError(w, http.StatusInternalServerError, "internal", "could not read the audio")
		return
	}
	defer file.Close()

	w.Header().Set("Content-Type", "audio/wav")
	// It is this user's voice saying this user's words: keep it out of any
	// shared cache between here and the phone.
	w.Header().Set("Cache-Control", "private, no-store")
	// As for model files, ServeContent gives range requests for free. The
	// zero time means "no Last-Modified header".
	http.ServeContent(w, r, "speech.wav", time.Time{}, file)
}
