// Package fake is a Provider that needs no network and no API key. It is
// used for local development and in tests.
package fake

import (
	"context"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/PelumiWeb/translator-app-go/server/internal/provider"
)

// Provider "transcribes" any audio to the same fixed sentence, one word at a
// time, so the partial events and the progress UI have something to show.
type Provider struct {
	Text      string
	WordDelay time.Duration
	Err       error // when set, Transcribe fails with it
}

func (p Provider) Transcribe(ctx context.Context, audio io.Reader, opts provider.Options, onPartial func(string)) (provider.Result, error) {
	// Read the audio to the end like a real provider would upload it.
	if _, err := io.Copy(io.Discard, audio); err != nil {
		return provider.Result{}, fmt.Errorf("reading audio: %w", err)
	}
	if p.Err != nil {
		return provider.Result{}, p.Err
	}

	words := strings.Fields(p.Text)
	for i := range words {
		// Sleep, but give up at once if the job is cancelled. A plain
		// time.Sleep would ignore ctx and hold up shutdown.
		select {
		case <-ctx.Done():
			return provider.Result{}, ctx.Err()
		case <-time.After(p.WordDelay):
		}
		onPartial(strings.Join(words[:i+1], " "))
	}

	return provider.Result{Text: strings.Join(words, " "), Language: opts.SourceLang}, nil
}
