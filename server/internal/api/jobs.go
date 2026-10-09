package api

import (
	"crypto/rand"
	"errors"
	"io"
	"net/http"
	"regexp"

	"github.com/PelumiWeb/translator-app-go/server/internal/queue"
)

// formMemory is how much of an upload is kept in RAM while it is parsed.
// Anything beyond it is spooled to a temporary file by the standard library.
const formMemory = 1 << 20 // 1 MB

// Loose check for a language tag such as "en", "yo" or "pt-BR". It keeps
// junk out of the database; the provider has the final say.
var languageTag = regexp.MustCompile(`^[A-Za-z]{2,3}(-[A-Za-z0-9]{2,8})*$`)

// An idempotency key is chosen by the client, typically a UUID: up to 128
// visible ASCII characters.
var idempotencyKeyPattern = regexp.MustCompile(`^[\x21-\x7E]{1,128}$`)

type createJobResponse struct {
	ID        string `json:"id"`
	Status    string `json:"status"`
	EventsURL string `json:"events_url"`
}

// handleCreateJob accepts multipart/form-data with an "audio" file (WAV) and
// a "source_lang" field. It stores the audio, enqueues a job, and answers
// 202 Accepted without waiting for the transcription.
//
// An optional Idempotency-Key header makes the request safe to send again:
// every request with the same key gets the same job.
func (a *API) handleCreateJob(w http.ResponseWriter, r *http.Request) {
	// Cap the body before anything reads it. Past the limit, reads fail and
	// the connection is closed, so a huge upload cannot fill the disk.
	r.Body = http.MaxBytesReader(w, r.Body, a.MaxUploadBytes)

	idempotencyKey := r.Header.Get("Idempotency-Key")
	if idempotencyKey != "" && !idempotencyKeyPattern.MatchString(idempotencyKey) {
		writeError(w, http.StatusBadRequest, "invalid_idempotency_key", "Idempotency-Key must be 1 to 128 visible ASCII characters")
		return
	}

	if err := r.ParseMultipartForm(formMemory); err != nil {
		// errors.As looks through wrapped errors for one of this type.
		var tooLarge *http.MaxBytesError
		if errors.As(err, &tooLarge) {
			writeError(w, http.StatusRequestEntityTooLarge, "audio_too_large", "the upload exceeds the size limit")
			return
		}
		writeError(w, http.StatusBadRequest, "invalid_form", "expected multipart/form-data")
		return
	}
	defer r.MultipartForm.RemoveAll() // deletes any temporary files

	sourceLang := r.FormValue("source_lang")
	if !languageTag.MatchString(sourceLang) {
		writeError(w, http.StatusBadRequest, "invalid_source_lang", `source_lang must be a language tag such as "en"`)
		return
	}

	file, _, err := r.FormFile("audio")
	if err != nil {
		writeError(w, http.StatusBadRequest, "missing_audio", `the "audio" file is required`)
		return
	}
	defer file.Close()

	if !isWAV(file) {
		writeError(w, http.StatusBadRequest, "unsupported_audio", "audio must be a WAV file")
		return
	}

	// The name is random and chosen here, never taken from the client, so a
	// crafted filename cannot point anywhere. rand.Text gives 26 characters.
	key := rand.Text() + ".wav"
	if err := a.Audio.Put(key, file); err != nil {
		a.Logger.Error("storing audio", "error", err)
		writeError(w, http.StatusInternalServerError, "internal", "could not store the audio")
		return
	}

	id, created, err := a.Jobs.EnqueueOnce(r.Context(), sourceLang, key, idempotencyKey)
	if err != nil {
		a.Logger.Error("enqueueing job", "error", err)
		a.deleteAudio(key) // no job refers to it, so nothing else would clean it up
		writeError(w, http.StatusInternalServerError, "internal", "could not create the job")
		return
	}

	status := queue.StatusQueued
	if created {
		a.Wake()
		a.Logger.Info("job enqueued", "job_id", id, "source_lang", sourceLang)
	} else {
		// A repeat of a request that already created its job. That job has
		// its own copy of the audio, so this one is surplus, and the job may
		// have moved on from "queued" by now.
		a.deleteAudio(key)
		if current, err := a.Jobs.Status(r.Context(), id); err == nil {
			status = current
		}
		a.Logger.Info("repeated upload matched an existing job", "job_id", id)
	}

	writeJSON(w, http.StatusAccepted, createJobResponse{
		ID:        id,
		Status:    status,
		EventsURL: "/v1/jobs/" + id + "/events",
	})
}

func (a *API) deleteAudio(key string) {
	if err := a.Audio.Delete(key); err != nil {
		a.Logger.Warn("deleting unused audio", "error", err)
	}
}

// isWAV checks the 12-byte RIFF/WAVE signature, then rewinds so the next
// reader starts at the beginning. It takes an io.ReadSeeker, the smallest
// interface that allows reading and rewinding.
func isWAV(f io.ReadSeeker) bool {
	var header [12]byte
	_, readErr := io.ReadFull(f, header[:])
	_, seekErr := f.Seek(0, io.SeekStart)
	if readErr != nil || seekErr != nil {
		return false
	}
	return string(header[0:4]) == "RIFF" && string(header[8:12]) == "WAVE"
}
