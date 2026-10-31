package fish

import (
	"bytes"
	"context"
	"encoding/binary"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/PelumiWeb/translator-app-go/server/internal/provider"
)

func TestSynthesizeSendsTheSampleWithItsTranscriptAndWritesWAV(t *testing.T) {
	sample := []byte("RIFF....WAVE the voice")
	pcm := []byte{1, 0, 2, 0, 3, 0, 9} // three samples and a stray byte

	fish := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/v1/tts" {
			t.Errorf("request = %s %s, want POST /v1/tts", r.Method, r.URL.Path)
		}
		for header, want := range map[string]string{
			"Authorization": "Bearer test-key",
			"Content-Type":  "application/msgpack",
			"Model":         "s2.1-pro",
		} {
			if got := r.Header.Get(header); got != want {
				t.Errorf("%s = %q, want %q", header, got, want)
			}
		}
		body, _ := io.ReadAll(r.Body)
		if want := encodeRequest("buenos días", "es", sample, "the passage"); !bytes.Equal(body, want) {
			t.Errorf("body is not the request for this text, sample and transcript")
		}
		w.Write(pcm)
	}))
	defer fish.Close()

	var out bytes.Buffer
	s := Synthesizer{APIKey: "test-key", Model: "s2.1-pro", VoiceText: "the passage", BaseURL: fish.URL}
	opts := provider.SynthesisOptions{Text: "  buenos días ", Language: "es", Attempt: 1}
	if err := s.Synthesize(context.Background(), bytes.NewReader(sample), opts, &out); err != nil {
		t.Fatalf("Synthesize: %v", err)
	}

	wav := out.Bytes()
	if len(wav) != 44+6 || string(wav[:4]) != "RIFF" || string(wav[8:12]) != "WAVE" {
		t.Fatalf("output is not a WAV of the three samples: % x", wav)
	}
	if rate := binary.LittleEndian.Uint32(wav[24:28]); rate != 16_000 {
		t.Errorf("sample rate = %d, want 16000", rate)
	}
	if size := binary.LittleEndian.Uint32(wav[40:44]); size != 6 {
		t.Errorf("data size = %d, want 6", size)
	}
	if !bytes.Equal(wav[44:], pcm[:6]) {
		t.Errorf("samples = % x, want % x", wav[44:], pcm[:6])
	}
}

func TestSynthesizeTellsPassingFailuresFromLastingOnes(t *testing.T) {
	tests := []struct {
		status    int
		permanent bool
	}{
		{http.StatusTooManyRequests, false},
		{http.StatusInternalServerError, false},
		{http.StatusServiceUnavailable, false},
		{http.StatusBadRequest, true},
		{http.StatusUnauthorized, true},    // a bad key
		{http.StatusPaymentRequired, true}, // no credit
	}
	for _, tt := range tests {
		t.Run(http.StatusText(tt.status), func(t *testing.T) {
			fish := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				http.Error(w, "nope", tt.status)
			}))
			defer fish.Close()

			s := Synthesizer{APIKey: "k", VoiceText: "p", BaseURL: fish.URL}
			err := s.Synthesize(context.Background(), strings.NewReader("voice"), provider.SynthesisOptions{Text: "hola"}, io.Discard)

			if err == nil {
				t.Fatal("no error")
			}
			if provider.IsPermanent(err) != tt.permanent {
				t.Errorf("permanent = %v, want %v (%v)", provider.IsPermanent(err), tt.permanent, err)
			}
			if !strings.Contains(err.Error(), "nope") {
				t.Errorf("error %q does not carry Fish Audio's reason", err)
			}
		})
	}
}

func TestSynthesizeDoesNotAskFishToSayNothing(t *testing.T) {
	var calls atomic.Int32
	fish := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { calls.Add(1) }))
	defer fish.Close()

	s := Synthesizer{APIKey: "k", BaseURL: fish.URL}
	err := s.Synthesize(context.Background(), strings.NewReader("voice"), provider.SynthesisOptions{Text: "  "}, io.Discard)

	if !provider.IsPermanent(err) {
		t.Errorf("error = %v, want a permanent one", err)
	}
	if calls.Load() != 0 {
		t.Errorf("Fish Audio was called %d times", calls.Load())
	}
}

func TestSynthesizeWhenFishCannotBeReachedIsWorthRetrying(t *testing.T) {
	fish := httptest.NewServer(http.NotFoundHandler())
	fish.Close() // nothing listens there any more

	s := Synthesizer{APIKey: "k", BaseURL: fish.URL, Client: &http.Client{Timeout: time.Second}}
	err := s.Synthesize(context.Background(), strings.NewReader("voice"), provider.SynthesisOptions{Text: "hola"}, io.Discard)

	if err == nil || provider.IsPermanent(err) {
		t.Errorf("error = %v, want a temporary one", err)
	}
}

// TestLiveSynthesis calls the real Fish Audio once, with whisper.cpp's sample
// clip as the voice. It costs a fraction of a cent, so it runs only when
// FISH_AUDIO_API_KEY is set (`make voice-check` does that from server/.env).
// FISH_LIVE_OUT names a file to keep the result in, to listen to it.
func TestLiveSynthesis(t *testing.T) {
	key := os.Getenv("FISH_AUDIO_API_KEY")
	if key == "" {
		t.Skip("FISH_AUDIO_API_KEY is not set")
	}
	clip, err := os.ReadFile("../../../../third_party/whisper.cpp/samples/jfk.wav")
	if err != nil {
		t.Skipf("whisper.cpp's sample clip is missing (git submodule update --init): %v", err)
	}

	s := Synthesizer{
		APIKey:    key,
		Model:     os.Getenv("FISH_AUDIO_MODEL"),
		VoiceText: "And so, my fellow Americans, ask not what your country can do for you, ask what you can do for your country.",
	}
	var out bytes.Buffer
	opts := provider.SynthesisOptions{Text: "Buenos días. ¿Cómo estás hoy?", Language: "es", Attempt: 1}
	started := time.Now()
	if err := s.Synthesize(context.Background(), bytes.NewReader(clip), opts, &out); err != nil {
		t.Fatalf("Synthesize: %v", err)
	}

	seconds := float64(out.Len()-44) / (16_000 * 2)
	t.Logf("%.1f s of speech in %.1f s", seconds, time.Since(started).Seconds())
	if seconds < 0.5 {
		t.Errorf("only %.2f s of audio", seconds)
	}
	if path := os.Getenv("FISH_LIVE_OUT"); path != "" {
		if err := os.WriteFile(path, out.Bytes(), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}
