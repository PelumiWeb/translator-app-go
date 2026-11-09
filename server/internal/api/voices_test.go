package api

import (
	"bytes"
	"context"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"

	"github.com/PelumiWeb/translator-app-go/server/internal/accounts"
)

// voiceRequest builds a multipart PUT /v1/me/voice. An empty consent or nil
// voice leaves that part out.
func voiceRequest(t *testing.T, token, consent string, voice []byte) *http.Request {
	t.Helper()
	var body bytes.Buffer
	form := multipart.NewWriter(&body)
	if consent != "" {
		if err := form.WriteField("consent", consent); err != nil {
			t.Fatal(err)
		}
	}
	if voice != nil {
		part, err := form.CreateFormFile("voice", "voice.wav")
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
	req := httptest.NewRequest(http.MethodPut, "/v1/me/voice", &body)
	req.Header.Set("Content-Type", form.FormDataContentType())
	req.Header.Set("Authorization", "Bearer "+token)
	return req
}

// signedInAPI is an API with one signed-in user, Ada, and her session token.
func signedInAPI() (*API, string) {
	a, store := accountsAPI()
	token, _, _, _ := store.SignIn(context.Background(), accounts.ProviderApple, "apple-1", "", "Ada")
	return a, token
}

func serve(a *API, req *http.Request) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	a.Handler().ServeHTTP(rec, req)
	return rec
}

