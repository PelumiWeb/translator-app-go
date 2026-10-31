package accounts

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/PelumiWeb/translator-app-go/server/internal/testdb"
)

func ptr(s string) *string { return &s }

func TestSignInCreatesTheUserOnceAndOpensASessionEachTime(t *testing.T) {
	store := NewStore(testdb.New(t))
	ctx := context.Background()

	first, user, created, err := store.SignIn(ctx, ProviderApple, "sub-1", "ada@example.com", "Ada")
	if err != nil {
		t.Fatal(err)
	}
	if !created || user.DisplayName != "Ada" || user.ID == "" {
		t.Errorf("first sign-in: created = %v, user = %+v", created, user)
	}

	second, again, created, err := store.SignIn(ctx, ProviderApple, "sub-1", "", "Not Ada")
	if err != nil {
		t.Fatal(err)
	}
	if created || again.ID != user.ID {
		t.Errorf("second sign-in: created = %v, user = %+v, want the same user", created, again)
	}
	if again.DisplayName != "Ada" {
		t.Errorf("name = %q: a later sign-in must not overwrite it", again.DisplayName)
	}
	if first == second || len(first) < 40 {
		t.Errorf("tokens %q and %q: each sign-in needs its own, long enough not to guess", first, second)
	}
	for _, token := range []string{first, second} {
		if got, err := store.UserForToken(ctx, token); err != nil || got.ID != user.ID {
			t.Errorf("UserForToken = %+v, %v", got, err)
		}
	}
}

func TestTheSameFirstSignInTwiceAtOnceMakesOneUser(t *testing.T) {
	db := testdb.New(t)
	store := NewStore(db)
	ctx := context.Background()

	ids := make([]string, 8)
	var wg sync.WaitGroup
	for i := range ids {
		wg.Go(func() {
			_, user, _, err := store.SignIn(ctx, ProviderApple, "sub-race", "", "")
			if err != nil {
				t.Error(err)
			}
			ids[i] = user.ID
		})
	}
	wg.Wait()

	for _, id := range ids {
		if id != ids[0] {
			t.Fatalf("user IDs = %v, want one user", ids)
		}
	}
	var users int
	if err := db.QueryRow(ctx, `SELECT count(*) FROM users`).Scan(&users); err != nil {
		t.Fatal(err)
	}
	if users != 1 {
		t.Errorf("%d users, want 1", users)
	}
}

func TestAnUnknownTokenHasNoSession(t *testing.T) {
	store := NewStore(testdb.New(t))

	if _, err := store.UserForToken(context.Background(), "made-up"); !errors.Is(err, ErrNoSession) {
		t.Errorf("error = %v, want ErrNoSession", err)
	}
}

func TestSignOutEndsOnlyThatSession(t *testing.T) {
	store := NewStore(testdb.New(t))
	ctx := context.Background()
	phone, _, _, _ := store.SignIn(ctx, ProviderApple, "sub-1", "", "Ada")
	tablet, _, _, _ := store.SignIn(ctx, ProviderApple, "sub-1", "", "Ada")

	if err := store.SignOut(ctx, phone); err != nil {
		t.Fatal(err)
	}

	if _, err := store.UserForToken(ctx, phone); !errors.Is(err, ErrNoSession) {
		t.Errorf("signed-out token: error = %v, want ErrNoSession", err)
	}
	if _, err := store.UserForToken(ctx, tablet); err != nil {
		t.Errorf("the other session: %v", err)
	}
}

func TestUpdateProfileChangesOnlyWhatIsGiven(t *testing.T) {
	store := NewStore(testdb.New(t))
	ctx := context.Background()
	_, user, _, _ := store.SignIn(ctx, ProviderApple, "sub-1", "", "Ada")

	updated, err := store.UpdateProfile(ctx, user.ID, ProfileUpdate{Handle: ptr("ada"), Language: ptr("yo")})
	if err != nil {
		t.Fatal(err)
	}
	want := User{ID: user.ID, DisplayName: "Ada", Handle: "ada", Language: "yo"}
	if updated != want {
		t.Errorf("after the update: %+v, want %+v", updated, want)
	}

	renamed, err := store.UpdateProfile(ctx, user.ID, ProfileUpdate{DisplayName: ptr("Ada L.")})
	if err != nil {
		t.Fatal(err)
	}
	want.DisplayName = "Ada L."
	if renamed != want {
		t.Errorf("after renaming: %+v, want %+v", renamed, want)
	}
}

func TestAHandleBelongsToOneUser(t *testing.T) {
	store := NewStore(testdb.New(t))
	ctx := context.Background()
	_, ada, _, _ := store.SignIn(ctx, ProviderApple, "sub-1", "", "Ada")
	_, bola, _, _ := store.SignIn(ctx, ProviderApple, "sub-2", "", "Bola")
	if _, err := store.UpdateProfile(ctx, ada.ID, ProfileUpdate{Handle: ptr("ada")}); err != nil {
		t.Fatal(err)
	}

	if _, err := store.UpdateProfile(ctx, bola.ID, ProfileUpdate{Handle: ptr("ada")}); !errors.Is(err, ErrHandleTaken) {
		t.Errorf("error = %v, want ErrHandleTaken", err)
	}
}

func TestDeleteUserEndsTheirSessions(t *testing.T) {
	store := NewStore(testdb.New(t))
	ctx := context.Background()
	token, user, _, _ := store.SignIn(ctx, ProviderApple, "sub-1", "", "Ada")

	if err := store.DeleteUser(ctx, user.ID); err != nil {
		t.Fatal(err)
	}

	if _, err := store.UserForToken(ctx, token); !errors.Is(err, ErrNoSession) {
		t.Errorf("token after deletion: error = %v, want ErrNoSession", err)
	}
	// Signing in again with the same identity starts a new account.
	_, again, created, err := store.SignIn(ctx, ProviderApple, "sub-1", "", "Ada")
	if err != nil || !created || again.ID == user.ID {
		t.Errorf("sign-in after deletion: created = %v, user = %+v, err = %v", created, again, err)
	}
	if err := store.DeleteUser(ctx, user.ID); !errors.Is(err, ErrNoUser) {
		t.Errorf("deleting twice: error = %v, want ErrNoUser", err)
	}
}
