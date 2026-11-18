package auth

import (
	"crypto/subtle"
	"errors"
	"fmt"
	"strings"
)

// ErrInvalidCode means the address and code are not a pair this server
// accepts.
var ErrInvalidCode = errors.New("auth: invalid sign-in code")

// minCodeLength keeps a fixed code out of reach of guessing: the server has
// no limit on attempts, so the length is the defence. Twelve letters and
// digits are about 10^21 possibilities.
const minCodeLength = 12

// TestAccounts are email addresses that sign in with a fixed code set on the
// server, instead of one sent by mail: accounts for testing with a second
// phone, and the demo account App Review asks for. Codes sent by mail wait for
// a mail service (ADR 0018); these addresses keep their fixed codes then.
type TestAccounts struct {
	codes map[string]string // by address, in lower case
}

// ParseTestAccounts reads accounts written as "address:code" pairs separated
// by commas, as in TEST_ACCOUNTS. An empty string means none, and nil.
func ParseTestAccounts(s string) (*TestAccounts, error) {
	if strings.TrimSpace(s) == "" {
		return nil, nil
	}
	t := &TestAccounts{codes: map[string]string{}}
	for i, pair := range strings.Split(s, ",") {
		address, code, ok := strings.Cut(strings.TrimSpace(pair), ":")
		address = NormalizeEmail(address)
		// The entry's place, never its content, goes in the error: it is a
		// secret, and errors end up in logs.
		switch {
		case !ok || !strings.Contains(address, "@"):
			return nil, fmt.Errorf("test account %d: want address:code", i+1)
		case len(code) < minCodeLength || !isCodeText(code):
			return nil, fmt.Errorf("test account %d: the code must be at least %d letters, digits, - or _", i+1, minCodeLength)
		}
		t.codes[address] = code
	}
	return t, nil
}

// Check returns the identity of the test account with this address, if code
// is its code.
func (t *TestAccounts) Check(email, code string) (Identity, error) {
	address := NormalizeEmail(email)
	want, ok := t.codes[address]
	// A constant-time comparison, so the time taken says nothing about how
	// much of a guess was right.
	if !ok || subtle.ConstantTimeCompare([]byte(code), []byte(want)) != 1 {
		return Identity{}, ErrInvalidCode
	}
	return Identity{Subject: address, Email: address}, nil
}

// Len is how many test accounts there are.
func (t *TestAccounts) Len() int {
	return len(t.codes)
}

// NormalizeEmail is how an address is compared and kept: trimmed, in lower
// case. Strictly the part before the @ may be case-sensitive, but no mail
// service people use treats it so.
func NormalizeEmail(address string) string {
	return strings.ToLower(strings.TrimSpace(address))
}

func isCodeText(code string) bool {
	for _, r := range code {
		if !(r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '-' || r == '_') {
			return false
		}
	}
	return true
}
