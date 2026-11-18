package auth

import (
	"errors"
	"strings"
	"testing"
)

func TestParseTestAccounts(t *testing.T) {
	tests := []struct {
		name    string
		in      string
		wantErr bool
	}{
		{"none", "", false},
		{"one", "tester@shinar.test:abcdefghijkl", false},
		{"two, with spaces", " one@shinar.test:abcdefghijkl , two@shinar.test:ABCDEF-123_xyz ", false},
		{"no code", "tester@shinar.test", true},
		{"not an address", "tester:abcdefghijkl", true},
		{"a short code", "tester@shinar.test:short", true},
		{"a code with a space", "tester@shinar.test:abcdef ghijkl", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ParseTestAccounts(tt.in)
			if (err != nil) != tt.wantErr {
				t.Errorf("err = %v, want an error: %v", err, tt.wantErr)
			}
		})
	}
}

func TestParseTestAccountsKeepsTheCodeOutOfItsErrors(t *testing.T) {
	_, err := ParseTestAccounts("tester@shinar.test:secret")
	if err == nil {
		t.Fatal("want an error")
	}
	for _, leaked := range []string{"secret", "tester"} {
		if strings.Contains(err.Error(), leaked) {
			t.Errorf("error %q names %q", err, leaked)
		}
	}
}

func TestCheckTestAccount(t *testing.T) {
	accounts, err := ParseTestAccounts("Tester@Shinar.test:abcdefghijkl")
	if err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name, email, code string
		wantErr           bool
	}{
		{"the right code", "tester@shinar.test", "abcdefghijkl", false},
		{"the address in other case, with spaces", "  TESTER@shinar.TEST ", "abcdefghijkl", false},
		{"a wrong code", "tester@shinar.test", "abcdefghijkX", true},
		{"the code in other case", "tester@shinar.test", "ABCDEFGHIJKL", true},
		{"another address", "someone@shinar.test", "abcdefghijkl", true},
		{"no code", "tester@shinar.test", "", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			identity, err := accounts.Check(tt.email, tt.code)
			if tt.wantErr {
				if !errors.Is(err, ErrInvalidCode) {
					t.Errorf("err = %v, want ErrInvalidCode", err)
				}
				return
			}
			if err != nil || identity.Subject != "tester@shinar.test" || identity.Email != "tester@shinar.test" {
				t.Errorf("identity = %+v, err = %v", identity, err)
			}
		})
	}
}
