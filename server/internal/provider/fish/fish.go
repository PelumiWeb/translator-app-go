// Package fish speaks text in a user's voice with Fish Audio's text-to-speech
// API (docs.fish.audio). ADR 0010 has the reasons for choosing it.
package fish

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/PelumiWeb/translator-app-go/server/internal/provider"
)

const (
	defaultBaseURL = "https://api.fish.audio"

	// The rate the phones play and the rest of the server works in.
	sampleRate = 16_000
)

// Synthesizer imitates the voice heard in a sample, which goes with every
// request together with its transcript. Fish Audio therefore keeps no voices:
// nothing is created there per user, and changing engine later never means
// asking anyone to record again.
type Synthesizer struct {
	APIKey string

	// Model is the Fish Audio model, sent as the "model" header; empty for
	// theirs by default.
	Model string

	// VoiceText is the transcript of the voice samples: the passage the
	// phones ask users to read.
	VoiceText string

	// Client and BaseURL default to an HTTP client with a timeout and Fish
	// Audio's address. Tests replace them.
	Client  *http.Client
	BaseURL string
}

func (s Synthesizer) Synthesize(ctx context.Context, voice io.Reader, opts provider.SynthesisOptions, out io.Writer) error {
	text := strings.TrimSpace(opts.Text)
	if text == "" {
		return provider.Permanent(errors.New("there is no text to speak"))
	}
	sample, err := io.ReadAll(voice)
	if err != nil {
		return fmt.Errorf("reading voice sample: %w", err)
	}

	baseURL := s.BaseURL
	if baseURL == "" {
		baseURL = defaultBaseURL
	}
	body := encodeRequest(text, opts.Language, sample, s.VoiceText)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, baseURL+"/v1/tts", bytes.NewReader(body))
	if err != nil {
		return provider.Permanent(fmt.Errorf("building the request: %w", err))
	}
	req.Header.Set("Authorization", "Bearer "+s.APIKey)
	req.Header.Set("Content-Type", "application/msgpack")
	if s.Model != "" {
		req.Header.Set("model", s.Model)
	}

	client := s.Client
	if client == nil {
		// A sentence takes a few seconds; this only stops a hung connection
		// from holding a worker forever.
		client = &http.Client{Timeout: time.Minute}
	}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("calling Fish Audio: %w", err) // a network failure: worth retrying
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		detail, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		err := fmt.Errorf("Fish Audio answered %d: %s", resp.StatusCode, bytes.TrimSpace(detail))
		// Too many requests, or trouble on their side, can pass. Anything
		// else is about the request or the account (a bad key, no credit),
		// and asking again gets the same answer.
		if resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode >= 500 {
			return err
		}
		return provider.Permanent(err)
	}

	pcm, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("reading Fish Audio's audio: %w", err)
	}
	if len(pcm) < 2 {
		return errors.New("Fish Audio sent no audio")
	}
	return writeWAV(out, pcm[:len(pcm)&^1]) // whole 16-bit samples only
}

// encodeRequest builds the body of a request to speak. The audio is asked
// for as raw 16 kHz PCM and given a WAV header here: a WAV file streamed by
// the engine cannot say its own length up front, which some readers reject.
func encodeRequest(text, language string, sample []byte, sampleText string) []byte {
	var e encoder
	entries := 4
	if language != "" {
		entries++
	}
	e.mapHeader(entries)
	e.str("text")
	e.str(text)
	// The voice to imitate, with what is said in it.
	e.str("references")
	e.arrayHeader(1)
	e.mapHeader(2)
	e.str("audio")
	e.bytes(sample)
	e.str("text")
	e.str(sampleText)
	e.str("format")
	e.str("pcm")
	e.str("sample_rate")
	e.uint(sampleRate)
	if language != "" {
		e.str("language")
		e.str(language)
	}
	return e.buf
}

// writeWAV writes 16 kHz, 16-bit mono PCM as a WAV file: a 44-byte header,
// then the samples. Every number in the header is little-endian.
func writeWAV(out io.Writer, pcm []byte) error {
	dataBytes := uint32(len(pcm))
	header := make([]byte, 0, 44)
	header = append(header, "RIFF"...)
	header = binary.LittleEndian.AppendUint32(header, 36+dataBytes) // everything after this field
	header = append(header, "WAVEfmt "...)
	header = binary.LittleEndian.AppendUint32(header, 16)           // size of the fmt chunk
	header = binary.LittleEndian.AppendUint16(header, 1)            // format: PCM
	header = binary.LittleEndian.AppendUint16(header, 1)            // channels
	header = binary.LittleEndian.AppendUint32(header, sampleRate)   // samples per second
	header = binary.LittleEndian.AppendUint32(header, sampleRate*2) // bytes per second
	header = binary.LittleEndian.AppendUint16(header, 2)            // bytes per frame
	header = binary.LittleEndian.AppendUint16(header, 16)           // bits per sample
	header = append(header, "data"...)
	header = binary.LittleEndian.AppendUint32(header, dataBytes)

	if _, err := out.Write(header); err != nil {
		return err
	}
	_, err := out.Write(pcm)
	return err
}
