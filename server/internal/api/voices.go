package api

import (
	"context"
	"crypto/rand"
	"errors"
	"io"
	"net/http"
	"regexp"
	"time"

	"github.com/PelumiWeb/translator-app-go/server/internal/accounts"
)

// consentPattern matches the name of a version of the consent wording, such
// as "2026-10-31". The phones send the version the user agreed to, and the
// server keeps it, so it is known later exactly what each person was told.
var consentPattern = regexp.MustCompile(`^[A-Za-z0-9._-]{1,40}$`)

type voiceResponse struct {
	ConsentVersion string    `json:"consent_version"`
	ConsentedAt    time.Time `json:"consented_at"`
}

func voiceJSON(v accounts.Voice) voiceResponse {
	return voiceResponse{ConsentVersion: v.ConsentVersion, ConsentedAt: v.ConsentedAt}
}

// handlePutVoice keeps the user's voice sample, replacing any earlier one.
//
// It accepts multipart/form-data with the "voice" (a WAV recording of the
// user reading the onboarding passage) and "consent", the version of the
// consent wording the user agreed to before recording it.
func (a *API) handlePutVoice(w http.ResponseWriter, r *http.Request, user accounts.User, _ string) {
	r.Body = http.MaxBytesReader(w, r.Body, a.MaxUploadBytes)
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

	consent := r.FormValue("consent")
	if !consentPattern.MatchString(consent) {
		writeError(w, http.StatusBadRequest, "missing_consent", `"consent" must name the consent wording the user agreed to`)
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

	// A new key for each upload, never the user's ID: if this upload fails
	// halfway, the sample it was replacing is still whole.
	key := rand.Text() + ".wav"
	if err := a.Voices.Put(key, file); err != nil {
		a.Logger.Error("storing a voice", "user_id", user.ID, "error", err)
		writeError(w, http.StatusInternalServerError, "internal", "could not store the voice")
		return
	}
	voice, replaced, err := a.Accounts.SetVoice(r.Context(), user.ID, key, consent)
	if err != nil {
		a.deleteVoiceBlob(key)
		if errors.Is(err, accounts.ErrNoUser) {
			writeError(w, http.StatusUnauthorized, "unauthenticated", "the account no longer exists")
			return
		}
		a.Logger.Error("keeping a voice", "user_id", user.ID, "error", err)
		writeError(w, http.StatusInternalServerError, "internal", "could not keep the voice")
		return
	}
	if replaced != "" {
		a.deleteVoiceBlob(replaced)
	}
	a.Logger.Info("voice kept", "user_id", user.ID, "consent", consent)
	writeJSON(w, http.StatusOK, voiceJSON(voice))
}

// handleGetVoice says whether the user has a kept voice, and what they agreed to.
func (a *API) handleGetVoice(w http.ResponseWriter, r *http.Request, user accounts.User, _ string) {
	voice, ok := a.keptVoice(r.Context(), w, user)
	if !ok {
		return
	}
	writeJSON(w, http.StatusOK, voiceJSON(voice))
}

// handleVoiceAudio sends the user their own kept voice sample, so a phone they
// sign in on later can play it back. Nobody else's sample is ever sent.
func (a *API) handleVoiceAudio(w http.ResponseWriter, r *http.Request, user accounts.User, _ string) {
	voice, ok := a.keptVoice(r.Context(), w, user)
	if !ok {
		return
	}
	file, err := a.Voices.Open(voice.BlobKey)
	if err != nil {
		a.Logger.Error("opening a kept voice", "user_id", user.ID, "error", err)
		writeError(w, http.StatusInternalServerError, "internal", "could not read the voice")
		return
	}
	defer file.Close()

	w.Header().Set("Content-Type", "audio/wav")
	w.Header().Set("Cache-Control", "private, no-store")
	http.ServeContent(w, r, "voice.wav", time.Time{}, file)
}

// handleDeleteVoice deletes the user's kept voice. Deleting one that is not
// there succeeds too: either way, there is no voice afterwards.
func (a *API) handleDeleteVoice(w http.ResponseWriter, r *http.Request, user accounts.User, _ string) {
	if err := a.deleteKeptVoice(r.Context(), user); err != nil {
		writeError(w, http.StatusInternalServerError, "internal", "could not delete the voice")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// keptVoice returns the user's voice, or answers 404 itself when there is
// none, in which case it returns false.
func (a *API) keptVoice(ctx context.Context, w http.ResponseWriter, user accounts.User) (accounts.Voice, bool) {
	voice, err := a.Accounts.Voice(ctx, user.ID)
	switch {
	case errors.Is(err, accounts.ErrNoVoice):
		writeError(w, http.StatusNotFound, "no_voice", "no voice has been recorded")
		return accounts.Voice{}, false
	case err != nil:
		a.Logger.Error("reading a voice", "user_id", user.ID, "error", err)
		writeError(w, http.StatusInternalServerError, "internal", "could not read the voice")
		return accounts.Voice{}, false
	}
	return voice, true
}

// deleteKeptVoice forgets the user's voice and deletes its audio. It logs its
// own errors, so callers only answer the request.
func (a *API) deleteKeptVoice(ctx context.Context, user accounts.User) error {
	key, err := a.Accounts.DeleteVoice(ctx, user.ID)
	if errors.Is(err, accounts.ErrNoVoice) {
		return nil
	}
	if err != nil {
		a.Logger.Error("deleting a voice", "user_id", user.ID, "error", err)
		return err
	}
	a.deleteVoiceBlob(key)
	a.Logger.Info("voice deleted", "user_id", user.ID)
	return nil
}

func (a *API) deleteVoiceBlob(key string) {
	if err := a.Voices.Delete(key); err != nil {
		a.Logger.Warn("deleting a voice file", "error", err)
	}
}

// speechVoice stores the voice to speak in as a new job blob and returns its
// key, or answers the request itself and returns false. The voice is the
// uploaded "voice" file when there is one; otherwise the signed-in user's
// kept voice, copied, since the job deletes its blob when it ends.
func (a *API) speechVoice(w http.ResponseWriter, r *http.Request, user accounts.User) (string, bool) {
	var voice io.Reader
	if file, _, err := r.FormFile("voice"); err == nil {
		defer file.Close()
		if !isWAV(file) {
			writeError(w, http.StatusBadRequest, "unsupported_audio", "the voice sample must be a WAV file")
			return "", false
		}
		voice = file
	} else {
		kept, err := a.Accounts.Voice(r.Context(), user.ID)
		switch {
		case errors.Is(err, accounts.ErrNoVoice):
			// 409: the request is fine, but the account is not ready for it.
			writeError(w, http.StatusConflict, "no_voice", "record your voice first")
			return "", false
		case err != nil:
			a.Logger.Error("reading a voice", "user_id", user.ID, "error", err)
			writeError(w, http.StatusInternalServerError, "internal", "could not read the voice")
			return "", false
		}
		file, err := a.Voices.Open(kept.BlobKey)
		if err != nil {
			a.Logger.Error("opening a kept voice", "user_id", user.ID, "error", err)
			writeError(w, http.StatusInternalServerError, "internal", "could not read the voice")
			return "", false
		}
		defer file.Close()
		voice = file
	}

	key := rand.Text() + ".wav"
	if err := a.Audio.Put(key, voice); err != nil {
		a.Logger.Error("storing voice sample", "error", err)
		writeError(w, http.StatusInternalServerError, "internal", "could not store the voice sample")
		return "", false
	}
	return key, true
}
