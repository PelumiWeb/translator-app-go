package api

import (
	"bytes"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/PelumiWeb/translator-app-go/server/internal/queue"
)

// speechRequest builds a multipart POST /v1/speech. Empty strings and a nil
// voice leave that part out.
func speechRequest(t *testing.T, lang, text string, voice []byte) *http.Request {
	t.Helper()
	var body bytes.Buffer
	form := multipart.NewWriter(&body)
	for name, value := range map[string]string{"lang": lang, "text": text} {
		if value != "" {
			if err := form.WriteField(name, value); err != nil {
				t.Fatal(err)
			}
		}
	}
	if voice != nil {
		part, err := form.CreateFormFile("voice", "my-voice.wav")
		if err != nil {
			t.Fatal(err)
		}
		if _, err := part.Write(voice); err != nil {
			t.Fatal(err)
		}
	}
	if err := form.Close(); err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodPost, "/v1/speech", &body)
	req.Header.Set("Content-Type", form.FormDataContentType())
	return req
}

func TestCreateSpeech(t *testing.T) {
	a := newTestAPI()
	jobs := &fakeJobs{}
	audio := &fakeAudio{}
	woken := 0
	a.Jobs, a.Audio, a.Wake = jobs, audio, func() { woken++ }

	voice := wavBytes(500)
	rec := httptest.NewRecorder()
	a.Handler().ServeHTTP(rec, speechRequest(t, "es", "  Buenos días  ", voice))

	if rec.Code != http.StatusAccepted {
		t.Fatalf("status = %d, want 202; body: %s", rec.Code, rec.Body)
	}
	var got createSpeechResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("decoding response: %v", err)
	}
	want := createSpeechResponse{
		ID: testJobID, Status: "queued",
		EventsURL: "/v1/jobs/" + testJobID + "/events",
		AudioURL:  "/v1/jobs/" + testJobID + "/audio",
	}
	if got != want {
		t.Errorf("response = %+v, want %+v", got, want)
	}

	keys := audio.keys()
	if len(keys) != 1 || audio.blobs[keys[0]] != string(voice) {
		t.Fatalf("stored blobs = %v, want the voice sample", keys)
	}
	// The text is trimmed, and the stored name is the server's, not the client's.
	if len(jobs.synthesized) != 1 || jobs.synthesized[0] != "es | Buenos días | "+keys[0] {
		t.Errorf("synthesis jobs = %v", jobs.synthesized)
	}
	if woken != 1 {
		t.Errorf("Wake called %d times, want 1", woken)
	}
}

func TestCreateSpeechRejectsBadInput(t *testing.T) {
	tests := []struct {
		name       string
		lang, text string
		voice      []byte
		wantStatus int
		wantCode   string
	}{
		{"missing language", "", "hola", wavBytes(10), http.StatusBadRequest, "invalid_lang"},
		{"malformed language", "spanish!", "hola", wavBytes(10), http.StatusBadRequest, "invalid_lang"},
		{"missing text", "es", "", wavBytes(10), http.StatusBadRequest, "missing_text"},
		{"text of only spaces", "es", "   ", wavBytes(10), http.StatusBadRequest, "missing_text"},
		{"text too long", "es", strings.Repeat("a", maxSpeechChars+1), wavBytes(10), http.StatusBadRequest, "text_too_long"},
		{"missing voice", "es", "hola", nil, http.StatusBadRequest, "missing_voice"},
		{"voice is not a wav", "es", "hola", []byte("ID3 an mp3, honest"), http.StatusBadRequest, "unsupported_audio"},
		{"voice over the size limit", "es", "hola", wavBytes(8192), http.StatusRequestEntityTooLarge, "voice_too_large"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := newTestAPI()
			jobs := &fakeJobs{}
			audio := &fakeAudio{}
			a.Jobs, a.Audio = jobs, audio
			a.MaxUploadBytes = 4096

			rec := httptest.NewRecorder()
			a.Handler().ServeHTTP(rec, speechRequest(t, tt.lang, tt.text, tt.voice))

			if rec.Code != tt.wantStatus {
				t.Errorf("status = %d, want %d; body: %s", rec.Code, tt.wantStatus, rec.Body)
			}
			var body errorBody
			if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
				t.Fatalf("decoding error body %q: %v", rec.Body, err)
			}
			if body.Error.Code != tt.wantCode {
				t.Errorf("error code = %q, want %q", body.Error.Code, tt.wantCode)
			}
			if len(jobs.synthesized) != 0 || len(audio.keys()) != 0 {
				t.Error("a rejected request left a job or a voice sample behind")
			}
		})
	}
}

