package billing

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/PelumiWeb/translator-app-go/server/internal/es256"
)

// AppStoreAPI calls Apple's App Store Server API, signed with the In-App
// Purchase key from App Store Connect. For now it asks for test
// notifications, which `make appstore-check` uses to check the key and the
// verifier against a record Apple really signed.
type AppStoreAPI struct {
	KeyID    string
	IssuerID string
	BundleID string
	Key      *ecdsa.PrivateKey
	// Sandbox picks Apple's sandbox, where test purchases live.
	Sandbox bool

	Client  *http.Client
	BaseURL string // tests point it at a local server
}

// ParseKey reads the .p8 file App Store Connect gives for an In-App Purchase
// key.
func ParseKey(p8 []byte) (*ecdsa.PrivateKey, error) {
	key, err := es256.ParseP8(p8)
	if err != nil {
		return nil, fmt.Errorf("the In-App Purchase key: %w", err)
	}
	return key, nil
}

// RequestTestNotification asks Apple to send a TEST notification to the
// server URL set in App Store Connect, and returns the token to ask after it.
func (c AppStoreAPI) RequestTestNotification(ctx context.Context) (string, error) {
	var answer struct {
		Token string `json:"testNotificationToken"`
	}
	if err := c.call(ctx, http.MethodPost, "/inApps/v1/notifications/test", &answer); err != nil {
		return "", err
	}
	return answer.Token, nil
}

// TestNotification returns the signed payload of a test notification asked
// for earlier. Apple keeps it whether or not our server took it.
func (c AppStoreAPI) TestNotification(ctx context.Context, token string) (signedPayload string, err error) {
	var answer struct {
		SignedPayload string `json:"signedPayload"`
	}
	if err := c.call(ctx, http.MethodGet, "/inApps/v1/notifications/test/"+token, &answer); err != nil {
		return "", err
	}
	return answer.SignedPayload, nil
}

func (c AppStoreAPI) call(ctx context.Context, method, path string, into any) error {
	base := c.BaseURL
	if base == "" {
		base = "https://api.storekit.itunes.apple.com"
		if c.Sandbox {
			base = "https://api.storekit-sandbox.itunes.apple.com"
		}
	}
	token, err := c.token(time.Now())
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, method, base+path, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	client := c.Client
	if client == nil {
		client = &http.Client{Timeout: 15 * time.Second}
	}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("calling the App Store Server API: %w", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("the App Store Server API answered %d: %s", resp.StatusCode, bytes.TrimSpace(body))
	}
	if err := json.Unmarshal(body, into); err != nil {
		return fmt.Errorf("reading the App Store Server API's answer: %w", err)
	}
	return nil
}

// token is the JSON Web Token Apple asks for, signed with the key. Apple
// refuses one that lives longer than an hour; this one lives five minutes.
func (c AppStoreAPI) token(now time.Time) (string, error) {
	return es256.Sign(c.Key,
		map[string]string{"alg": "ES256", "kid": c.KeyID, "typ": "JWT"},
		map[string]any{
			"iss": c.IssuerID, "iat": now.Unix(), "exp": now.Add(5 * time.Minute).Unix(),
			"aud": "appstoreconnect-v1", "bid": c.BundleID,
		})
}
