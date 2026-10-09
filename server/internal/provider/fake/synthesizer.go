package fake

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"math"
	"strings"
	"time"

	"github.com/PelumiWeb/translator-app-go/server/internal/provider"
)

const (
	sampleRate = 16_000

	// A real engine cannot imitate a voice from a fraction of a second of
	// it, so the fake refuses too: one second of 16 kHz 16-bit mono.
	minVoiceBytes = sampleRate * 2

	beepMs = 140 // one beep per word
	gapMs  = 90
)

// Synthesizer stands in for a voice-cloning engine. It cannot speak, so it
// answers with one short beep per word of the text: audibly not a voice, but
// real audio whose length follows what was asked for, which is enough to
// build and test everything around the engine.
type Synthesizer struct {
	Delay time.Duration // how long "synthesis" takes
	Err   error         // when set, Synthesize always fails with it

	// FailAttempts makes the first N attempts of every job fail with a
	// temporary error, to exercise retries.
	FailAttempts int
}

func (s Synthesizer) Synthesize(ctx context.Context, voice io.Reader, opts provider.SynthesisOptions, out io.Writer) error {
	// Read the sample to the end like a real engine would upload it.
	voiceBytes, err := io.Copy(io.Discard, voice)
	if err != nil {
		return fmt.Errorf("reading voice sample: %w", err)
	}
	if s.Err != nil {
		return s.Err
	}
	if opts.Attempt <= s.FailAttempts {
		return fmt.Errorf("fake synthesizer: simulated outage on attempt %d", opts.Attempt)
	}
	// Problems with the request itself: trying again would not help.
	if voiceBytes < minVoiceBytes {
		return provider.Permanent(errors.New("the voice sample is too short to imitate"))
	}
	words := len(strings.Fields(opts.Text))
	if words == 0 {
		return provider.Permanent(errors.New("there is no text to speak"))
	}

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(s.Delay):
	}
	return writeBeeps(out, words)
}

// writeBeeps writes a WAV file with one beep per word.
func writeBeeps(out io.Writer, words int) error {
	beep, gap := sampleRate*beepMs/1000, sampleRate*gapMs/1000
	samples := make([]int16, 0, words*(beep+gap))
	for w := range words {
		// Alternate two pitches so a run of beeps does not sound like a fault.
		pitch := 440.0
		if w%2 == 1 {
			pitch = 554.0
		}
		for i := range beep {
			samples = append(samples, int16(6000*math.Sin(2*math.Pi*pitch*float64(i)/sampleRate)))
		}
		samples = append(samples, make([]int16, gap)...)
	}
	return writeWAV(out, samples)
}

// writeWAV writes 16 kHz, 16-bit mono samples as a WAV file: a 44-byte
// header and then the samples. Every number in the format is little-endian.
func writeWAV(out io.Writer, samples []int16) error {
	dataBytes := uint32(len(samples) * 2)
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
	// binary.Write encodes the whole slice in one call.
	return binary.Write(out, binary.LittleEndian, samples)
}
