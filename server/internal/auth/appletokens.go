package auth

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	"github.com/PelumiWeb/translator-app-go/server/internal/es256"
)

// ErrAppleRefused means Sign in with Apple's REST API refused a request, with
// the reason it gave: "invalid_grant" for a code that is wrong, used or older
// than five minutes; "invalid_client" for a key, team or app it does not know.
type ErrAppleRefused struct{ Reason string }

func (e ErrAppleRefused) Error() string { return "Sign in with Apple refused: " + e.Reason }

// AppleTokens talks to Sign in with Apple's REST API with the Sign in with
// Apple key. It trades the authorization code a phone gets at sign-in for a
// refresh token, and revokes that token when the account is deleted: Apple
// asks apps that offer Sign in with Apple to do so (Guideline 5.1.1(v),
// ADR 0018).
type AppleTokens struct {
	TeamID   string
	KeyID    string
	ClientID string // the app's bundle ID, which the phone signs in as
	Key      *ecdsa.PrivateKey

	Client  *http.Client
	BaseURL string // tests point it at a local server
}

// Exchange trades an authorization code for the refresh token Apple keeps for
// this person and app.
func (a AppleTokens) Exchange(ctx context.Context, code string) (refreshToken string, err error) {
	var answer struct {
		RefreshToken string `json:"refresh_token"`
	}
	err = a.post(ctx, "/auth/token", url.Values{"code": {code}, "grant_type": {"authorization_code"}}, &answer)
	if err != nil {
		return "", err
	}
	if answer.RefreshToken == "" {
		return "", errors.New("Sign in with Apple gave no refresh token")
	}
	return answer.RefreshToken, nil
}

// Revoke ends a refresh token, and with it the link between the person's
// Apple account and the app: they see Shinar leave the list of apps using
// Sign in with Apple. Revoking one already revoked is not an error.
func (a AppleTokens) Revoke(ctx context.Context, refreshToken string) error {
	return a.post(ctx, "/auth/revoke", url.Values{"token": {refreshToken}, "token_type_hint": {"refresh_token"}}, nil)
}

func (a AppleTokens) post(ctx context.Context, path string, form url.Values, into any) error {
	secret, err := a.clientSecret(time.Now())
	if err != nil {
		return err
	}
	form.Set("client_id", a.ClientID)
	form.Set("client_secret", secret)
	base := a.BaseURL
	if base == "" {
		base = "https://appleid.apple.com"
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, base+path, bytes.NewBufferString(form.Encode()))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	client := a.Client
	if client == nil {
		client = &http.Client{Timeout: 10 * time.Second}
	}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("calling Sign in with Apple: %w", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 64<<10))
	if resp.StatusCode != http.StatusOK {
		var refused struct {
			Error string `json:"error"`
		}
		if json.Unmarshal(body, &refused) == nil && refused.Error != "" {
			return ErrAppleRefused{Reason: refused.Error}
		}
		return fmt.Errorf("Sign in with Apple answered %d: %s", resp.StatusCode, bytes.TrimSpace(body))
	}
	if into == nil {
		return nil
	}
	if err := json.Unmarshal(body, into); err != nil {
		return fmt.Errorf("reading Sign in with Apple's answer: %w", err)
	}
	return nil
}

// clientSecret is the token that stands for the app in each request, signed
// with the key. Apple allows up to six months; this one lives five minutes.
func (a AppleTokens) clientSecret(now time.Time) (string, error) {
	return es256.Sign(a.Key,
		map[string]string{"alg": "ES256", "kid": a.KeyID},
		map[string]any{
			"iss": a.TeamID, "iat": now.Unix(), "exp": now.Add(5 * time.Minute).Unix(),
			"aud": "https://appleid.apple.com", "sub": a.ClientID,
		})
}
