package auth

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/PelumiWeb/translator-app-go/server/internal/es256"
)

func testTokens(t *testing.T, handler http.HandlerFunc) AppleTokens {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	apple := httptest.NewServer(handler)
	t.Cleanup(apple.Close)
	return AppleTokens{TeamID: "TEAM", KeyID: "KEY", ClientID: "com.pelumi.shinar", Key: key, BaseURL: apple.URL}
}

func TestTheCodeIsTradedForARefreshToken(t *testing.T) {
	tokens := testTokens(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/auth/token" || r.FormValue("grant_type") != "authorization_code" || r.FormValue("code") != "code-1" ||
			r.FormValue("client_id") != "com.pelumi.shinar" {
			t.Errorf("request = %s %v", r.URL.Path, r.Form)
		}
		// The client secret is a token for the app, from the team, for Apple.
		parts := strings.Split(r.FormValue("client_secret"), ".")
		raw, _ := base64.RawURLEncoding.DecodeString(parts[1])
		var claims map[string]any
		if err := json.Unmarshal(raw, &claims); err != nil || claims["iss"] != "TEAM" || claims["sub"] != "com.pelumi.shinar" ||
			claims["aud"] != "https://appleid.apple.com" {
			t.Errorf("client secret claims = %v", claims)
		}
		w.Write([]byte(`{"access_token":"a","token_type":"bearer","expires_in":3600,"refresh_token":"r-1","id_token":"i"}`))
	})

	got, err := tokens.Exchange(context.Background(), "code-1")
	if err != nil || got != "r-1" {
		t.Errorf("Exchange = %q, %v", got, err)
	}
}

func TestRevokingNamesTheRefreshToken(t *testing.T) {
	tokens := testTokens(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/auth/revoke" || r.FormValue("token") != "r-1" || r.FormValue("token_type_hint") != "refresh_token" {
			t.Errorf("request = %s %v", r.URL.Path, r.Form)
		}
	})
	if err := tokens.Revoke(context.Background(), "r-1"); err != nil {
		t.Error(err)
	}
}

func TestARefusalGivesAppleReason(t *testing.T) {
	tokens := testTokens(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(`{"error":"invalid_grant","error_description":"The code has expired or has been revoked."}`))
	})
	_, err := tokens.Exchange(context.Background(), "old")
	var refused ErrAppleRefused
	if !errors.As(err, &refused) || refused.Reason != "invalid_grant" {
		t.Errorf("err = %v, want Apple's invalid_grant", err)
	}
}

// TestLiveSignInWithAppleKey checks the Sign in with Apple key with Apple: a
// made-up code must be refused as a bad code ("invalid_grant"), which Apple
// says only once the app's signature is accepted; a key, team or app it does
// not know is "invalid_client". It runs only with the key (make signin-check).
func TestLiveSignInWithAppleKey(t *testing.T) {
	keyFile, keyID, team := os.Getenv("APPLE_SIGNIN_KEY_FILE"), os.Getenv("APPLE_SIGNIN_KEY_ID"), os.Getenv("APNS_TEAM_ID")
	if keyFile == "" || keyID == "" {
		t.Skip("APPLE_SIGNIN_KEY_FILE and APPLE_SIGNIN_KEY_ID are not set")
	}
	if team == "" {
		team = "4USZRKF452"
	}
	p8, err := os.ReadFile(keyFile)
	if err != nil {
		t.Fatal(err)
	}
	key, err := es256.ParseP8(p8)
	if err != nil {
		t.Fatal(err)
	}
	tokens := AppleTokens{TeamID: team, KeyID: keyID, ClientID: "com.pelumi.shinar", Key: key}
	_, err = tokens.Exchange(context.Background(), "not-a-real-code")
	var refused ErrAppleRefused
	if !errors.As(err, &refused) || refused.Reason != "invalid_grant" {
		t.Fatalf("Apple answered %v, want invalid_grant: the key, the team or the app is not accepted", err)
	}
	t.Log("Apple accepted the key: a made-up code was refused as a bad code, not as a bad client")
}
