package push

import (
	"context"
	"crypto"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/PelumiWeb/translator-app-go/server/internal/provider"
)

var note = Notification{
	Title:      "Rosa",
	Body:       "Sent a voice note · 0:14",
	Thread:     "room-1",
	CollapseID: "message-1",
	Data:       map[string]string{"room_id": "room-1", "message_id": "message-1"},
}

// newTestAPNs is an APNs client with a fresh key, sending to server.
func newTestAPNs(t *testing.T, server *httptest.Server) (*APNs, *ecdsa.PublicKey) {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	der, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		t.Fatal(err)
	}
	a, err := NewAPNs(pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der}), "KEY1234567", "TEAM123456", "com.pelumi.shinar")
	if err != nil {
		t.Fatal(err)
	}
	a.Client = server.Client()
	a.Production = server.URL + "/production"
	a.Sandbox = server.URL + "/sandbox"
	return a, &key.PublicKey
}

// jwtParts checks a token's signature with verify and returns its header
// and claims.
func jwtParts(t *testing.T, token string, verify func(input string, sig []byte) bool) (header, claims map[string]any) {
	t.Helper()
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		t.Fatalf("token has %d parts", len(parts))
	}
	sig, err := b64.DecodeString(parts[2])
	if err != nil {
		t.Fatal(err)
	}
	if !verify(parts[0]+"."+parts[1], sig) {
		t.Fatal("the token's signature does not verify")
	}
	for i, into := range []*map[string]any{&header, &claims} {
		raw, err := b64.DecodeString(parts[i])
		if err != nil {
			t.Fatal(err)
		}
		if err := json.Unmarshal(raw, into); err != nil {
			t.Fatal(err)
		}
	}
	return header, claims
}

func verifyES256(key *ecdsa.PublicKey) func(string, []byte) bool {
	return func(input string, sig []byte) bool {
		sum := sha256.Sum256([]byte(input))
		r, s := new(big.Int).SetBytes(sig[:32]), new(big.Int).SetBytes(sig[32:])
		return len(sig) == 64 && ecdsa.Verify(key, sum[:], r, s)
	}
}

func TestAPNsSendsTheAlert(t *testing.T) {
	var got *http.Request
	var body map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = r
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Error(err)
		}
	}))
	defer server.Close()
	a, public := newTestAPNs(t, server)

	for _, sandbox := range []bool{false, true} {
		if err := a.Send(context.Background(), "abc123", sandbox, note); err != nil {
			t.Fatal(err)
		}
		want := "/production/3/device/abc123"
		if sandbox {
			want = "/sandbox/3/device/abc123"
		}
		if got.URL.Path != want {
			t.Errorf("sandbox %v: sent to %s, want %s", sandbox, got.URL.Path, want)
		}
	}

	for header, want := range map[string]string{
		"Apns-Topic":       "com.pelumi.shinar",
		"Apns-Push-Type":   "alert",
		"Apns-Priority":    "10",
		"Apns-Collapse-Id": "message-1",
	} {
		if v := got.Header.Get(header); v != want {
			t.Errorf("%s = %q, want %q", header, v, want)
		}
	}
	auth, ok := strings.CutPrefix(got.Header.Get("Authorization"), "bearer ")
	if !ok {
		t.Fatalf("Authorization = %q", got.Header.Get("Authorization"))
	}
	header, claims := jwtParts(t, auth, verifyES256(public))
	if header["alg"] != "ES256" || header["kid"] != "KEY1234567" || claims["iss"] != "TEAM123456" {
		t.Errorf("token header %v, claims %v", header, claims)
	}

	aps, _ := body["aps"].(map[string]any)
	alert, _ := aps["alert"].(map[string]any)
	if alert["title"] != "Rosa" || alert["body"] != "Sent a voice note · 0:14" || aps["thread-id"] != "room-1" {
		t.Errorf("aps = %v", aps)
	}
	if body["room_id"] != "room-1" || body["message_id"] != "message-1" {
		t.Errorf("the app's data is missing: %v", body)
	}
}

func TestAPNsKeepsItsTokenForFiftyMinutes(t *testing.T) {
	var tokens []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tokens = append(tokens, r.Header.Get("Authorization"))
	}))
	defer server.Close()
	a, _ := newTestAPNs(t, server)
	now := time.Date(2026, 11, 15, 12, 0, 0, 0, time.UTC)
	a.now = func() time.Time { return now }

	for _, step := range []time.Duration{0, 49 * time.Minute, 2 * time.Minute} {
		now = now.Add(step)
		if err := a.Send(context.Background(), "abc123", false, note); err != nil {
			t.Fatal(err)
		}
	}
	if tokens[0] != tokens[1] || tokens[1] == tokens[2] {
		t.Error("want the same token for 50 minutes, then a new one")
	}
}

