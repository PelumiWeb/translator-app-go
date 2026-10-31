package auth

import (
	"context"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"math/big"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

const (
	audience    = "pelumi.PTranslate"
	appleIssuer = "https://appleid.apple.com"
)

var now = time.Unix(1_800_000_000, 0)

// appleKeys serves a key set holding key under the ID "k1", the way Apple
// and Google serve theirs, and counts the requests.
func appleKeys(t *testing.T, key *rsa.PublicKey) (*httptest.Server, *atomic.Int32) {
	t.Helper()
	var fetches atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fetches.Add(1)
		json.NewEncoder(w).Encode(map[string]any{"keys": []map[string]string{{
			"kty": "RSA", "kid": "k1", "use": "sig", "alg": "RS256",
			"n": base64.RawURLEncoding.EncodeToString(key.N.Bytes()),
			"e": base64.RawURLEncoding.EncodeToString(big.NewInt(int64(key.E)).Bytes()),
		}}})
	}))
	t.Cleanup(server.Close)
	return server, &fetches
}

func newKey(t *testing.T) *rsa.PrivateKey {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	return key
}

// token builds a JWT as Apple does: header and claims in URL-safe base64,
// signed with RS256.
func token(t *testing.T, key *rsa.PrivateKey, alg, kid string, claims map[string]any) string {
	t.Helper()
	encode := func(v any) string {
		raw, err := json.Marshal(v)
		if err != nil {
			t.Fatal(err)
		}
		return base64.RawURLEncoding.EncodeToString(raw)
	}
	unsigned := encode(map[string]string{"alg": alg, "kid": kid}) + "." + encode(claims)
	digest := sha256.Sum256([]byte(unsigned))
	signature, err := rsa.SignPKCS1v15(rand.Reader, key, crypto.SHA256, digest[:])
	if err != nil {
		t.Fatal(err)
	}
	return unsigned + "." + base64.RawURLEncoding.EncodeToString(signature)
}

func hashed(nonce string) string {
	sum := sha256.Sum256([]byte(nonce))
	return hex.EncodeToString(sum[:])
}

// goodClaims are what Apple puts in a token for this app, with changes on top.
func goodClaims(changes map[string]any) map[string]any {
	claims := map[string]any{
		"iss":            appleIssuer,
		"aud":            audience,
		"exp":            now.Add(10 * time.Minute).Unix(),
		"iat":            now.Unix(),
		"sub":            "001234.abcd",
		"email":          "ada@privaterelay.appleid.com",
		"email_verified": "true",
		"nonce":          hashed("raw-nonce"),
	}
	for k, v := range changes {
		claims[k] = v
	}
	return claims
}

func TestVerify(t *testing.T) {
	key, other := newKey(t), newKey(t)
	keys, _ := appleKeys(t, &key.PublicKey)
	verifier := newVerifier([]string{appleIssuer}, audience, keys.URL, keys.Client(), func() time.Time { return now })

	tests := []struct {
		name    string
		token   string
		nonce   string
		invalid bool
	}{
		{"a good token", token(t, key, "RS256", "k1", goodClaims(nil)), "raw-nonce", false},
		{"for another app", token(t, key, "RS256", "k1", goodClaims(map[string]any{"aud": "other.app"})), "raw-nonce", true},
		{"from another issuer", token(t, key, "RS256", "k1", goodClaims(map[string]any{"iss": "https://example.com"})), "raw-nonce", true},
		{"expired", token(t, key, "RS256", "k1", goodClaims(map[string]any{"exp": now.Unix() - 1})), "raw-nonce", true},
		{"about no one", token(t, key, "RS256", "k1", goodClaims(map[string]any{"sub": ""})), "raw-nonce", true},
		{"made for another sign-in", token(t, key, "RS256", "k1", goodClaims(nil)), "another-nonce", true},
		{"without a nonce", token(t, key, "RS256", "k1", goodClaims(nil)), "", true},
		{"signed with another key", token(t, other, "RS256", "k1", goodClaims(nil)), "raw-nonce", true},
		{"naming a key Apple does not have", token(t, key, "RS256", "k2", goodClaims(nil)), "raw-nonce", true},
		{"claiming another algorithm", token(t, key, "none", "k1", goodClaims(nil)), "raw-nonce", true},
		{"not a JWT at all", "not-a-token", "raw-nonce", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			identity, err := verifier.Verify(context.Background(), tt.token, tt.nonce)
			if tt.invalid {
				if !errors.Is(err, ErrInvalidToken) {
					t.Fatalf("error = %v, want ErrInvalidToken", err)
				}
				return
			}
			if err != nil {
				t.Fatalf("Verify: %v", err)
			}
			want := Identity{Subject: "001234.abcd", Email: "ada@privaterelay.appleid.com"}
			if identity != want {
				t.Errorf("identity = %+v, want %+v", identity, want)
			}
		})
	}
}

