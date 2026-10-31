// Package auth checks the identity tokens that phones get from Sign in with
// Apple and Sign in with Google before the server trusts who someone is.
package auth

import (
	"context"
	"crypto"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"net/http"
	"slices"
	"strings"
	"sync"
	"time"
)

// ErrInvalidToken means the token is not one this server accepts: forged,
// expired, issued for another app, or not tied to this sign-in. Anything else
// Verify returns is a failure to check it, such as the provider's keys being
// out of reach, which may pass.
var ErrInvalidToken = errors.New("auth: invalid identity token")

const (
	// Apple and Google rotate their signing keys now and then; a day-old copy
	// is fine, because an unknown key ID triggers a fresh fetch anyway.
	keysMaxAge = 24 * time.Hour
	// But never more than once a minute, so a stream of forged tokens with
	// made-up key IDs cannot turn this server into a load on theirs.
	minRefetch = time.Minute
)

// Identity is what a verified token says about the person.
type Identity struct {
	Subject string // the provider's stable ID for this person in this app
	Email   string // empty unless verified; from Apple, perhaps a private relay address
	Name    string // Google puts the name in its tokens; Apple never does
}

// Verifier checks one provider's identity tokens. Apple's and Google's are
// both JSON Web Tokens signed with RS256; the checks below are the whole of
// what a JWT library would do for them, so the standard library is enough.
type Verifier struct {
	issuers  []string
	audience string // the app the token must be issued for
	keysURL  string
	client   *http.Client
	now      func() time.Time

	mu        sync.Mutex
	keys      map[string]*rsa.PublicKey // by key ID
	fetchedAt time.Time
}

// NewAppleVerifier accepts Sign in with Apple tokens issued for the iOS app
// with this bundle ID.
func NewAppleVerifier(bundleID string) *Verifier {
	return newVerifier([]string{"https://appleid.apple.com"}, bundleID,
		"https://appleid.apple.com/auth/keys", defaultClient(), time.Now)
}

// NewGoogleVerifier accepts Sign in with Google tokens issued for this OAuth
// client ID: the "web" client the Android app names when it asks Google to
// sign the person in, which makes the server the token's audience.
func NewGoogleVerifier(clientID string) *Verifier {
	// Google writes its issuer both with and without the scheme.
	return newVerifier([]string{"https://accounts.google.com", "accounts.google.com"}, clientID,
		"https://www.googleapis.com/oauth2/v3/certs", defaultClient(), time.Now)
}

func defaultClient() *http.Client {
	return &http.Client{Timeout: 10 * time.Second}
}

func newVerifier(issuers []string, audience, keysURL string, client *http.Client, now func() time.Time) *Verifier {
	return &Verifier{issuers: issuers, audience: audience, keysURL: keysURL, client: client, now: now}
}