func TestAPNsTellsWhatToRetry(t *testing.T) {
	tests := []struct {
		status    int
		reason    string
		gone      bool
		permanent bool
	}{
		{http.StatusGone, "Unregistered", true, false},
		{http.StatusBadRequest, "BadDeviceToken", true, false},
		{http.StatusBadRequest, "DeviceTokenNotForTopic", true, false},
		{http.StatusBadRequest, "BadTopic", false, true},
		{http.StatusForbidden, "InvalidProviderToken", false, true},
		{http.StatusForbidden, "ExpiredProviderToken", false, false},
		{http.StatusTooManyRequests, "TooManyRequests", false, false},
		{http.StatusServiceUnavailable, "ServiceUnavailable", false, false},
	}
	for _, tt := range tests {
		t.Run(tt.reason, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tt.status)
				fmt.Fprintf(w, `{"reason":%q}`, tt.reason)
			}))
			defer server.Close()
			a, _ := newTestAPNs(t, server)

			err := a.Send(context.Background(), "abc123", false, note)
			if err == nil {
				t.Fatal("want an error")
			}
			if errors.Is(err, ErrGone) != tt.gone || provider.IsPermanent(err) != tt.permanent {
				t.Errorf("err = %v: gone %v, permanent %v; want %v, %v",
					err, errors.Is(err, ErrGone), provider.IsPermanent(err), tt.gone, tt.permanent)
			}
		})
	}
}

func TestNewAPNsRefusesOtherKeys(t *testing.T) {
	rsaKey := testRSAKey(t)
	der, err := x509.MarshalPKCS8PrivateKey(rsaKey)
	if err != nil {
		t.Fatal(err)
	}
	for name, p8 := range map[string][]byte{
		"not a key": []byte("hello"),
		"RSA key":   pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der}),
	} {
		if _, err := NewAPNs(p8, "KEY1234567", "TEAM123456", "com.pelumi.shinar"); err == nil {
			t.Errorf("%s: want an error", name)
		}
	}
}

var (
	rsaOnce sync.Once
	rsaKey  *rsa.PrivateKey
)

// testRSAKey is made once: an RSA key takes a moment to make.
func testRSAKey(t *testing.T) *rsa.PrivateKey {
	t.Helper()
	rsaOnce.Do(func() {
		var err error
		if rsaKey, err = rsa.GenerateKey(rand.Reader, 2048); err != nil {
			t.Fatal(err)
		}
	})
	return rsaKey
}

// fcmServer plays Google: its token endpoint and FCM's send endpoint. send
// answers each message; it is nil for one that always accepts.
type fcmServer struct {
	*httptest.Server
	t       *testing.T
	key     *rsa.PublicKey
	send    func(w http.ResponseWriter)
	mu      sync.Mutex
	logins  int
	message map[string]any
}

func newFCMServer(t *testing.T) (*fcmServer, *FCM) {
	t.Helper()
	key := testRSAKey(t)
	s := &fcmServer{t: t, key: &key.PublicKey}
	s.Server = httptest.NewServer(http.HandlerFunc(s.handle))
	t.Cleanup(s.Close)

	der, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		t.Fatal(err)
	}
	account, err := json.Marshal(map[string]string{
		"type":           "service_account",
		"project_id":     "shinar-test",
		"private_key_id": "key-1",
		"private_key":    string(pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der})),
		"client_email":   "sender@shinar-test.iam.gserviceaccount.com",
		"token_uri":      s.URL + "/token",
	})
	if err != nil {
		t.Fatal(err)
	}
	f, err := NewFCM(account)
	if err != nil {
		t.Fatal(err)
	}
	f.Client = s.Client()
	f.BaseURL = s.URL
	return s, f
}

func (s *fcmServer) handle(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	defer s.mu.Unlock()
	switch r.URL.Path {
	case "/token":
		if err := r.ParseForm(); err != nil {
			s.t.Error(err)
		}
		if g := r.PostForm.Get("grant_type"); g != "urn:ietf:params:oauth:grant-type:jwt-bearer" {
			s.t.Errorf("grant_type = %q", g)
		}
		header, claims := jwtParts(s.t, r.PostForm.Get("assertion"), func(input string, sig []byte) bool {
			sum := sha256.Sum256([]byte(input))
			return rsa.VerifyPKCS1v15(s.key, crypto.SHA256, sum[:], sig) == nil
		})
		if header["alg"] != "RS256" || header["kid"] != "key-1" ||
			claims["iss"] != "sender@shinar-test.iam.gserviceaccount.com" ||
			claims["scope"] != fcmScope || claims["aud"] != s.URL+"/token" {
			s.t.Errorf("assertion header %v, claims %v", header, claims)
		}
		s.logins++
		fmt.Fprintf(w, `{"access_token":"access-%d","expires_in":3599,"token_type":"Bearer"}`, s.logins)
	case "/v1/projects/shinar-test/messages:send":
		if want := fmt.Sprintf("Bearer access-%d", s.logins); r.Header.Get("Authorization") != want {
			s.t.Errorf("Authorization = %q, want %q", r.Header.Get("Authorization"), want)
		}
		var body map[string]map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			s.t.Error(err)
		}
		s.message = body["message"]
		if s.send != nil {
			s.send(w)
		}
	default:
		s.t.Errorf("unexpected request to %s", r.URL.Path)
		w.WriteHeader(http.StatusNotFound)
	}
}

