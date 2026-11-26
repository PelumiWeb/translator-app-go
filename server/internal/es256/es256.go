// Package es256 signs JSON Web Tokens with an elliptic-curve key from a .p8
// file, as Apple's services ask: the App Store Server API, and Sign in with
// Apple's REST API. (APNs has its own copy in internal/push, beside the RS256
// tokens Google wants.)
package es256

import (
	"crypto/ecdsa"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
)

// ParseP8 reads a key file App Store Connect or the developer account gives:
// a PKCS #8 elliptic-curve key in PEM.
func ParseP8(p8 []byte) (*ecdsa.PrivateKey, error) {
	block, _ := pem.Decode(p8)
	if block == nil {
		return nil, errors.New("the key file is not a PEM .p8 key")
	}
	parsed, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("reading the key: %w", err)
	}
	key, ok := parsed.(*ecdsa.PrivateKey)
	if !ok {
		return nil, errors.New("the key is not an elliptic-curve key")
	}
	return key, nil
}

// Sign makes a JSON Web Token of header and claims, signed with key.
func Sign(key *ecdsa.PrivateKey, header, claims any) (string, error) {
	h, err := json.Marshal(header)
	if err != nil {
		return "", err
	}
	c, err := json.Marshal(claims)
	if err != nil {
		return "", err
	}
	input := base64.RawURLEncoding.EncodeToString(h) + "." + base64.RawURLEncoding.EncodeToString(c)
	sum := sha256.Sum256([]byte(input))
	r, s, err := ecdsa.Sign(rand.Reader, key, sum[:])
	if err != nil {
		return "", fmt.Errorf("signing a token: %w", err)
	}
	// A JWT carries the signature as the two numbers side by side, 32 bytes
	// each, not in the ASN.1 form Go's SignASN1 would give.
	sig := make([]byte, 64)
	r.FillBytes(sig[:32])
	s.FillBytes(sig[32:])
	return input + "." + base64.RawURLEncoding.EncodeToString(sig), nil
}