// The limit is on characters. Text in a script that takes three bytes a
// letter must be allowed as many letters as English.
func TestCreateSpeechCountsCharactersNotBytes(t *testing.T) {
	a := newTestAPI()
	a.MaxUploadBytes = 1 << 20
	text := strings.Repeat("語", maxSpeechChars) // 2000 characters, 6000 bytes

	rec := httptest.NewRecorder()
	a.Handler().ServeHTTP(rec, speechRequest(t, "ja", text, wavBytes(10)))

	if rec.Code != http.StatusAccepted {
		t.Errorf("status = %d, want 202; body: %s", rec.Code, rec.Body)
	}
}

func TestCreateSpeechWithTheSameIdempotencyKeyReturnsTheSameJob(t *testing.T) {
	a := newTestAPI()
	jobs := &fakeJobs{status: "done"}
	audio := &fakeAudio{}
	a.Jobs, a.Audio = jobs, audio

	for range 2 {
		req := speechRequest(t, "es", "hola", wavBytes(100))
		req.Header.Set("Idempotency-Key", "speech-attempt-1")
		rec := httptest.NewRecorder()
		a.Handler().ServeHTTP(rec, req)
		if rec.Code != http.StatusAccepted {
			t.Fatalf("status = %d, want 202", rec.Code)
		}
	}

	if len(jobs.synthesized) != 1 {
		t.Errorf("%d jobs created, want 1", len(jobs.synthesized))
	}
	if len(audio.keys()) != 1 || len(audio.deleted) != 1 {
		t.Errorf("stored = %v, deleted = %v; want the repeat's sample deleted", audio.keys(), audio.deleted)
	}
}

func TestJobAudio(t *testing.T) {
	speech := string(wavBytes(100))
	newAPI := func(jobs *fakeJobs) *API {
		a := newTestAPI()
		a.Jobs = jobs
		a.Audio = &fakeAudio{blobs: map[string]string{"speech.wav": speech}}
		return a
	}
	const path = "/v1/jobs/" + testJobID + "/audio"

	t.Run("sends the audio when the job is done", func(t *testing.T) {
		rec := get(newAPI(&fakeJobs{resultKey: "speech.wav"}), path, nil)

		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200; body: %s", rec.Code, rec.Body)
		}
		if rec.Body.String() != speech {
			t.Error("the body is not the stored audio")
		}
		if got := rec.Header().Get("Content-Type"); got != "audio/wav" {
			t.Errorf("Content-Type = %q, want audio/wav", got)
		}
		if got := rec.Header().Get("Cache-Control"); got != "private, no-store" {
			t.Errorf("Cache-Control = %q: a voice must not sit in shared caches", got)
		}
	})

	t.Run("supports range requests", func(t *testing.T) {
		rec := get(newAPI(&fakeJobs{resultKey: "speech.wav"}), path, map[string]string{"Range": "bytes=0-3"})

		if rec.Code != http.StatusPartialContent || rec.Body.String() != "RIFF" {
			t.Errorf("status = %d, body = %q; want 206 and the first four bytes", rec.Code, rec.Body)
		}
	})

	t.Run("says so when the job has no audio yet", func(t *testing.T) {
		rec := get(newAPI(&fakeJobs{resultErr: queue.ErrNotReady}), path, nil)

		if rec.Code != http.StatusConflict || !strings.Contains(rec.Body.String(), `"not_ready"`) {
			t.Errorf("status = %d, body = %s; want 409 not_ready", rec.Code, rec.Body)
		}
	})

	t.Run("unknown job", func(t *testing.T) {
		for _, p := range []string{path, "/v1/jobs/not-a-uuid/audio"} {
			rec := get(newAPI(&fakeJobs{resultErr: queue.ErrJobNotFound}), p, nil)
			if rec.Code != http.StatusNotFound {
				t.Errorf("GET %s: status = %d, want 404", p, rec.Code)
			}
		}
	})
}
