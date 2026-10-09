// Package provider defines what the server needs from the outside engines it
// uses: a speech-to-text service and a voice-cloning speech synthesizer.
// Implementations live in sub-packages (fake, and real ones later).
package provider

import (
	"context"
	"errors"
	"io"
)

type Options struct {
	SourceLang string // BCP 47 code such as "en" or "yo"
	Attempt    int    // 1 on the first try of this job, 2 on the first retry, ...
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

type SynthesisOptions struct {
	Text     string // what to say
	Language string // the language Text is in, as a BCP 47 code
	Attempt  int    // 1 on the first try of this job
}

// Synthesizer speaks a text in the voice heard in a sample recording.
//
// The audio is written to out as a WAV file. A writer is used instead of a
// returned buffer so an implementation can stream what the engine sends
// straight into storage without holding it all in memory.
type Synthesizer interface {
	Synthesize(ctx context.Context, voice io.Reader, opts SynthesisOptions, out io.Writer) error
}

// permanentError marks a failure that trying again cannot fix: audio the
// provider cannot decode, an unsupported language. Anything not marked is
// treated as temporary (a timeout, a rate limit, a provider outage) and
// retried, because that is the safer guess for an unknown error.
type permanentError struct{ err error }

func (e permanentError) Error() string { return e.err.Error() }

// Unwrap lets errors.Is and errors.As see through to the original error.
func (e permanentError) Unwrap() error { return e.err }

// Permanent wraps err so that the queue does not retry the job. The message
// is unchanged.
func Permanent(err error) error {
	if err == nil {
		return nil
	}
	return permanentError{err}
}

// IsPermanent reports whether err, or any error it wraps, was marked with
// Permanent.
func IsPermanent(err error) bool {
	var p permanentError
	return errors.As(err, &p)
}
