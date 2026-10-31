package fake

import (
	"context"
	"errors"

	"github.com/PelumiWeb/translator-app-go/server/internal/provider"
)

// Translator stands in for a translation service. It cannot translate, so it
// marks the text with the language it was asked for, "(es) hello": plainly
// not a translation, but enough to see a message travel between two phones.
type Translator struct {
	Err error // when set, Translate always fails with it
}

func (t Translator) Translate(_ context.Context, text, _, to string) (string, error) {
	if t.Err != nil {
		return "", t.Err
	}
	if text == "" {
		return "", provider.Permanent(errors.New("there is no text to translate"))
	}
	return "(" + to + ") " + text, nil
}
