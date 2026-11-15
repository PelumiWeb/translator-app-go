package push

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/PelumiWeb/translator-app-go/server/internal/provider"
)

const (
	apnsProduction = "https://api.push.apple.com"
	apnsSandbox    = "https://api.sandbox.push.apple.com"

	// Apple accepts a signed token for an hour and refuses new ones made
	// more often than every 20 minutes, so one is kept for 50.
	apnsTokenLife = 50 * time.Minute
)

// APNs sends notifications to iPhones through Apple's push service. It signs
// in with a key made in the developer account (Keys, Apple Push
// Notifications service), which works for every app of the team, in both
// the production and the test service.
type APNs struct {
	keyID  string
	teamID string
	topic  string // the app's bundle ID
	key    *ecdsa.PrivateKey

	// Client, Production and Sandbox default to an HTTP client with a
	// timeout and Apple's addresses; tests point them at a local server.
	Client     *http.Client
	Production string
	Sandbox    string
	now        func() time.Time

	mu      sync.Mutex
	token   string
	tokenAt time.Time
}

// NewAPNs reads the key file Apple gave (AuthKey_<keyID>.p8).
func NewAPNs(p8 []byte, keyID, teamID, topic string) (*APNs, error) {
	block, _ := pem.Decode(p8)
	if block == nil {
		return nil, errors.New("apns: the key file is not a .p8 key")
	}
	parsed, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("apns: reading the key: %w", err)
	}
	key, ok := parsed.(*ecdsa.PrivateKey)
	if !ok {
		return nil, errors.New("apns: the key is not an elliptic-curve key")
	}
	if keyID == "" || teamID == "" || topic == "" {
		return nil, errors.New("apns: the key ID, team ID and bundle ID are all needed")
	}
	return &APNs{keyID: keyID, teamID: teamID, topic: topic, key: key, now: time.Now}, nil
}

// apnsPayload is the body Apple expects: the alert under "aps", and the
// app's own data beside it.
func apnsPayload(n Notification) ([]byte, error) {
	aps := map[string]any{
		"alert": map[string]string{"title": n.Title, "body": n.Body},
		"sound": "default",
	}
	if n.Thread != "" {
		aps["thread-id"] = n.Thread
	}
	body := map[string]any{"aps": aps}
	for k, v := range n.Data {
		body[k] = v
	}
	return json.Marshal(body)
}

// Send delivers n to the iPhone with this device token, through the test
// service when sandbox is true.
func (a *APNs) Send(ctx context.Context, deviceToken string, sandbox bool, n Notification) error {
	body, err := apnsPayload(n)
	if err != nil {
		return provider.Permanent(fmt.Errorf("apns: %w", err))
	}
	auth, err := a.authToken()
	if err != nil {
		return provider.Permanent(err)
	}

	host := a.Production
	if host == "" {
		host = apnsProduction
	}
	if sandbox {
		host = a.Sandbox
		if host == "" {
			host = apnsSandbox
		}
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, host+"/3/device/"+deviceToken, bytes.NewReader(body))
	if err != nil {
		return provider.Permanent(fmt.Errorf("apns: building the request: %w", err))
	}
	req.Header.Set("Authorization", "bearer "+auth)
	req.Header.Set("Apns-Topic", a.topic)
	req.Header.Set("Apns-Push-Type", "alert")
	req.Header.Set("Apns-Priority", "10")
	// Kept for a day for a phone that is off; Apple keeps only the latest.
	req.Header.Set("Apns-Expiration", strconv.FormatInt(a.now().Add(24*time.Hour).Unix(), 10))
	if n.CollapseID != "" {
		req.Header.Set("Apns-Collapse-Id", n.CollapseID)
	}

	client := a.Client
	if client == nil {
		// Go's default transport speaks HTTP/2 to servers that offer it,
		// which Apple's service requires.
		client = &http.Client{Timeout: 15 * time.Second}
	}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("apns: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusOK {
		return nil
	}

	var failure struct {
		Reason string `json:"reason"`
	}
	data, _ := io.ReadAll(io.LimitReader(resp.Body, 4<<10))
	_ = json.Unmarshal(data, &failure)
	err = fmt.Errorf("apns: %d %s", resp.StatusCode, failure.Reason)
	switch {
	case resp.StatusCode == http.StatusGone,
		failure.Reason == "BadDeviceToken",
		failure.Reason == "DeviceTokenNotForTopic":
		// BadDeviceToken is also the answer for a token sent to the wrong
		// service, test or production: the phone registers again, with the
		// right one, the next time the app opens.
		return fmt.Errorf("%w (%v)", ErrGone, err)
	case failure.Reason == "ExpiredProviderToken":
		a.forgetToken()
		return err
	case resp.StatusCode == http.StatusTooManyRequests, resp.StatusCode >= 500:
		return err
	default:
		return provider.Permanent(err)
	}
}

// authToken returns the signed token that proves the server may send for
// the team, signing a new one when the last is about to expire.
func (a *APNs) authToken() (string, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	now := a.now()
	if a.token != "" && now.Sub(a.tokenAt) < apnsTokenLife {
		return a.token, nil
	}
	token, err := signES256(a.key,
		map[string]string{"alg": "ES256", "kid": a.keyID},
		map[string]any{"iss": a.teamID, "iat": now.Unix()},
	)
	if err != nil {
		return "", fmt.Errorf("apns: signing a token: %w", err)
	}
	a.token, a.tokenAt = token, now
	return token, nil
}

func (a *APNs) forgetToken() {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.token = ""
}

// signES256 makes a JSON Web Token signed with an elliptic-curve key.
func signES256(key *ecdsa.PrivateKey, header, claims any) (string, error) {
	input, err := jwtInput(header, claims)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256([]byte(input))
	r, s, err := ecdsa.Sign(rand.Reader, key, sum[:])
	if err != nil {
		return "", err
	}
	// A JWT carries the signature as the two numbers side by side, 32 bytes
	// each, not in the ASN.1 form Go's SignASN1 would give.
	sig := make([]byte, 64)
	r.FillBytes(sig[:32])
	s.FillBytes(sig[32:])
	return input + "." + b64.EncodeToString(sig), nil
}

// jwtInput is the part of a JSON Web Token that is signed.
func jwtInput(header, claims any) (string, error) {
	h, err := json.Marshal(header)
	if err != nil {
		return "", err
	}
	c, err := json.Marshal(claims)
	if err != nil {
		return "", err
	}
	return b64.EncodeToString(h) + "." + b64.EncodeToString(c), nil
}
