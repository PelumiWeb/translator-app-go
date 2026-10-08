package provider

import (
	"errors"
	"fmt"
	"testing"
)

func TestPermanent(t *testing.T) {
	cause := errors.New("unsupported audio encoding")

	tests := []struct {
		name string
		err  error
		want bool
	}{
		{"plain error", cause, false},
		{"marked permanent", Permanent(cause), true},
		{"permanent, then wrapped with context", fmt.Errorf("transcribing: %w", Permanent(cause)), true},
		{"nil", nil, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsPermanent(tt.err); got != tt.want {
				t.Errorf("IsPermanent = %v, want %v", got, tt.want)
			}
		})
	}

	if got := Permanent(cause).Error(); got != cause.Error() {
		t.Errorf("message = %q, want it unchanged: %q", got, cause.Error())
	}
	if !errors.Is(Permanent(cause), cause) {
		t.Error("errors.Is cannot see the original error through Permanent")
	}
	if Permanent(nil) != nil {
		t.Error("Permanent(nil) must stay nil, or a success would look like a failure")
	}
}