func TestVerifyFetchesApplesKeysOnceAndKeepsThem(t *testing.T) {
	key := newKey(t)
	keys, fetches := appleKeys(t, &key.PublicKey)
	clock := now
	verifier := newVerifier([]string{appleIssuer}, audience, keys.URL, keys.Client(), func() time.Time { return clock })

	for range 3 {
		if _, err := verifier.Verify(context.Background(), token(t, key, "RS256", "k1", goodClaims(nil)), "raw-nonce"); err != nil {
			t.Fatal(err)
		}
	}
	if got := fetches.Load(); got != 1 {
		t.Errorf("fetched the keys %d times for three sign-ins, want once", got)
	}

	// A day later they are fetched again, in case Apple has rotated them.
	clock = now.Add(25 * time.Hour)
	claims := goodClaims(map[string]any{"exp": clock.Add(time.Minute).Unix()})
	if _, err := verifier.Verify(context.Background(), token(t, key, "RS256", "k1", claims), "raw-nonce"); err != nil {
		t.Fatal(err)
	}
	if got := fetches.Load(); got != 2 {
		t.Errorf("fetches = %d after a day, want 2", got)
	}
}

func TestVerifyWhenAppleCannotBeReachedIsNotAnInvalidToken(t *testing.T) {
	key := newKey(t)
	keys, _ := appleKeys(t, &key.PublicKey)
	keys.Close() // nothing listens there any more
	verifier := newVerifier([]string{appleIssuer}, audience, keys.URL, &http.Client{Timeout: time.Second}, func() time.Time { return now })

	_, err := verifier.Verify(context.Background(), token(t, key, "RS256", "k1", goodClaims(nil)), "raw-nonce")

	if err == nil || errors.Is(err, ErrInvalidToken) {
		t.Errorf("error = %v, want a failure to check, not an invalid token", err)
	}
}

func TestVerifyGoogle(t *testing.T) {
	key := newKey(t)
	keys, _ := appleKeys(t, &key.PublicKey)
	verifier := newVerifier([]string{"https://accounts.google.com", "accounts.google.com"}, "web-client.apps.googleusercontent.com",
		keys.URL, keys.Client(), func() time.Time { return now })
	googleClaims := func(changes map[string]any) map[string]any {
		claims := goodClaims(map[string]any{
			"iss":            "https://accounts.google.com",
			"aud":            "web-client.apps.googleusercontent.com",
			"sub":            "1098765",
			"email":          "ada@gmail.com",
			"email_verified": true,
			"name":           "Ada Lovelace",
		})
		for k, v := range changes {
			claims[k] = v
		}
		return claims
	}

	tests := []struct {
		name   string
		claims map[string]any
		want   Identity
	}{
		{"a good token", googleClaims(nil), Identity{Subject: "1098765", Email: "ada@gmail.com", Name: "Ada Lovelace"}},
		{"the issuer without its scheme", googleClaims(map[string]any{"iss": "accounts.google.com"}), Identity{Subject: "1098765", Email: "ada@gmail.com", Name: "Ada Lovelace"}},
		{"an email Google has not verified is left out", googleClaims(map[string]any{"email_verified": false}), Identity{Subject: "1098765", Name: "Ada Lovelace"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			identity, err := verifier.Verify(context.Background(), token(t, key, "RS256", "k1", tt.claims), "raw-nonce")
			if err != nil {
				t.Fatalf("Verify: %v", err)
			}
			if identity != tt.want {
				t.Errorf("identity = %+v, want %+v", identity, tt.want)
			}
		})
	}

	// An Apple token is not a Google one, even signed with the same key.
	if _, err := verifier.Verify(context.Background(), token(t, key, "RS256", "k1", googleClaims(map[string]any{"iss": appleIssuer})), "raw-nonce"); !errors.Is(err, ErrInvalidToken) {
		t.Errorf("Apple's issuer: error = %v, want ErrInvalidToken", err)
	}
}
