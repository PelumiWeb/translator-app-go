package google

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/PelumiWeb/translator-app-go/server/internal/provider"
)

func TestTranslateSendsTheTextAndReadsTheTranslation(t *testing.T) {
	google := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/language/translate/v2" {
			t.Errorf("request = %s %s, want POST /language/translate/v2", r.Method, r.URL.Path)
		}
		if got := r.Header.Get("X-Goog-Api-Key"); got != "test-key" {
			t.Errorf("API key header = %q", got)
		}
		if r.URL.RawQuery != "" {
			t.Errorf("query = %q: the key must not travel in the URL", r.URL.RawQuery)
		}
		var body struct {
			Q      []string `json:"q"`
			Source string   `json:"source"`
			Target string   `json:"target"`
			Format string   `json:"format"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		if len(body.Q) != 1 || body.Q[0] != "I'm on my way" || body.Source != "en" || body.Target != "no" || body.Format != "text" {
			t.Errorf("body = %+v", body)
		}
		w.Write([]byte(`{"data":{"translations":[{"translatedText":"Jeg er på vei"}]}}`))
	}))
	defer google.Close()

	tr := Translator{APIKey: "test-key", BaseURL: google.URL}
	// Nynorsk goes to Google as written Norwegian.
	got, err := tr.Translate(context.Background(), "I'm on my way", "en", "nn")
	if err != nil || got != "Jeg er på vei" {
		t.Errorf("Translate = %q, %v", got, err)
	}
}

func TestTranslateTellsWhichFailuresAreWorthRetrying(t *testing.T) {
	tests := []struct {
		status        int
		wantPermanent bool
	}{
		{http.StatusBadRequest, true}, // e.g. a language pair Google does not have
		{http.StatusForbidden, true},  // a bad key, or the API not enabled
		{http.StatusTooManyRequests, false},
		{http.StatusServiceUnavailable, false},
	}
	for _, tt := range tests {
		t.Run(http.StatusText(tt.status), func(t *testing.T) {
			google := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				http.Error(w, `{"error":{"message":"nope"}}`, tt.status)
			}))
			defer google.Close()

			_, err := Translator{APIKey: "k", BaseURL: google.URL}.Translate(context.Background(), "hello", "en", "es")
			if err == nil {
				t.Fatal("no error")
			}
			if provider.IsPermanent(err) != tt.wantPermanent {
				t.Errorf("IsPermanent = %v, want %v (error %v)", !tt.wantPermanent, tt.wantPermanent, err)
			}
		})
	}
}

func TestTranslateRefusesEmptyText(t *testing.T) {
	_, err := Translator{APIKey: "k", BaseURL: "http://unused"}.Translate(context.Background(), "", "en", "es")
	if !provider.IsPermanent(err) {
		t.Errorf("err = %v, want a permanent error", err)
	}
}