// Verify checks a token and returns who it is about. nonce is the random
// string the phone made for this sign-in; the phone gave the provider its
// SHA-256, which the provider wrote into the token. Matching the two shows the
// token was made for this sign-in and is not one captured earlier and sent
// again.
func (v *Verifier) Verify(ctx context.Context, token, nonce string) (Identity, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return Identity{}, fmt.Errorf("%w: not a JWT", ErrInvalidToken)
	}

	var header struct {
		Alg string `json:"alg"`
		Kid string `json:"kid"`
	}
	if err := decodeSegment(parts[0], &header); err != nil {
		return Identity{}, err
	}
	// Only RS256: accepting whatever the token names would let a forger pick
	// "none", or a symmetric algorithm keyed with the public key.
	if header.Alg != "RS256" {
		return Identity{}, fmt.Errorf("%w: algorithm %q", ErrInvalidToken, header.Alg)
	}

	key, err := v.key(ctx, header.Kid)
	if err != nil {
		return Identity{}, err
	}
	signature, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil {
		return Identity{}, fmt.Errorf("%w: unreadable signature", ErrInvalidToken)
	}
	digest := sha256.Sum256([]byte(parts[0] + "." + parts[1]))
	if err := rsa.VerifyPKCS1v15(key, crypto.SHA256, digest[:], signature); err != nil {
		return Identity{}, fmt.Errorf("%w: bad signature", ErrInvalidToken)
	}

	// Only now, with the signature good, are the claims worth reading.
	var claims struct {
		Iss   string `json:"iss"`
		Aud   string `json:"aud"`
		Exp   int64  `json:"exp"`
		Sub   string `json:"sub"`
		Email string `json:"email"`
		// Google writes true; Apple has written both true and "true".
		EmailVerified any    `json:"email_verified"`
		Name          string `json:"name"`
		Nonce         string `json:"nonce"`
	}
	if err := decodeSegment(parts[1], &claims); err != nil {
		return Identity{}, err
	}
	sum := sha256.Sum256([]byte(nonce))
	switch {
	case !slices.Contains(v.issuers, claims.Iss):
		return Identity{}, fmt.Errorf("%w: issued by %q", ErrInvalidToken, claims.Iss)
	case claims.Aud != v.audience:
		return Identity{}, fmt.Errorf("%w: issued for %q", ErrInvalidToken, claims.Aud)
	case v.now().Unix() >= claims.Exp:
		return Identity{}, fmt.Errorf("%w: expired", ErrInvalidToken)
	case claims.Sub == "":
		return Identity{}, fmt.Errorf("%w: no subject", ErrInvalidToken)
	case nonce == "" || claims.Nonce != hex.EncodeToString(sum[:]):
		return Identity{}, fmt.Errorf("%w: nonce does not match", ErrInvalidToken)
	}

	identity := Identity{Subject: claims.Sub, Name: claims.Name}
	if claims.EmailVerified == true || claims.EmailVerified == "true" {
		identity.Email = claims.Email
	}
	return identity, nil
}

// key returns the provider's public key with this ID, fetching the set when
// it is stale or does not have it.
func (v *Verifier) key(ctx context.Context, kid string) (*rsa.PublicKey, error) {
	// One lock around check and fetch: concurrent sign-ins wait for a single
	// fetch instead of each making their own.
	v.mu.Lock()
	defer v.mu.Unlock()

	age := v.now().Sub(v.fetchedAt)
	if key, ok := v.keys[kid]; ok && age < keysMaxAge {
		return key, nil
	}
	if v.keys == nil || age >= minRefetch {
		keys, err := v.fetchKeys(ctx)
		if err != nil {
			return nil, err
		}
		v.keys, v.fetchedAt = keys, v.now()
	}
	if key, ok := v.keys[kid]; ok {
		return key, nil
	}
	return nil, fmt.Errorf("%w: unknown key %q", ErrInvalidToken, kid)
}

func (v *Verifier) fetchKeys(ctx context.Context) (map[string]*rsa.PublicKey, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, v.keysURL, nil)
	if err != nil {
		return nil, fmt.Errorf("building the request for signing keys: %w", err)
	}
	resp, err := v.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetching signing keys: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("fetching signing keys: status %d", resp.StatusCode)
	}

	// A JSON Web Key Set: each RSA key is its modulus (n) and exponent (e),
	// both big-endian numbers written in URL-safe base64.
	var set struct {
		Keys []struct {
			Kty string `json:"kty"`
			Kid string `json:"kid"`
			N   string `json:"n"`
			E   string `json:"e"`
		} `json:"keys"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&set); err != nil {
		return nil, fmt.Errorf("reading signing keys: %w", err)
	}
	keys := make(map[string]*rsa.PublicKey, len(set.Keys))
	for _, k := range set.Keys {
		if k.Kty != "RSA" {
			continue
		}
		n, errN := base64.RawURLEncoding.DecodeString(k.N)
		e, errE := base64.RawURLEncoding.DecodeString(k.E)
		if errN != nil || errE != nil || len(e) > 4 {
			continue
		}
		keys[k.Kid] = &rsa.PublicKey{N: new(big.Int).SetBytes(n), E: int(new(big.Int).SetBytes(e).Int64())}
	}
	return keys, nil
}

func decodeSegment(segment string, into any) error {
	raw, err := base64.RawURLEncoding.DecodeString(segment)
	if err != nil {
		return fmt.Errorf("%w: unreadable segment", ErrInvalidToken)
	}
	if err := json.Unmarshal(raw, into); err != nil {
		return fmt.Errorf("%w: unreadable JSON", ErrInvalidToken)
	}
	return nil
}
