package push

import (
	"bytes"
	"context"
	"crypto"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/PelumiWeb/translator-app-go/server/internal/provider"
)

const (
	fcmBaseURL = "https://fcm.googleapis.com"
	fcmScope   = "https://www.googleapis.com/auth/firebase.messaging"
)

// FCM sends notifications to Android phones through Firebase Cloud
// Messaging. It signs in as the Firebase project's service account, with the
// key file from the Firebase console (Project settings, Service accounts).
type FCM struct {
	projectID string
	email     string
	keyID     string
	tokenURL  string
	key       *rsa.PrivateKey

	// Client and BaseURL default to an HTTP client with a timeout and
	// Google's address; tests point them at a local server.
	Client  *http.Client
	BaseURL string
	now     func() time.Time

	mu      sync.Mutex
	access  string
	expires time.Time
}

// NewFCM reads the service account's key file.
func NewFCM(serviceAccount []byte) (*FCM, error) {
	var file struct {
		Type         string `json:"type"`
		ProjectID    string `json:"project_id"`
		PrivateKeyID string `json:"private_key_id"`
		PrivateKey   string `json:"private_key"`
		ClientEmail  string `json:"client_email"`
		TokenURI     string `json:"token_uri"`
	}
	if err := json.Unmarshal(serviceAccount, &file); err != nil {
		return nil, fmt.Errorf("fcm: reading the service account: %w", err)
	}
	if file.Type != "service_account" || file.ProjectID == "" || file.ClientEmail == "" || file.TokenURI == "" {
		return nil, errors.New("fcm: the file is not a service account key")
	}
	block, _ := pem.Decode([]byte(file.PrivateKey))
	if block == nil {
		return nil, errors.New("fcm: the service account has no private key")
	}
	parsed, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("fcm: reading the private key: %w", err)
	}
	key, ok := parsed.(*rsa.PrivateKey)
	if !ok {
		return nil, errors.New("fcm: the private key is not an RSA key")
	}
	return &FCM{
		projectID: file.ProjectID,
		email:     file.ClientEmail,
		keyID:     file.PrivateKeyID,
		tokenURL:  file.TokenURI,
		key:       key,
		now:       time.Now,
	}, nil
}

// Send delivers n to the Android app with this Firebase installation ID (FID),
// which the app gets from FirebaseMessaging.register(). FCM is retiring the
// older registration tokens in favour of it.
//
// It sends a data message, not a notification message: the app then always
// decides what to show, and can stay quiet about a room that is open on the
// screen. High priority wakes the phone for it.
func (f *FCM) Send(ctx context.Context, token string, n Notification) error {
	data := map[string]string{"title": n.Title, "body": n.Body}
	if n.CollapseID != "" {
		data["collapse_id"] = n.CollapseID
	}
	for k, v := range n.Data {
		data[k] = v
	}
	body, err := json.Marshal(map[string]any{
		"message": map[string]any{
			"fid":  token,
			"data": data,
			// No collapse key: FCM keeps at most four of them per phone, so
			// the app replaces repeats itself, by collapse_id.
			"android": map[string]string{"priority": "high", "ttl": "86400s"},
		},
	})
	if err != nil {
		return provider.Permanent(fmt.Errorf("fcm: %w", err))
	}
	access, err := f.accessToken(ctx)
	if err != nil {
		return err
	}

	baseURL := f.BaseURL
	if baseURL == "" {
		baseURL = fcmBaseURL
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		baseURL+"/v1/projects/"+url.PathEscape(f.projectID)+"/messages:send", bytes.NewReader(body))
	if err != nil {
		return provider.Permanent(fmt.Errorf("fcm: building the request: %w", err))
	}
	req.Header.Set("Authorization", "Bearer "+access)
	req.Header.Set("Content-Type", "application/json")
	resp, err := f.client().Do(req)
	if err != nil {
		return fmt.Errorf("fcm: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusOK {
		return nil
	}

	code := fcmErrorCode(resp.Body)
	err = fmt.Errorf("fcm: %d %s", resp.StatusCode, code)
	switch {
	case code == "UNREGISTERED", code == "SENDER_ID_MISMATCH":
		// The app was removed, or the address belongs to another project.
		return fmt.Errorf("%w (%v)", ErrGone, err)
	case resp.StatusCode == http.StatusUnauthorized:
		f.forgetAccess()
		return err
	case resp.StatusCode == http.StatusTooManyRequests, resp.StatusCode >= 500:
		return err
	default:
		return provider.Permanent(err)
	}
}

// fcmErrorCode reads FCM's own error code from an error response, falling
// back to Google's general status.
func fcmErrorCode(body io.Reader) string {
	var failure struct {
		Error struct {
			Status  string `json:"status"`
			Details []struct {
				ErrorCode string `json:"errorCode"`
			} `json:"details"`
		} `json:"error"`
	}
	data, _ := io.ReadAll(io.LimitReader(body, 16<<10))
	_ = json.Unmarshal(data, &failure)
	for _, d := range failure.Error.Details {
		if d.ErrorCode != "" {
			return d.ErrorCode
		}
	}
	return failure.Error.Status
}

// accessToken returns an OAuth access token for FCM, asking Google for a new
// one when the last is about to expire. Google gives it in exchange for a
// token signed with the service account's key (the JWT bearer grant).
func (f *FCM) accessToken(ctx context.Context) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	now := f.now()
	if f.access != "" && now.Before(f.expires) {
		return f.access, nil
	}

	assertion, err := signRS256(f.key,
		map[string]string{"alg": "RS256", "typ": "JWT", "kid": f.keyID},
		map[string]any{
			"iss":   f.email,
			"scope": fcmScope,
			"aud":   f.tokenURL,
			"iat":   now.Unix(),
			"exp":   now.Add(time.Hour).Unix(),
		},
	)
	if err != nil {
		return "", provider.Permanent(fmt.Errorf("fcm: signing: %w", err))
	}
	form := url.Values{
		"grant_type": {"urn:ietf:params:oauth:grant-type:jwt-bearer"},
		"assertion":  {assertion},
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, f.tokenURL, strings.NewReader(form.Encode()))
	if err != nil {
		return "", provider.Permanent(fmt.Errorf("fcm: building the token request: %w", err))
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := f.client().Do(req)
	if err != nil {
		return "", fmt.Errorf("fcm: signing in: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		err := fmt.Errorf("fcm: signing in: %d", resp.StatusCode)
		if resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode >= 500 {
			return "", err
		}
		// A key deleted in the console, or a clock far off: no retry helps.
		return "", provider.Permanent(err)
	}
	var token struct {
		AccessToken string `json:"access_token"`
		ExpiresIn   int    `json:"expires_in"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&token); err != nil || token.AccessToken == "" {
		return "", fmt.Errorf("fcm: reading the access token: %v", err)
	}
	f.access = token.AccessToken
	// Five minutes early, so a token never expires on its way to Google.
	f.expires = now.Add(time.Duration(token.ExpiresIn)*time.Second - 5*time.Minute)
	return f.access, nil
}

func (f *FCM) forgetAccess() {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.access = ""
}

func (f *FCM) client() *http.Client {
	if f.Client != nil {
		return f.Client
	}
	return &http.Client{Timeout: 15 * time.Second}
}

// signRS256 makes a JSON Web Token signed with an RSA key.
func signRS256(key *rsa.PrivateKey, header, claims any) (string, error) {
	input, err := jwtInput(header, claims)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256([]byte(input))
	sig, err := rsa.SignPKCS1v15(nil, key, crypto.SHA256, sum[:])
	if err != nil {
		return "", err
	}
	return input + "." + b64.EncodeToString(sig), nil
}
