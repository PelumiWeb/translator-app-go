package waitlist

import (
	"context"
	"fmt"
	"slices"
	"sync"
	"testing"

	"github.com/PelumiWeb/translator-app-go/server/internal/testdb"
)

func emails(entries []Entry) []string {
	var out []string
	for _, e := range entries {
		out = append(out, e.Email)
	}
	return out
}

func TestPeopleAreInvitedInTheOrderTheyJoined(t *testing.T) {
	s := NewStore(testdb.New(t))
	ctx := context.Background()
	for _, e := range []Entry{
		{Email: "ada@example.com", Name: "Ada", Phone: PhoneIPhone},
		{Email: "kenji@example.com", Phone: PhoneAndroid},
		{Email: "rosa@example.com", Phone: PhoneIPhone},
		{Email: "tunde@example.com", Phone: PhoneIPhone},
	} {
		if err := s.Join(ctx, e); err != nil {
			t.Fatal(err)
		}
	}

	first, err := s.Invite(ctx, PhoneIPhone, 2)
	if err != nil {
		t.Fatal(err)
	}
	if got := emails(first); !slices.Equal(got, []string{"ada@example.com", "rosa@example.com"}) {
		t.Errorf("first batch = %v", got)
	}
	if first[0].Name != "Ada" || first[1].Name != "" {
		t.Errorf("names = %q, %q", first[0].Name, first[1].Name)
	}
	rest, err := s.Invite(ctx, PhoneIPhone, 10)
	if err != nil {
		t.Fatal(err)
	}
	if got := emails(rest); !slices.Equal(got, []string{"tunde@example.com"}) {
		t.Errorf("second batch = %v, want only who was left", got)
	}

	waiting, err := s.Waiting(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if waiting[PhoneIPhone] != 0 || waiting[PhoneAndroid] != 1 {
		t.Errorf("waiting = %v, want no iPhones and one Android", waiting)
	}
}

func TestJoiningAgainKeepsThePlace(t *testing.T) {
	s := NewStore(testdb.New(t))
	ctx := context.Background()
	steps := []Entry{
		{Email: "ada@example.com", Name: "Ada", Phone: PhoneAndroid},
		{Email: "rosa@example.com", Phone: PhoneIPhone},
		// The same address, written differently, with another phone and no name.
		{Email: "ADA@Example.com", Phone: PhoneIPhone},
	}
	for _, e := range steps {
		if err := s.Join(ctx, e); err != nil {
			t.Fatal(err)
		}
	}
	invited, err := s.Invite(ctx, PhoneIPhone, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(invited) != 2 || invited[0].Email != "ada@example.com" || invited[0].Name != "Ada" {
		t.Errorf("invited = %+v, want Ada first, once, with her name", invited)
	}
}

func TestTwoRunsAtOnceNeverInviteTheSamePerson(t *testing.T) {
	s := NewStore(testdb.New(t))
	ctx := context.Background()
	for i := range 20 {
		if err := s.Join(ctx, Entry{Email: fmt.Sprintf("p%d@example.com", i), Phone: PhoneIPhone}); err != nil {
			t.Fatal(err)
		}
	}
	var (
		wg   sync.WaitGroup
		mu   sync.Mutex
		seen = map[string]int{}
	)
	for range 4 {
		wg.Go(func() {
			batch, err := s.Invite(ctx, PhoneIPhone, 5)
			if err != nil {
				t.Error(err)
				return
			}
			mu.Lock()
			defer mu.Unlock()
			for _, e := range batch {
				seen[e.Email]++
			}
		})
	}
	wg.Wait()
	// A run that lost a race may take fewer than it asked for; one more
	// gathers whoever is left.
	rest, err := s.Invite(ctx, PhoneIPhone, 20)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range rest {
		seen[e.Email]++
	}
	for email, n := range seen {
		if n > 1 {
			t.Errorf("%s invited %d times", email, n)
		}
	}
	if len(seen) != 20 {
		t.Errorf("%d invited, want all 20", len(seen))
	}
}

func TestRemoveForgetsSomeone(t *testing.T) {
	s := NewStore(testdb.New(t))
	ctx := context.Background()
	if err := s.Join(ctx, Entry{Email: "ada@example.com", Phone: PhoneIPhone}); err != nil {
		t.Fatal(err)
	}
	if removed, err := s.Remove(ctx, "Ada@example.com"); err != nil || !removed {
		t.Fatalf("Remove = %v, %v", removed, err)
	}
	if removed, err := s.Remove(ctx, "ada@example.com"); err != nil || removed {
		t.Errorf("removing again = %v, %v; want false", removed, err)
	}
	if invited, _ := s.Invite(ctx, PhoneIPhone, 10); len(invited) != 0 {
		t.Errorf("invited after removal: %v", emails(invited))
	}
}

func TestListShowsEveryoneAndChangesNothing(t *testing.T) {
	s := NewStore(testdb.New(t))
	ctx := context.Background()
	for _, e := range []Entry{
		{Email: "ada@example.com", Name: "Ada", Phone: PhoneIPhone},
		{Email: "kenji@example.com", Phone: PhoneAndroid},
		{Email: "rosa@example.com", Phone: PhoneIPhone},
	} {
		if err := s.Join(ctx, e); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := s.Invite(ctx, PhoneIPhone, 1); err != nil {
		t.Fatal(err)
	}

	listed, err := s.List(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if got := emails(listed); !slices.Equal(got, []string{"ada@example.com", "kenji@example.com", "rosa@example.com"}) {
		t.Fatalf("listed = %v, want everyone in the order they joined", got)
	}
	if listed[0].InvitedAt == nil || listed[1].InvitedAt != nil || listed[2].InvitedAt != nil {
		t.Errorf("invited = %v, %v, %v; want only Ada", listed[0].InvitedAt, listed[1].InvitedAt, listed[2].InvitedAt)
	}
	if listed[0].Name != "Ada" || listed[1].Phone != PhoneAndroid {
		t.Errorf("first two = %+v, %+v", listed[0], listed[1])
	}

	waiting, err := s.Waiting(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if waiting[PhoneIPhone] != 1 || waiting[PhoneAndroid] != 1 {
		t.Errorf("waiting after listing = %v, want one of each still waiting", waiting)
	}
}
