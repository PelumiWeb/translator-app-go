package azure

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/PelumiWeb/translator-app-go/server/internal/provider"
)

func TestTranslateSendsTheTextAndReadsTheTranslation(t *testing.T) {
	azure := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/translate" {
			t.Errorf("request = %s %s, want POST /translate", r.Method, r.URL.Path)
		}
		q := r.URL.Query()
		if q.Get("api-version") != "3.0" || q.Get("from") != "en" || q.Get("to") != "zh-Hans" {
			t.Errorf("query = %q", r.URL.RawQuery)
		}
		if got := r.Header.Get("Ocp-Apim-Subscription-Key"); got != "test-key" {
			t.Errorf("key header = %q", got)
		}
		if got := r.Header.Get("Ocp-Apim-Subscription-Region"); got != "southafricanorth" {
			t.Errorf("region header = %q", got)
		}
		var body []struct{ Text string }
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		if len(body) != 1 || body[0].Text != "I'm on my way" {
			t.Errorf("body = %+v", body)
		}
		w.Write([]byte(`[{"translations":[{"text":"我在路上","to":"zh-Hans"}]}]`))
	}))
	defer azure.Close()

	tr := Translator{Key: "test-key", Region: "southafricanorth", BaseURL: azure.URL}
	// Chinese goes to Azure with its script.
	got, err := tr.Translate(context.Background(), "I'm on my way", "en", "zh")
	if err != nil || got != "我在路上" {
		t.Errorf("Translate = %q, %v", got, err)
	}
}

func TestAGlobalResourceSendsNoRegion(t *testing.T) {
	azure := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if _, sent := r.Header["Ocp-Apim-Subscription-Region"]; sent {
			t.Errorf("region header sent for a global resource: %q", r.Header.Get("Ocp-Apim-Subscription-Region"))
		}
		w.Write([]byte(`[{"translations":[{"text":"hola","to":"es"}]}]`))
	}))
	defer azure.Close()

	got, err := Translator{Key: "k", Region: "global", BaseURL: azure.URL}.Translate(context.Background(), "hello", "en", "es")
	if err != nil || got != "hola" {
		t.Errorf("Translate = %q, %v", got, err)
	}
}

func TestTranslateTellsWhichFailuresAreWorthRetrying(t *testing.T) {
	tests := []struct {
		status        int
		wantPermanent bool
	}{
		{http.StatusBadRequest, true},   // e.g. a language Azure does not have
		{http.StatusUnauthorized, true}, // a bad key
		{http.StatusForbidden, true},    // the free tier's month used up
		{http.StatusTooManyRequests, false},
		{http.StatusServiceUnavailable, false},
	}
	for _, tt := range tests {
		t.Run(http.StatusText(tt.status), func(t *testing.T) {
			azure := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				http.Error(w, `{"error":{"code":400036,"message":"nope"}}`, tt.status)
			}))
			defer azure.Close()

			_, err := Translator{Key: "k", BaseURL: azure.URL}.Translate(context.Background(), "hello", "en", "es")
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
	_, err := Translator{Key: "k", BaseURL: "http://unused"}.Translate(context.Background(), "", "en", "es")
	if !provider.IsPermanent(err) {
		t.Errorf("err = %v, want a permanent error", err)
	}
}

// TestLiveTranslation calls the real Azure AI Translator with a few short
// sentences, from the free tier's monthly allowance, so it runs only when
// AZURE_TRANSLATOR_KEY is set (`make translate-check` does that from
// server/.env). -v prints the translations.
func TestLiveTranslation(t *testing.T) {
	key := os.Getenv("AZURE_TRANSLATOR_KEY")
	if key == "" {
		t.Skip("AZURE_TRANSLATOR_KEY is not set")
	}
	tr := Translator{Key: key, Region: os.Getenv("AZURE_TRANSLATOR_REGION")}
	pairs := []struct{ text, from, to string }{
		{"Good morning, mum. Did you sleep well?", "en", "yo"},
		{"Good morning, mum. Did you sleep well?", "en", "ha"},
		{"Good morning, mum. Did you sleep well?", "en", "es"},
		{"Good morning, mum. Did you sleep well?", "en", "zh"},
		{"Ẹ káàárọ̀, ṣé ẹ sùn dáadáa?", "yo", "en"},
	}
	for _, p := range pairs {
		got, err := tr.Translate(context.Background(), p.text, p.from, p.to)
		if err != nil {
			t.Errorf("%s to %s: %v", p.from, p.to, err)
			continue
		}
		t.Logf("%s to %s: %s", p.from, p.to, got)
	}
}
