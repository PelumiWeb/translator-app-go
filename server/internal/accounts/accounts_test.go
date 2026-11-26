package accounts

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

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

func TestASuspendedAccountIsSignedOutAndStaysOut(t *testing.T) {
	store := NewStore(testdb.New(t))
	ctx := context.Background()
	token, user, _, err := store.SignIn(ctx, ProviderApple, "sub-1", "", "Ada")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.UpdateProfile(ctx, user.ID, ProfileUpdate{Handle: ptr("ada"), Language: ptr("en")}); err != nil {
		t.Fatal(err)
	}

	for range 2 { // suspending twice changes nothing
		if err := store.Suspend(ctx, user.ID); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := store.UserForToken(ctx, token); !errors.Is(err, ErrNoSession) {
		t.Errorf("the old session: err = %v, want ErrNoSession", err)
	}
	if _, _, _, err := store.SignIn(ctx, ProviderApple, "sub-1", "", "Ada"); !errors.Is(err, ErrSuspended) {
		t.Errorf("signing in again: err = %v, want ErrSuspended", err)
	}
	var sessions int
	if err := store.db.QueryRow(ctx, `SELECT count(*) FROM sessions WHERE user_id = $1`, user.ID).Scan(&sessions); err != nil || sessions != 0 {
		t.Errorf("sessions = %d (%v), want none, not even from the refused sign-in", sessions, err)
	}
	if _, err := store.UserByHandle(ctx, "ada"); !errors.Is(err, ErrNoUser) {
		t.Errorf("finding them by ID: err = %v, want ErrNoUser", err)
	}
	if found, err := store.FindAccount(ctx, "ADA"); err != nil || found.ID != user.ID {
		t.Errorf("the owner's lookup: %+v, %v; want the account", found, err)
	}
	if err := store.Suspend(ctx, "00000000-0000-0000-0000-000000000000"); !errors.Is(err, ErrNoUser) {
		t.Errorf("suspending no one: err = %v, want ErrNoUser", err)
	}
}

func TestAppleRefreshTokensAreKeptForTheirIdentity(t *testing.T) {
	store := NewStore(testdb.New(t))
	ctx := context.Background()
	_, ada, _, err := store.SignIn(ctx, ProviderApple, "apple-ada", "", "Ada")
	if err != nil {
		t.Fatal(err)
	}
	if tokens, err := store.AppleRefreshTokens(ctx, ada.ID); err != nil || len(tokens) != 0 {
		t.Errorf("before: %v, %v; want none", tokens, err)
	}
	if err := store.SetAppleRefreshToken(ctx, "apple-ada", "r-1"); err != nil {
		t.Fatal(err)
	}
	// Signing in again with a new code replaces it.
	if err := store.SetAppleRefreshToken(ctx, "apple-ada", "r-2"); err != nil {
		t.Fatal(err)
	}
	tokens, err := store.AppleRefreshTokens(ctx, ada.ID)
	if err != nil || len(tokens) != 1 || tokens[0] != "r-2" {
		t.Errorf("tokens = %v, %v; want r-2", tokens, err)
	}
}

func TestAgreeingToTheServicesIsRecordedWithItsWording(t *testing.T) {
	store := NewStore(testdb.New(t))
	ctx := context.Background()
	token, ada, _, err := store.SignIn(ctx, ProviderApple, "sub-1", "", "Ada")
	if err != nil {
		t.Fatal(err)
	}
	if ada.ServicesConsent != "" {
		t.Errorf("a new account has agreed to %q", ada.ServicesConsent)
	}
	updated, err := store.UpdateProfile(ctx, ada.ID, ProfileUpdate{ServicesConsent: ptr("2026-11-23")})
	if err != nil || updated.ServicesConsent != "2026-11-23" {
		t.Fatalf("after agreeing: %+v, %v", updated, err)
	}
	// Another change leaves it, and when it was given, as they were.
	if _, err := store.UpdateProfile(ctx, ada.ID, ProfileUpdate{DisplayName: ptr("Ada L")}); err != nil {
		t.Fatal(err)
	}
	var when *time.Time
	if err := store.db.QueryRow(ctx, `SELECT services_consented_at FROM users WHERE id = $1`, ada.ID).Scan(&when); err != nil || when == nil {
		t.Errorf("services_consented_at = %v, %v", when, err)
	}
	if got, err := store.UserForToken(ctx, token); err != nil || got.ServicesConsent != "2026-11-23" {
		t.Errorf("as the session reads it: %+v, %v", got, err)
	}
}
