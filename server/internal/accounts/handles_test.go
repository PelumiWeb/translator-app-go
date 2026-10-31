package accounts

import (
	"context"
	"errors"
	"slices"
	"testing"

	"github.com/PelumiWeb/translator-app-go/server/internal/testdb"
)

func TestHandleCandidates(t *testing.T) {
	tests := []struct {
		name, handle, person string
		want                 []string // the first few
	}{
		{"two names", "pelumi", "Pelumi Adeyemi", []string{"pelumi.a", "pelumi_adeyemi", "pelumi26", "pelumi_26", "pelumi1"}},
		{"one name", "pelumi", "Pelumi", []string{"pelumi26", "pelumi_26", "pelumi1"}},
		{"accented last name is skipped", "rosa", "Rosa López", []string{"rosa26", "rosa_26", "rosa1"}},
		{"cut to fit twenty characters", "abcdefghijklmnopqrst", "Ada Lovelace", []string{"abcdefghijklmnopqr.l", "abcdefghijk_lovelace", "abcdefghijklmnopqr26"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := handleCandidates(tt.handle, tt.person, 2026)
			if len(got) < len(tt.want) || !slices.Equal(got[:len(tt.want)], tt.want) {
				t.Errorf("handleCandidates = %v, want it to start with %v", got, tt.want)
			}
			for _, c := range got {
				if !ValidHandle(c) || c == tt.handle {
					t.Errorf("candidate %q: want a valid ID other than %q", c, tt.handle)
				}
			}
		})
	}
}

func TestHandleFreeAndSuggestions(t *testing.T) {
	store := NewStore(testdb.New(t))
	ctx := context.Background()

	_, owner, _, err := store.SignIn(ctx, ProviderApple, "owner", "", "")
	if err != nil {
		t.Fatal(err)
	}
	_, other, _, err := store.SignIn(ctx, ProviderApple, "other", "", "")
	if err != nil {
		t.Fatal(err)
	}
	for user, handle := range map[string]string{owner.ID: "pelumi", other.ID: "pelumi.a"} {
		if _, err := store.UpdateProfile(ctx, user, ProfileUpdate{Handle: ptr(handle)}); err != nil {
			t.Fatal(err)
		}
	}

	tests := []struct {
		user, handle string
		want         bool
	}{
		{owner.ID, "pelumi", true}, // their own
		{other.ID, "pelumi", false},
		{other.ID, "nobody_has_this", true},
	}
	for _, tt := range tests {
		if free, err := store.HandleFree(ctx, tt.user, tt.handle); err != nil || free != tt.want {
			t.Errorf("HandleFree(%q) = %v, %v; want %v", tt.handle, free, err, tt.want)
		}
	}

	suggestions, err := store.SuggestHandles(ctx, "pelumi", "Pelumi Adeyemi", 3)
	if err != nil {
		t.Fatal(err)
	}
	// "pelumi.a" is taken, so the next three are offered.
	want := []string{"pelumi_adeyemi", "pelumi26", "pelumi_26"}
	if !slices.Equal(suggestions, want) {
		t.Errorf("SuggestHandles = %v, want %v", suggestions, want)
	}
}

func TestUserByHandleFindsOnlyThoseWithALanguage(t *testing.T) {
	store := NewStore(testdb.New(t))
	ctx := context.Background()

	_, user, _, err := store.SignIn(ctx, ProviderApple, "rosa", "", "Rosa")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.UpdateProfile(ctx, user.ID, ProfileUpdate{Handle: ptr("rosa.lm")}); err != nil {
		t.Fatal(err)
	}
	if _, err := store.UserByHandle(ctx, "rosa.lm"); !errors.Is(err, ErrNoUser) {
		t.Errorf("before choosing a language: err = %v, want ErrNoUser", err)
	}

	if _, err := store.UpdateProfile(ctx, user.ID, ProfileUpdate{Language: ptr("es")}); err != nil {
		t.Fatal(err)
	}
	found, err := store.UserByHandle(ctx, "rosa.lm")
	if err != nil || found.ID != user.ID || found.Language != "es" || found.DisplayName != "Rosa" {
		t.Errorf("UserByHandle = %+v, %v", found, err)
	}
	if _, err := store.UserByHandle(ctx, "nobody"); !errors.Is(err, ErrNoUser) {
		t.Errorf("unknown handle: err = %v, want ErrNoUser", err)
	}
}