func TestFCMSendsADataMessage(t *testing.T) {
	server, f := newFCMServer(t)
	for range 2 {
		if err := f.Send(context.Background(), "phone-token", note); err != nil {
			t.Fatal(err)
		}
	}
	if server.logins != 1 {
		t.Errorf("signed in %d times for two messages, want once", server.logins)
	}

	m := server.message
	if m["fid"] != "phone-token" {
		t.Errorf("fid = %v, want the phone's installation ID", m["fid"])
	}
	if _, ok := m["notification"]; ok {
		t.Error("want a data message, with no notification for the system to show")
	}
	data, _ := m["data"].(map[string]any)
	for k, want := range map[string]string{
		"title": "Rosa", "body": "Sent a voice note · 0:14", "collapse_id": "message-1",
		"room_id": "room-1", "message_id": "message-1",
	} {
		if data[k] != want {
			t.Errorf("data[%q] = %v, want %q", k, data[k], want)
		}
	}
	android, _ := m["android"].(map[string]any)
	if android["priority"] != "high" {
		t.Errorf("android = %v", android)
	}
}

func TestFCMTellsWhatToRetry(t *testing.T) {
	tests := []struct {
		status    int
		code      string
		gone      bool
		permanent bool
	}{
		{http.StatusNotFound, "UNREGISTERED", true, false},
		{http.StatusForbidden, "SENDER_ID_MISMATCH", true, false},
		{http.StatusBadRequest, "INVALID_ARGUMENT", false, true},
		{http.StatusUnauthorized, "THIRD_PARTY_AUTH_ERROR", false, false},
		{http.StatusTooManyRequests, "QUOTA_EXCEEDED", false, false},
		{http.StatusServiceUnavailable, "UNAVAILABLE", false, false},
	}
	for _, tt := range tests {
		t.Run(tt.code, func(t *testing.T) {
			server, f := newFCMServer(t)
			server.send = func(w http.ResponseWriter) {
				w.WriteHeader(tt.status)
				fmt.Fprintf(w, `{"error":{"code":%d,"status":"X","details":[
					{"@type":"type.googleapis.com/google.firebase.fcm.v1.FcmError","errorCode":%q}]}}`,
					tt.status, tt.code)
			}
			err := f.Send(context.Background(), "phone-token", note)
			if err == nil {
				t.Fatal("want an error")
			}
			if errors.Is(err, ErrGone) != tt.gone || provider.IsPermanent(err) != tt.permanent {
				t.Errorf("err = %v: gone %v, permanent %v; want %v, %v",
					err, errors.Is(err, ErrGone), provider.IsPermanent(err), tt.gone, tt.permanent)
			}
		})
	}
}

func TestFCMSignsInAgainWhenRefused(t *testing.T) {
	server, f := newFCMServer(t)
	refuse := true
	server.send = func(w http.ResponseWriter) {
		if refuse {
			w.WriteHeader(http.StatusUnauthorized)
			refuse = false
		}
	}
	if err := f.Send(context.Background(), "phone-token", note); err == nil {
		t.Fatal("want the first send refused")
	}
	if err := f.Send(context.Background(), "phone-token", note); err != nil {
		t.Fatal(err)
	}
	if server.logins != 2 {
		t.Errorf("signed in %d times, want twice", server.logins)
	}
}

func TestNewFCMRefusesOtherFiles(t *testing.T) {
	for name, file := range map[string]string{
		"not JSON":          "hello",
		"not an account":    `{"type":"authorized_user"}`,
		"no private key":    `{"type":"service_account","project_id":"p","client_email":"e","token_uri":"u"}`,
		"private key wrong": `{"type":"service_account","project_id":"p","client_email":"e","token_uri":"u","private_key":"-----BEGIN PRIVATE KEY-----\nAAAA\n-----END PRIVATE KEY-----\n"}`,
	} {
		if _, err := NewFCM([]byte(file)); err == nil {
			t.Errorf("%s: want an error", name)
		}
	}
}

func TestServicesSendThroughTheRightOne(t *testing.T) {
	var paths []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		paths = append(paths, r.URL.Path)
		_, _ = io.Copy(io.Discard, r.Body)
	}))
	defer server.Close()
	a, _ := newTestAPNs(t, server)

	only := Services{APNs: a}
	for _, d := range []Device{
		{Service: ServiceAPNs, Token: "one"},
		{Service: ServiceAPNsSandbox, Token: "two"},
		{Service: ServiceFCM, Token: "three"}, // no FCM here: skipped
	} {
		if err := only.Push(context.Background(), d, note); err != nil {
			t.Errorf("%s: %v", d.Service, err)
		}
	}
	if want := "/production/3/device/one /sandbox/3/device/two"; strings.Join(paths, " ") != want {
		t.Errorf("sent to %v, want %s", paths, want)
	}
	if err := only.Push(context.Background(), Device{Service: "pager", Token: "x"}, note); !provider.IsPermanent(err) {
		t.Errorf("unknown service: err = %v, want a permanent error", err)
	}
	if (Services{}).Enabled() || !only.Enabled() {
		t.Error("Enabled is wrong")
	}
}
