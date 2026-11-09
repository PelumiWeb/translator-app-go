// Package azure translates text with Microsoft's Azure AI Translator (its v3
// REST API), used while the Google Cloud project cannot be billed (ADR 0020,
// addendum of 2026-11-09).
package azure

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	"github.com/PelumiWeb/translator-app-go/server/internal/provider"
)

const defaultBaseURL = "https://api.cognitive.microsofttranslator.com"

// azureCodes maps the languages the phones name differently from Azure,
// which wants a script for Chinese, Serbian and Mongolian, and calls
// Norwegian "nb" and Tagalog "fil". Nine of the phones' languages Azure does
// not have at all (Latin, Breton, Occitan, Tajik, Yiddish, Sanskrit,
// Hawaiian, Javanese, Sundanese); it refuses those.
var azureCodes = map[string]string{
	"zh": "zh-Hans",
	"no": "nb",
	"nn": "nb",
	"sr": "sr-Cyrl",
	"mn": "mn-Cyrl",
	"tl": "fil",
}

// Translator calls Azure AI Translator with a key from the resource's Keys
// and Endpoint page.
type Translator struct {
	Key string
	// Region is the resource's location from the same page. A "global"
	// resource needs none sent.
	Region string

	// Client and BaseURL default to an HTTP client with a timeout and
	// Azure's address; tests point them at a local server.
	Client  *http.Client
	BaseURL string
}

func (t Translator) Translate(ctx context.Context, text, from, to string) (string, error) {
	if text == "" {
		return "", provider.Permanent(errors.New("there is no text to translate"))
	}
	body, err := json.Marshal([]map[string]string{{"Text": text}})
	if err != nil {
		return "", provider.Permanent(fmt.Errorf("building the request: %w", err))
	}

	baseURL := t.BaseURL
	if baseURL == "" {
		baseURL = defaultBaseURL
	}
	query := url.Values{"api-version": {"3.0"}, "from": {code(from)}, "to": {code(to)}}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, baseURL+"/translate?"+query.Encode(), bytes.NewReader(body))
	if err != nil {
		return "", provider.Permanent(fmt.Errorf("building the request: %w", err))
	}
	req.Header.Set("Content-Type", "application/json; charset=UTF-8")
	req.Header.Set("Ocp-Apim-Subscription-Key", t.Key)
	if t.Region != "" && t.Region != "global" {
		req.Header.Set("Ocp-Apim-Subscription-Region", t.Region)
	}

	client := t.Client
	if client == nil {
		// A sentence takes well under a second; a stuck connection should
		// not hold a worker for long before the job is retried.
		client = &http.Client{Timeout: 15 * time.Second}
	}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("calling Azure AI Translator: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		detail, _ := io.ReadAll(io.LimitReader(resp.Body, 2<<10))
		err := fmt.Errorf("Azure AI Translator answered %d: %s", resp.StatusCode, bytes.TrimSpace(detail))
		// Rate limits and outages pass. A bad key or a language Azure does
		// not have will be as bad next time, and so is the free tier's
		// month running out (403): only the next month fixes that.
		if resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode >= 500 {
			return "", err
		}
		return "", provider.Permanent(err)
	}

	var result []struct {
		Translations []struct {
			Text string `json:"text"`
		} `json:"translations"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&result); err != nil {
		return "", fmt.Errorf("reading Azure's answer: %w", err)
	}
	if len(result) != 1 || len(result[0].Translations) != 1 {
		return "", fmt.Errorf("Azure answered with %d results for one text", len(result))
	}
	return result[0].Translations[0].Text, nil
}

func code(lang string) string {
	if c, ok := azureCodes[lang]; ok {
		return c
	}
	return lang
}