func TestPutVoiceKeepsItWithTheConsent(t *testing.T) {
	a, token := signedInAPI()

	rec := serve(a, voiceRequest(t, token, "2026-10-31", wavBytes(100)))

	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"consent_version":"2026-10-31"`) {
		t.Fatalf("status %d, body %s", rec.Code, rec.Body)
	}
	if kept := a.Voices.(*fakeAudio).keys(); len(kept) != 1 {
		t.Errorf("voice files = %v, want one", kept)
	}
	if rec := send(a, http.MethodGet, "/v1/me/voice", token, ""); rec.Code != http.StatusOK {
		t.Errorf("GET /v1/me/voice: status %d, body %s", rec.Code, rec.Body)
	}
	audio := send(a, http.MethodGet, "/v1/me/voice/audio", token, "")
	if audio.Code != http.StatusOK || !bytes.Equal(audio.Body.Bytes(), wavBytes(100)) {
		t.Errorf("GET /v1/me/voice/audio: status %d, %d bytes", audio.Code, audio.Body.Len())
	}
}

func TestPutVoiceAgainDeletesTheOldRecording(t *testing.T) {
	a, token := signedInAPI()
	serve(a, voiceRequest(t, token, "2026-10-31", wavBytes(100)))
	first := a.Voices.(*fakeAudio).keys()

	serve(a, voiceRequest(t, token, "2026-10-31", wavBytes(200)))

	now := a.Voices.(*fakeAudio).keys()
	if len(now) != 1 || slices.Equal(now, first) {
		t.Errorf("voice files before %v, after %v: want only the new one", first, now)
	}
}

func TestPutVoiceRejectsBadInput(t *testing.T) {
	tests := []struct {
		name     string
		consent  string
		voice    []byte
		wantCode string
	}{
		{"no consent", "", wavBytes(100), "missing_consent"},
		{"a consent that is not a version", "I agree!", wavBytes(100), "missing_consent"},
		{"no recording", "2026-10-31", nil, "missing_voice"},
		{"not a WAV", "2026-10-31", []byte("ID3 an mp3, say"), "unsupported_audio"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a, token := signedInAPI()

			rec := serve(a, voiceRequest(t, token, tt.consent, tt.voice))

			if rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), `"code":"`+tt.wantCode+`"`) {
				t.Errorf("status %d, body %s; want 400 %s", rec.Code, rec.Body, tt.wantCode)
			}
			if kept := a.Voices.(*fakeAudio).keys(); len(kept) != 0 {
				t.Errorf("voice files = %v, want none", kept)
			}
		})
	}
}

func TestNoVoiceUntilOneIsRecorded(t *testing.T) {
	a, token := signedInAPI()

	for _, path := range []string{"/v1/me/voice", "/v1/me/voice/audio"} {
		if rec := send(a, http.MethodGet, path, token, ""); rec.Code != http.StatusNotFound || !strings.Contains(rec.Body.String(), `"no_voice"`) {
			t.Errorf("GET %s: status %d, body %s; want 404 no_voice", path, rec.Code, rec.Body)
		}
	}
}

func TestDeleteVoiceDeletesTheRecording(t *testing.T) {
	a, token := signedInAPI()
	serve(a, voiceRequest(t, token, "2026-10-31", wavBytes(100)))

	if rec := send(a, http.MethodDelete, "/v1/me/voice", token, ""); rec.Code != http.StatusNoContent {
		t.Fatalf("status %d, body %s; want 204", rec.Code, rec.Body)
	}
	if kept := a.Voices.(*fakeAudio).keys(); len(kept) != 0 {
		t.Errorf("voice files = %v, want none", kept)
	}
	if rec := send(a, http.MethodGet, "/v1/me/voice", token, ""); rec.Code != http.StatusNotFound {
		t.Errorf("afterwards: status %d, want 404", rec.Code)
	}
	// Once more: still no voice, so still a success.
	if rec := send(a, http.MethodDelete, "/v1/me/voice", token, ""); rec.Code != http.StatusNoContent {
		t.Errorf("deleting twice: status %d, want 204", rec.Code)
	}
}

func TestDeleteMeDeletesTheRecording(t *testing.T) {
	a, token := signedInAPI()
	serve(a, voiceRequest(t, token, "2026-10-31", wavBytes(100)))

	if rec := send(a, http.MethodDelete, "/v1/me", token, ""); rec.Code != http.StatusNoContent {
		t.Fatalf("status %d, body %s; want 204", rec.Code, rec.Body)
	}
	if kept := a.Voices.(*fakeAudio).keys(); len(kept) != 0 {
		t.Errorf("voice files = %v, want none", kept)
	}
}

func TestSpeechInTheKeptVoice(t *testing.T) {
	a, token := signedInAPI()
	serve(a, voiceRequest(t, token, "2026-10-31", wavBytes(100)))
	req := speechRequest(t, "es", "hola", nil) // no voice file
	req.Header.Set("Authorization", "Bearer "+token)

	rec := serve(a, req)

	if rec.Code != http.StatusAccepted {
		t.Fatalf("status %d, body %s; want 202", rec.Code, rec.Body)
	}
	// The job has its own copy, which it may delete; the kept voice stays.
	jobBlobs := a.Audio.(*fakeAudio).keys()
	if len(jobBlobs) != 1 || a.Audio.(*fakeAudio).blobs[jobBlobs[0]] != string(wavBytes(100)) {
		t.Errorf("job audio = %v, want one copy of the kept voice", jobBlobs)
	}
	if kept := a.Voices.(*fakeAudio).keys(); len(kept) != 1 || kept[0] == jobBlobs[0] {
		t.Errorf("voice files = %v, want the kept voice untouched", kept)
	}
}

func TestSpeechWithoutAVoiceFile(t *testing.T) {
	tests := []struct {
		name       string
		token      string // "ada" stands for Ada's real token
		wantStatus int
		wantCode   string
	}{
		{"and not signed in", "", http.StatusUnauthorized, "unauthenticated"},
		{"from an ended session", "made-up", http.StatusUnauthorized, "unauthenticated"},
		{"from a user with no voice yet", "ada", http.StatusConflict, "no_voice"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a, token := signedInAPI()
			req := speechRequest(t, "es", "hola", nil)
			switch tt.token {
			case "":
				req.Header.Del("Authorization")
			case "ada":
				req.Header.Set("Authorization", "Bearer "+token)
			default:
				req.Header.Set("Authorization", "Bearer "+tt.token)
			}

			rec := serve(a, req)

			if rec.Code != tt.wantStatus || !strings.Contains(rec.Body.String(), `"code":"`+tt.wantCode+`"`) {
				t.Errorf("status %d, body %s; want %d %s", rec.Code, rec.Body, tt.wantStatus, tt.wantCode)
			}
		})
	}
}
