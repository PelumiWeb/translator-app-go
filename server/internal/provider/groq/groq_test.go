package groq

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/PelumiWeb/translator-app-go/server/internal/provider"
)

func TestTranscribeSendsTheAudioAndReadsTheText(t *testing.T) {
	groq := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/audio/transcriptions" {
			t.Errorf("request = %s %s, want POST /audio/transcriptions", r.Method, r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer test-key" {
			t.Errorf("Authorization = %q", got)
		}
		file, header, err := r.FormFile("file")
		if err != nil {
			t.Fatal(err)
		}
		audio, _ := io.ReadAll(file)
		if string(audio) != "RIFF...WAVE" || header.Filename != "audio.wav" {
			t.Errorf("file = %q named %q", audio, header.Filename)
		}
		want := map[string]string{
			"model":           "whisper-large-v3",
			"language":        "pt", // from "pt-BR"
			"response_format": "verbose_json",
			"temperature":     "0",
		}
		for name, value := range want {
			if got := r.FormValue(name); got != value {
				t.Errorf("%s = %q, want %q", name, got, value)
			}
		}
		w.Write([]byte(`{"task":"transcribe","language":"portuguese","text":" Bom dia. Tudo bem?","segments":[
			{"id":0,"text":" Bom dia.","avg_logprob":-0.2,"no_speech_prob":0.01,"compression_ratio":0.9},
			{"id":1,"text":" Tudo bem?","avg_logprob":-0.3,"no_speech_prob":0.02,"compression_ratio":0.8}]}`))
	}))
	defer groq.Close()

	tr := Transcriber{APIKey: "test-key", BaseURL: groq.URL}
	got, err := tr.Transcribe(context.Background(), strings.NewReader("RIFF...WAVE"),
		provider.Options{SourceLang: "pt-BR", Attempt: 1}, func(string) { t.Error("onPartial called") })
	if err != nil {
		t.Fatal(err)
	}
	if got.Text != "Bom dia. Tudo bem?" || got.Language != "pt-BR" {
		t.Errorf("Transcribe = %+v", got)
	}
}

func TestTranscribeUsesTheModelAskedFor(t *testing.T) {
	groq := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.FormValue("model"); got != "whisper-large-v3-turbo" {
			t.Errorf("model = %q", got)
		}
		w.Write([]byte(`{"text":"hello"}`))
	}))
	defer groq.Close()

	tr := Transcriber{APIKey: "k", Model: "whisper-large-v3-turbo", BaseURL: groq.URL}
	if _, err := tr.Transcribe(context.Background(), strings.NewReader("wav"), provider.Options{SourceLang: "en"}, nil); err != nil {
		t.Fatal(err)
	}
}

func TestTranscribeTellsWhichFailuresAreWorthRetrying(t *testing.T) {
	tests := []struct {
		status        int
		wantPermanent bool
	}{
		{http.StatusBadRequest, true},            // audio it cannot read, or a language Whisper does not have
		{http.StatusUnauthorized, true},          // a bad key
		{http.StatusRequestEntityTooLarge, true}, // over the 25 MB a file may be
		{http.StatusTooManyRequests, false},      // the free plan's rate limit
		{http.StatusInternalServerError, false},
		{http.StatusServiceUnavailable, false},
	}
	for _, tt := range tests {
		t.Run(http.StatusText(tt.status), func(t *testing.T) {
			groq := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				http.Error(w, `{"error":{"message":"nope"}}`, tt.status)
			}))
			defer groq.Close()

			_, err := Transcriber{APIKey: "k", BaseURL: groq.URL}.Transcribe(context.Background(), strings.NewReader("wav"), provider.Options{SourceLang: "en"}, nil)
			if err == nil {
				t.Fatal("no error")
			}
			if provider.IsPermanent(err) != tt.wantPermanent {
				t.Errorf("IsPermanent = %v, want %v (error %v)", !tt.wantPermanent, tt.wantPermanent, err)
			}
		})
	}
}

func TestSpeechLeavesOutWhatIsNotSpeech(t *testing.T) {
	type segment = struct {
		Text             string  `json:"text"`
		AvgLogProb       float64 `json:"avg_logprob"`
		NoSpeechProb     float64 `json:"no_speech_prob"`
		CompressionRatio float64 `json:"compression_ratio"`
	}
	tests := []struct {
		name    string
		answer  transcription
		want    string
		wantErr string
	}{
		{
			name:   "no segments: the whole text",
			answer: transcription{Text: " Hello there. "},
			want:   "Hello there.",
		},
		{
			name: "a silent segment is left out",
			answer: transcription{Text: " Hello. Thank you for watching.", Segments: []segment{
				{Text: " Hello.", AvgLogProb: -0.2, NoSpeechProb: 0.05, CompressionRatio: 1},
				{Text: " Thank you for watching.", AvgLogProb: -1.4, NoSpeechProb: 0.9, CompressionRatio: 1},
			}},
			want: "Hello.",
		},
		{
			name: "likely silence but confident words are kept",
			answer: transcription{Segments: []segment{
				{Text: " Ẹ káàárọ̀.", AvgLogProb: -0.4, NoSpeechProb: 0.7, CompressionRatio: 1},
			}},
			want: "Ẹ káàárọ̀.",
		},
		{
			name: "only silence",
			answer: transcription{Text: " Thank you.", Segments: []segment{
				{Text: " Thank you.", AvgLogProb: -1.2, NoSpeechProb: 0.95, CompressionRatio: 1},
			}},
			wantErr: "no speech was recognised",
		},
		{
			name:    "only sound labels",
			answer:  transcription{Text: " [Music] (applause)"},
			wantErr: "no speech was recognised",
		},
		{
			name:    "nothing at all",
			answer:  transcription{Text: "  "},
			wantErr: "no speech was recognised",
		},
		{
			name: "text that repeats itself",
			answer: transcription{Segments: []segment{
				{Text: " no no no no no no no no no no no no no no", AvgLogProb: -0.5, NoSpeechProb: 0.1, CompressionRatio: 3.1},
			}},
			wantErr: "the speech could not be made out; please try again",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := tt.answer.speech()
			if tt.wantErr != "" {
				if err == nil || err.Error() != tt.wantErr || !provider.IsPermanent(err) {
					t.Fatalf("speech() = %q, %v; want the permanent error %q", got, err, tt.wantErr)
				}
				return
			}
			if err != nil || got != tt.want {
				t.Errorf("speech() = %q, %v; want %q", got, err, tt.want)
			}
		})
	}
}

// TestLiveTranscription sends whisper.cpp's sample clip (11 seconds of
// President Kennedy) to the real Groq, so it runs only when GROQ_API_KEY is
// set (`make transcribe-check` does that from server/.env). -v prints the
// text.
func TestLiveTranscription(t *testing.T) {
	key := os.Getenv("GROQ_API_KEY")
	if key == "" {
		t.Skip("GROQ_API_KEY is not set")
	}
	clip, err := os.Open("../../../../third_party/whisper.cpp/samples/jfk.wav")
	if err != nil {
		t.Fatalf("%v (run `git submodule update --init`)", err)
	}
	defer clip.Close()

	tr := Transcriber{APIKey: key, Model: os.Getenv("GROQ_MODEL")}
	got, err := tr.Transcribe(context.Background(), clip, provider.Options{SourceLang: "en", Attempt: 1}, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("%s", got.Text)
	if !strings.Contains(strings.ToLower(got.Text), "ask not what your country can do for you") {
		t.Errorf("the transcript does not have the clip's words: %q", got.Text)
	}
}
