// Package google translates text with Google Cloud Translation (the "basic"
// edition, its v2 REST API), chosen by the owner on 2026-10-31 (ADR 0020).
package google

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/PelumiWeb/translator-app-go/server/internal/provider"
)

const defaultBaseURL = "https://translation.googleapis.com"

// googleCodes maps the few languages the phones name differently from
// Google. Nynorsk is the one Whisper knows that Google does not; written
// Norwegian is the nearest.
var googleCodes = map[string]string{
	"nn": "no",
}

// Translator calls Google Cloud Translation with an API key from the
// project's credentials page, restricted to that API.
type Translator struct {
	APIKey string

	// Client and BaseURL default to an HTTP client with a timeout and
	// Google's address; tests point them at a local server.
	Client  *http.Client
	BaseURL string
}

func (t Translator) Translate(ctx context.Context, text, from, to string) (string, error) {
	if text == "" {
		return "", provider.Permanent(errors.New("there is no text to translate"))
	}
	body, err := json.Marshal(map[string]any{
		"q":      []string{text},
		"source": code(from),
		"target": code(to),
		// "html", the default, would turn an apostrophe into &#39;.
		"format": "text",
	})
	if err != nil {
		return "", provider.Permanent(fmt.Errorf("building the request: %w", err))
	}

	baseURL := t.BaseURL
	if baseURL == "" {
		baseURL = defaultBaseURL
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, baseURL+"/language/translate/v2", bytes.NewReader(body))
	if err != nil {
		return "", provider.Permanent(fmt.Errorf("building the request: %w", err))
	}
	req.Header.Set("Content-Type", "application/json")
	// In a header rather than the URL's ?key=, so the key never ends up in a
	// log line that prints a request's URL.
	req.Header.Set("X-Goog-Api-Key", t.APIKey)

	client := t.Client
	if client == nil {
		// A sentence takes well under a second; a stuck connection should
		// not hold a worker for long before the job is retried.
		client = &http.Client{Timeout: 15 * time.Second}
	}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("calling Google Cloud Translation: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		detail, _ := io.ReadAll(io.LimitReader(resp.Body, 2<<10))
		err := fmt.Errorf("Google Cloud Translation answered %d: %s", resp.StatusCode, bytes.TrimSpace(detail))
		// Rate limits and outages pass; a bad key, a disabled API or a
		// language Google does not have will be just as bad next time.
		if resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode >= 500 {
			return "", err
		}
		return "", provider.Permanent(err)
	}

	var result struct {
		Data struct {
			Translations []struct {
				TranslatedText string `json:"translatedText"`
			} `json:"translations"`
		} `json:"data"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&result); err != nil {
		return "", fmt.Errorf("reading Google's answer: %w", err)
	}
	if len(result.Data.Translations) != 1 {
		return "", fmt.Errorf("Google answered with %d translations for one text", len(result.Data.Translations))
	}
	return result.Data.Translations[0].TranslatedText, nil
}

func code(lang string) string {
	if c, ok := googleCodes[lang]; ok {
		return c
	}
	return lang
}
