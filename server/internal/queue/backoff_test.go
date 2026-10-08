package queue

import (
	"testing"
	"time"
)

func TestExponentialBackoff(t *testing.T) {
	backoff := ExponentialBackoff(2*time.Second, 30*time.Second)

	tests := []struct {
		attempt int
		want    time.Duration // before jitter
	}{
		{1, 2 * time.Second},
		{2, 4 * time.Second},
		{3, 8 * time.Second},
		{4, 16 * time.Second},
		{5, 30 * time.Second},  // 32s, capped
		{50, 30 * time.Second}, // far past the cap, and must not overflow
	}
	for _, tt := range tests {
		lowest, highest := time.Duration(float64(tt.want)*0.8), time.Duration(float64(tt.want)*1.2)
		// Jitter is random, so sample it many times.
		for range 200 {
			got := backoff(tt.attempt)
			if got < lowest || got > highest {
				t.Fatalf("attempt %d: backoff = %v, want between %v and %v", tt.attempt, got, lowest, highest)
			}
		}
	}
}

func TestExponentialBackoffIsJittered(t *testing.T) {
	backoff := ExponentialBackoff(2*time.Second, 30*time.Second)

	seen := map[time.Duration]bool{}
	for range 50 {
		seen[backoff(1)] = true
	}
	if len(seen) < 10 {
		t.Errorf("50 calls gave only %d distinct delays; jobs would retry in step", len(seen))
	}
}
