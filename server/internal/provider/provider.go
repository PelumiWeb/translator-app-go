// Package provider defines what the server needs from a speech-to-text
// service. Implementations live in sub-packages (fake, and a real one later).
package provider

import (
	"context"
	"io"
)

type Options struct {
	SourceLang string // BCP 47 code such as "en" or "yo"
}

type Result struct {
	Text     string
	Language string
}

// Provider turns audio into text.
//
// Partial results are delivered through a callback instead of a returned
// channel: the caller has nothing to drain or close, and a provider that
// cannot stream simply never calls it. onPartial receives the whole text so
// far, not just the new words, and is called from the same goroutine as
// Transcribe.
type Provider interface {
	Transcribe(ctx context.Context, audio io.Reader, opts Options, onPartial func(text string)) (Result, error)
}
