// Package groq transcribes speech with OpenAI's Whisper large-v3, run by
// Groq behind an OpenAI-compatible API (ADR 0026). It is the server's
// transcriber for phones that cannot transcribe on their own.
package groq

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/PelumiWeb/translator-app-go/server/internal/provider"
)

const (
	defaultBaseURL = "https://api.groq.com/openai/v1"
	// The full model rather than the faster "turbo" one: the turbo model
	// has fewer decoder layers, which costs most in the languages with the
	// least training data, Yoruba and Hausa among them.
	defaultModel = "whisper-large-v3"
)

// The reference Whisper's rules for audio that is not speech, with its
// thresholds. A segment is silence when the model thinks so and is also
// unsure of the words it gave; text that compresses this well repeats itself
// too much to be speech. The phones apply the same thresholds on device.
const (
	noSpeechThreshold    = 0.6
	logProbThreshold     = -1.0
	compressionThreshold = 2.4
)

// Transcriber calls Groq with a key from console.groq.com.
type Transcriber struct {
	APIKey string
	Model  string // empty for whisper-large-v3

	// Client and BaseURL default to an HTTP client with a timeout and
	// Groq's address; tests point them at a local server.
	Client  *http.Client
	BaseURL string
}

// Transcribe sends the whole recording and waits for the text: Groq does not
// stream, so onPartial is never called.
func (t Transcriber) Transcribe(ctx context.Context, audio io.Reader, opts provider.Options, _ func(string)) (provider.Result, error) {
	body, contentType, err := t.form(audio, opts.SourceLang)
	if err != nil {
		return provider.Result{}, err
	}

	baseURL := t.BaseURL
	if baseURL == "" {
		baseURL = defaultBaseURL
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, baseURL+"/audio/transcriptions", body)
	if err != nil {
		return provider.Result{}, provider.Permanent(fmt.Errorf("building the request: %w", err))
	}
	req.Header.Set("Authorization", "Bearer "+t.APIKey)
	req.Header.Set("Content-Type", contentType)

	client := t.Client
	if client == nil {
		// A voice note takes a second or two; the timeout only stops a stuck
		// connection from holding a worker until its lease runs out.
		client = &http.Client{Timeout: 60 * time.Second}
	}
	resp, err := client.Do(req)
	if err != nil {
		return provider.Result{}, fmt.Errorf("calling Groq: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		detail, _ := io.ReadAll(io.LimitReader(resp.Body, 2<<10))
		err := fmt.Errorf("Groq answered %d: %s", resp.StatusCode, bytes.TrimSpace(detail))
		// Rate limits and outages pass. Audio Groq cannot read, a language
		// Whisper does not have, or a bad key will be as bad next time.
		if resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode >= 500 {
			return provider.Result{}, err
		}
		return provider.Result{}, provider.Permanent(err)
	}

	var answer transcription
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&answer); err != nil {
		return provider.Result{}, fmt.Errorf("reading Groq's answer: %w", err)
	}
	text, err := answer.speech()
	if err != nil {
		return provider.Result{}, err
	}
	return provider.Result{Text: text, Language: opts.SourceLang}, nil
}

// form builds the multipart body. It is held in memory rather than streamed:
// a voice note is a few hundred kilobytes, and a body with a known length is
// what every HTTP server accepts.
func (t Transcriber) form(audio io.Reader, lang string) (io.Reader, string, error) {
	var body bytes.Buffer
	form := multipart.NewWriter(&body)
	// Groq tells the audio's format from the file name's extension.
	file, err := form.CreateFormFile("file", "audio.wav")
	if err != nil {
		return nil, "", provider.Permanent(fmt.Errorf("building the request: %w", err))
	}
	if _, err := io.Copy(file, audio); err != nil {
		return nil, "", fmt.Errorf("reading audio: %w", err)
	}
	fields := map[string]string{
		"model": t.Model,
		// verbose_json carries each segment's no-speech probability, which
		// json leaves out.
		"response_format": "verbose_json",
		"temperature":     "0",
	}
	if fields["model"] == "" {
		fields["model"] = defaultModel
	}
	// Whisper takes the bare language ("pt"), not a regional tag ("pt-BR").
	if code, _, _ := strings.Cut(lang, "-"); code != "" {
		fields["language"] = strings.ToLower(code)
	}
	for name, value := range fields {
		if err := form.WriteField(name, value); err != nil {
			return nil, "", provider.Permanent(fmt.Errorf("building the request: %w", err))
		}
	}
	if err := form.Close(); err != nil {
		return nil, "", provider.Permanent(fmt.Errorf("building the request: %w", err))
	}
	return &body, form.FormDataContentType(), nil
}

// transcription is the part of Groq's verbose_json answer that is used.
type transcription struct {
	Text     string `json:"text"`
	Segments []struct {
		Text             string  `json:"text"`
		AvgLogProb       float64 `json:"avg_logprob"`
		NoSpeechProb     float64 `json:"no_speech_prob"`
		CompressionRatio float64 `json:"compression_ratio"`
	} `json:"segments"`
}

// soundLabels matches text that is only labels such as "[Music]" or
// "(laughs)": Whisper's way of saying it heard no words.
var soundLabels = regexp.MustCompile(`^(\s*(\[[^\]]*\]|\([^)]*\)))+\s*$`)

// speech is the text the speaker said: the segments that are not silence,
// joined. Given silence or noise, Whisper invents a plausible sentence, so a
// recording with no speech in it is an error rather than that sentence.
func (t transcription) speech() (string, error) {
	text := t.Text
	if len(t.Segments) > 0 {
		var kept strings.Builder
		for _, s := range t.Segments {
			if s.NoSpeechProb > noSpeechThreshold && s.AvgLogProb < logProbThreshold {
				continue
			}
			if s.CompressionRatio > compressionThreshold {
				return "", provider.Permanent(errors.New("the speech could not be made out; please try again"))
			}
			// Each segment's text starts with its own space, where the
			// language uses spaces, so they join as they are.
			kept.WriteString(s.Text)
		}
		text = kept.String()
	}
	text = strings.TrimSpace(text)
	if text == "" || soundLabels.MatchString(text) {
		return "", provider.Permanent(errors.New("no speech was recognised"))
	}
	return text, nil
}
