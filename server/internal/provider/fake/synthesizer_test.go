package fake

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"strings"
	"testing"

	"github.com/PelumiWeb/translator-app-go/server/internal/provider"
)

func voice(seconds float64) *bytes.Reader {
	return bytes.NewReader(make([]byte, int(seconds*sampleRate*2)))
}

func TestSynthesizerWritesAWAVWhoseLengthFollowsTheText(t *testing.T) {
	var out bytes.Buffer
	opts := provider.SynthesisOptions{Text: "buenos días a todos", Language: "es", Attempt: 1}

	if err := (Synthesizer{}).Synthesize(context.Background(), voice(3), opts, &out); err != nil {
		t.Fatalf("Synthesize: %v", err)
	}

	wav := out.Bytes()
	if len(wav) < 44 || string(wav[0:4]) != "RIFF" || string(wav[8:12]) != "WAVE" || string(wav[36:40]) != "data" {
		t.Fatalf("output is not a WAV file: % x", wav[:min(len(wav), 44)])
	}
	if rate := binary.LittleEndian.Uint32(wav[24:28]); rate != sampleRate {
		t.Errorf("sample rate = %d, want %d", rate, sampleRate)
	}
	dataBytes := binary.LittleEndian.Uint32(wav[40:44])
	if int(dataBytes) != len(wav)-44 {
		t.Errorf("header says %d data bytes, file has %d", dataBytes, len(wav)-44)
	}
	// Four words, each a beep and a gap.
	wantSamples := 4 * (sampleRate*beepMs/1000 + sampleRate*gapMs/1000)
	if got := int(dataBytes) / 2; got != wantSamples {
		t.Errorf("%d samples, want %d for four words", got, wantSamples)
	}
	// And it is sound, not silence.
	loud := false
	for i := 44; i+1 < len(wav); i += 2 {
		if v := int16(binary.LittleEndian.Uint16(wav[i:])); v > 3000 || v < -3000 {
			loud = true
			break
		}
	}
	if !loud {
		t.Error("the audio is silent")
	}
}

func TestSynthesizerRefusesRequestsNoEngineCouldServe(t *testing.T) {
	tests := []struct {
		name  string
		voice float64 // seconds
		text  string
		want  string
	}{
		{"voice sample too short", 0.4, "hola", "the voice sample is too short to imitate"},
		{"nothing to say", 3, "   ", "there is no text to speak"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var out bytes.Buffer
			err := Synthesizer{}.Synthesize(context.Background(), voice(tt.voice), provider.SynthesisOptions{Text: tt.text, Attempt: 1}, &out)

			if err == nil || err.Error() != tt.want {
				t.Fatalf("error = %v, want %q", err, tt.want)
			}
			// Sending the same request again would fail the same way.
			if !provider.IsPermanent(err) {
				t.Error("the error should be permanent, so the queue does not retry it")
			}
			if out.Len() != 0 {
				t.Error("audio was written for a refused request")
			}
		})
	}
}

func TestSynthesizerSimulatedFailures(t *testing.T) {
	opts := func(attempt int) provider.SynthesisOptions {
		return provider.SynthesisOptions{Text: "hola", Attempt: attempt}
	}
	flaky := Synthesizer{FailAttempts: 2}
	for attempt := 1; attempt <= 2; attempt++ {
		err := flaky.Synthesize(context.Background(), voice(3), opts(attempt), &bytes.Buffer{})
		if err == nil || provider.IsPermanent(err) || !strings.Contains(err.Error(), "simulated outage") {
			t.Errorf("attempt %d: error = %v, want a temporary simulated outage", attempt, err)
		}
	}
	if err := flaky.Synthesize(context.Background(), voice(3), opts(3), &bytes.Buffer{}); err != nil {
		t.Errorf("attempt 3: %v, want success", err)
	}

	boom := errors.New("engine exploded")
	if err := (Synthesizer{Err: boom}).Synthesize(context.Background(), voice(3), opts(1), &bytes.Buffer{}); !errors.Is(err, boom) {
		t.Errorf("error = %v, want the configured one", err)
	}
}
