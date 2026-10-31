package accounts

import (
	"context"
	"errors"
	"slices"
	"sync"
	"testing"

	"github.com/PelumiWeb/translator-app-go/server/internal/testdb"
)

func TestAVoiceIsKeptReplacedAndDeleted(t *testing.T) {
	store := NewStore(testdb.New(t))
	ctx := context.Background()
	_, user, _, _ := store.SignIn(ctx, ProviderApple, "sub-1", "", "Ada")

	if _, err := store.Voice(ctx, user.ID); !errors.Is(err, ErrNoVoice) {
		t.Fatalf("before recording: error = %v, want ErrNoVoice", err)
	}

	first, replaced, err := store.SetVoice(ctx, user.ID, "first.wav", "2026-10-31")
	if err != nil {
		t.Fatal(err)
	}
	if replaced != "" || first.BlobKey != "first.wav" || first.ConsentVersion != "2026-10-31" || first.ConsentedAt.IsZero() {
		t.Errorf("first voice = %+v, replaced %q", first, replaced)
	}

	_, replaced, err = store.SetVoice(ctx, user.ID, "second.wav", "2026-11-15")
	if err != nil {
		t.Fatal(err)
	}
	if replaced != "first.wav" {
		t.Errorf("replaced = %q, want the first recording's key, to delete it", replaced)
	}
	kept, err := store.Voice(ctx, user.ID)
	if err != nil || kept.BlobKey != "second.wav" || kept.ConsentVersion != "2026-11-15" {
		t.Errorf("kept voice = %+v, %v", kept, err)
	}

	key, err := store.DeleteVoice(ctx, user.ID)
	if err != nil || key != "second.wav" {
		t.Errorf("DeleteVoice = %q, %v", key, err)
	}
	if _, err := store.Voice(ctx, user.ID); !errors.Is(err, ErrNoVoice) {
		t.Errorf("after deleting: error = %v, want ErrNoVoice", err)
	}
	if _, err := store.DeleteVoice(ctx, user.ID); !errors.Is(err, ErrNoVoice) {
		t.Errorf("deleting twice: error = %v, want ErrNoVoice", err)
	}
}

func TestAVoiceNeedsAUser(t *testing.T) {
	store := NewStore(testdb.New(t))
	ctx := context.Background()
	_, user, _, _ := store.SignIn(ctx, ProviderApple, "sub-1", "", "Ada")
	if err := store.DeleteUser(ctx, user.ID); err != nil {
		t.Fatal(err)
	}

	if _, _, err := store.SetVoice(ctx, user.ID, "late.wav", "2026-10-31"); !errors.Is(err, ErrNoUser) {
		t.Errorf("error = %v, want ErrNoUser", err)
	}
}

func TestDeletingAUserForgetsTheirVoice(t *testing.T) {
	store := NewStore(testdb.New(t))
	ctx := context.Background()
	_, user, _, _ := store.SignIn(ctx, ProviderApple, "sub-1", "", "Ada")
	if _, _, err := store.SetVoice(ctx, user.ID, "voice.wav", "2026-10-31"); err != nil {
		t.Fatal(err)
	}

	if err := store.DeleteUser(ctx, user.ID); err != nil {
		t.Fatal(err)
	}

	if _, err := store.Voice(ctx, user.ID); !errors.Is(err, ErrNoVoice) {
		t.Errorf("error = %v, want ErrNoVoice", err)
	}
}

func TestUploadsAtOnceEachReplaceOneAndLeaveNoneBehind(t *testing.T) {
	store := NewStore(testdb.New(t))
	ctx := context.Background()
	_, user, _, _ := store.SignIn(ctx, ProviderApple, "sub-1", "", "Ada")

	keys := []string{"a.wav", "b.wav", "c.wav", "d.wav", "e.wav", "f.wav"}
	replaced := make([]string, len(keys))
	var wg sync.WaitGroup
	for i, key := range keys {
		wg.Go(func() {
			var err error
			if _, replaced[i], err = store.SetVoice(ctx, user.ID, key, "2026-10-31"); err != nil {
				t.Error(err)
			}
		})
	}
	wg.Wait()

	// Every key but the one kept was replaced exactly once, so each file is
	// either kept or handed back to be deleted.
	kept, err := store.Voice(ctx, user.ID)
	if err != nil {
		t.Fatal(err)
	}
	accounted := append(slices.DeleteFunc(slices.Clone(replaced), func(k string) bool { return k == "" }), kept.BlobKey)
	slices.Sort(accounted)
	if !slices.Equal(accounted, keys) {
		t.Errorf("replaced %v and kept %q; want each of %v once", replaced, kept.BlobKey, keys)
	}
}
