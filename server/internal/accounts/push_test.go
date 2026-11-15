package accounts

import (
	"context"
	"errors"
	"slices"
	"testing"

	"github.com/PelumiWeb/translator-app-go/server/internal/testdb"
)

// pushTokens lists the stored push tokens as "service token user".
func pushTokens(t *testing.T, store *Store) []string {
	t.Helper()
	rows, err := store.db.Query(context.Background(), `
		SELECT p.service || ' ' || p.token || ' ' || u.display_name
		FROM push_tokens p JOIN sessions s ON s.token_hash = p.session_hash JOIN users u ON u.id = s.user_id
		ORDER BY 1`)
	if err != nil {
		t.Fatal(err)
	}
	var found []string
	for rows.Next() {
		var row string
		if err := rows.Scan(&row); err != nil {
			t.Fatal(err)
		}
		found = append(found, row)
	}
	return found
}

func TestPushTokensFollowTheSession(t *testing.T) {
	store := NewStore(testdb.New(t))
	ctx := context.Background()
	ada, adaUser, _, err := store.SignIn(ctx, ProviderApple, "sub-ada", "", "Ada")
	if err != nil {
		t.Fatal(err)
	}
	rosa, _, _, err := store.SignIn(ctx, ProviderGoogle, "sub-rosa", "", "Rosa")
	if err != nil {
		t.Fatal(err)
	}

	steps := []struct {
		name string
		do   func() error
		want []string
	}{
		{"each phone gives its address",
			func() error {
				if err := store.SetPushToken(ctx, ada, "apns-sandbox", "phone-a"); err != nil {
					return err
				}
				return store.SetPushToken(ctx, rosa, "fcm", "phone-r")
			},
			[]string{"apns-sandbox phone-a Ada", "fcm phone-r Rosa"}},
		{"a new address replaces the session's old one",
			func() error { return store.SetPushToken(ctx, ada, "apns", "phone-a2") },
			[]string{"apns phone-a2 Ada", "fcm phone-r Rosa"}},
		{"an address moves to the session that gives it last",
			func() error { return store.SetPushToken(ctx, ada, "fcm", "phone-r") },
			[]string{"fcm phone-r Ada"}},
		{"the push service forgets one",
			func() error { return store.ForgetPushToken(ctx, "phone-r") },
			nil},
		{"signing out takes the address away",
			func() error {
				if err := store.SetPushToken(ctx, ada, "apns", "phone-a"); err != nil {
					return err
				}
				return store.SignOut(ctx, ada)
			},
			nil},
		{"so does deleting the account",
			func() error {
				again, _, _, err := store.SignIn(ctx, ProviderApple, "sub-ada", "", "Ada")
				if err != nil {
					return err
				}
				if err := store.SetPushToken(ctx, again, "apns", "phone-a"); err != nil {
					return err
				}
				return store.DeleteUser(ctx, adaUser.ID)
			},
			nil},
	}
	for _, step := range steps {
		if err := step.do(); err != nil {
			t.Fatalf("%s: %v", step.name, err)
		}
		if got := pushTokens(t, store); !slices.Equal(got, step.want) {
			t.Errorf("%s: tokens %q, want %q", step.name, got, step.want)
		}
	}

	if err := store.SetPushToken(ctx, "no-such-session", "fcm", "phone-x"); !errors.Is(err, ErrNoSession) {
		t.Errorf("unknown session: err = %v, want ErrNoSession", err)
	}
}
