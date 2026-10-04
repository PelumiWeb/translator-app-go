package api

import (
	"bytes"
	"encoding/json"
	"errors"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// wavBytes returns something with a valid WAV signature and n bytes of body.
func wavBytes(n int) []byte {
	return append([]byte("RIFF\x00\x00\x00\x00WAVE"), bytes.Repeat([]byte{0}, n)...)
}

// uploadRequest builds a multipart POST /v1/jobs. An empty lang or nil audio
// leaves that part out.
func uploadRequest(t *testing.T, lang string, audio []byte) *http.Request {
	t.Helper()
	var body bytes.Buffer
	form := multipart.NewWriter(&body)
	if lang != "" {
		if err := form.WriteField("source_lang", lang); err != nil {
			t.Fatal(err)
		}
	}
	if audio != nil {
		part, err := form.CreateFormFile("audio", "whatever-the-client-says.wav")
		if err != nil {
			t.Fatal(err)
		}
		if _, err := part.Write(audio); err != nil {
			t.Fatal(err)
		}
	}
	if err := form.Close(); err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodPost, "/v1/jobs", &body)
	req.Header.Set("Content-Type", form.FormDataContentType())
	return req
}

func TestCreateJob(t *testing.T) {
	a := newTestAPI()
	jobs := &fakeJobs{}
	audio := &fakeAudio{}
	woken := 0
	a.Jobs, a.Audio, a.Wake = jobs, audio, func() { woken++ }

	clip := wavBytes(100)
	rec := httptest.NewRecorder()
	a.Handler().ServeHTTP(rec, uploadRequest(t, "en", clip))

	if rec.Code != http.StatusAccepted {
		t.Fatalf("status = %d, want 202; body: %s", rec.Code, rec.Body)
	}
	var got createJobResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("decoding response: %v", err)
	}
	want := createJobResponse{ID: testJobID, Status: "queued", EventsURL: "/v1/jobs/" + testJobID + "/events"}
	if got != want {
		t.Errorf("response = %+v, want %+v", got, want)
	}

	keys := audio.keys()
	if len(keys) != 1 {
		t.Fatalf("stored %d blobs, want 1", len(keys))
	}
	if audio.blobs[keys[0]] != string(clip) {
		t.Error("stored audio differs from the upload")
	}
	if strings.Contains(keys[0], "whatever") {
		t.Errorf("storage key %q uses the client's filename", keys[0])
	}
	if len(jobs.enqueued) != 1 || jobs.enqueued[0] != "en "+keys[0] {
		t.Errorf("enqueued = %v, want one job for en and key %s", jobs.enqueued, keys[0])
	}
	if woken != 1 {
		t.Errorf("Wake called %d times, want 1", woken)
	}
}

func TestCreateJobRejectsBadInput(t *testing.T) {
	tests := []struct {
		name       string
		lang       string
		audio      []byte
		wantStatus int
		wantCode   string
	}{
		{"missing language", "", wavBytes(10), http.StatusBadRequest, "invalid_source_lang"},
		{"malformed language", "english!", wavBytes(10), http.StatusBadRequest, "invalid_source_lang"},
		{"missing audio", "en", nil, http.StatusBadRequest, "missing_audio"},
		{"not a wav file", "en", []byte("ID3 this is an mp3, honest"), http.StatusBadRequest, "unsupported_audio"},
		{"too short to be a wav", "en", []byte("RIFF"), http.StatusBadRequest, "unsupported_audio"},
		{"over the size limit", "en", wavBytes(4096), http.StatusRequestEntityTooLarge, "audio_too_large"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := newTestAPI()
			jobs := &fakeJobs{}
			audio := &fakeAudio{}
			a.Jobs, a.Audio = jobs, audio
			a.MaxUploadBytes = 2048

			rec := httptest.NewRecorder()
			a.Handler().ServeHTTP(rec, uploadRequest(t, tt.lang, tt.audio))

			if rec.Code != tt.wantStatus {
				t.Errorf("status = %d, want %d", rec.Code, tt.wantStatus)
			}
			var body errorBody
			if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
				t.Fatalf("decoding error body %q: %v", rec.Body, err)
			}
			if body.Error.Code != tt.wantCode {
				t.Errorf("error code = %q, want %q", body.Error.Code, tt.wantCode)
			}
			if len(jobs.enqueued) != 0 || len(audio.keys()) != 0 {
				t.Error("a rejected upload left a job or audio behind")
			}
		})
	}
}

func TestCreateJobRejectsNonMultipartBody(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/v1/jobs", strings.NewReader(`{"audio":"nope"}`))
	req.Header.Set("Content-Type", "application/json")

	rec := httptest.NewRecorder()
	newTestAPI().Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", rec.Code)
	}
}

func TestCreateJobDeletesAudioWhenEnqueueFails(t *testing.T) {
	a := newTestAPI()
	audio := &fakeAudio{}
	a.Audio = audio
	a.Jobs = &fakeJobs{enqueueErr: errors.New("database is down")}

	rec := httptest.NewRecorder()
	a.Handler().ServeHTTP(rec, uploadRequest(t, "en", wavBytes(10)))

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want 500", rec.Code)
	}
	if strings.Contains(rec.Body.String(), "database is down") {
		t.Error("the response leaks the internal error message")
	}
	if len(audio.keys()) != 0 || len(audio.deleted) != 1 {
		t.Errorf("blobs = %v, deleted = %v; want the stored audio deleted", audio.keys(), audio.deleted)
	}
}
