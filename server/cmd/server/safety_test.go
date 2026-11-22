package main

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/PelumiWeb/translator-app-go/server/internal/accounts"
	"github.com/PelumiWeb/translator-app-go/server/internal/api"
	"github.com/PelumiWeb/translator-app-go/server/internal/queue"
	"github.com/PelumiWeb/translator-app-go/server/internal/rooms"
	"github.com/PelumiWeb/translator-app-go/server/internal/storage"
	"github.com/PelumiWeb/translator-app-go/server/internal/testdb"
)

func TestWriteReports(t *testing.T) {
	made := time.Date(2026, 11, 22, 15, 5, 0, 0, time.FixedZone("WAT", 3600))
	tests := []struct {
		name    string
		reports []rooms.Report
		want    []string
	}{
		{"none", nil, []string{"no open reports"}},
		{
			name: "a message, its sender since deleted",
			reports: []rooms.Report{{
				ID: "r1", Reporter: "@ada Ada", Reported: "@rosa.m Rosa", Reason: "harassment", Note: "every day", CreatedAt: made,
				Message: &rooms.ReportedMessage{Kind: "voice", Lang: "es", Transcript: "tú otra vez", Translation: "you again", AudioKey: "k.wav"},
			}},
			want: []string{
				"REPORT r1 · 2026-11-22 14:05 UTC · harassment",
				`  about:  @rosa.m Rosa (account since deleted)`,
				`  by:     @ada Ada`,
				`  note:   "every day"`,
				`  said:   "tú otra vez" (voice, es)`,
				`  heard:  "you again"`,
				`  audio:  reports/k.wav`,
				`  close:  make report-close ID=r1 NOTE="what you did"`,
			},
		},
		{
			name:    "a person, by someone since deleted",
			reports: []rooms.Report{{ID: "r2", ReportedID: "u2", Reported: "@kenji", Reason: "spam", CreatedAt: made}},
			want: []string{
				"REPORT r2 · 2026-11-22 14:05 UTC · spam",
				"  about:  @kenji",
				"  by:     an account since deleted",
				`  close:  make report-close ID=r2 NOTE="what you did"`,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var out strings.Builder
			if err := writeReports(&out, tt.reports); err != nil {
				t.Fatal(err)
			}
			got := strings.Split(strings.TrimRight(out.String(), "\n"), "\n")
			if strings.Join(got, "\n") != strings.Join(tt.want, "\n") {
				t.Errorf("got:\n%s\nwant:\n%s", strings.Join(got, "\n"), strings.Join(tt.want, "\n"))
			}
		})
	}
}

func TestDeletingAnAccountAsksFirst(t *testing.T) {
	db := testdb.New(t)
	ctx := context.Background()
	accountStore := accounts.NewStore(db)
	roomStore := rooms.NewStore(db, queue.NewStore(db), rooms.Limits{})
	files := func() *storage.Local {
		l, err := storage.NewLocal(t.TempDir())
		if err != nil {
			t.Fatal(err)
		}
		return l
	}
	stores := safetyStores{
		accounts: accountStore, rooms: roomStore, reports: files(),
		api: &api.API{
			Logger:   slog.New(slog.NewTextHandler(io.Discard, nil)),
			Accounts: accountStore, Rooms: roomStore, Voices: files(), Messages: files(),
		},
	}
	_, user, _, err := accountStore.SignIn(ctx, accounts.ProviderApple, "sub-1", "", "Rosa")
	if err != nil {
		t.Fatal(err)
	}
	handle := "rosa.m"
	if _, err := accountStore.UpdateProfile(ctx, user.ID, accounts.ProfileUpdate{Handle: &handle}); err != nil {
		t.Fatal(err)
	}

	var out strings.Builder
	if err := (safetyCommand{deleteUser: "@rosa.m"}).run(ctx, stores, &out); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "make account-delete HANDLE=rosa.m YES=1") {
		t.Errorf("without -yes it said %q", out.String())
	}
	if _, err := accountStore.FindAccount(ctx, "rosa.m"); err != nil {
		t.Fatalf("the account should still be there: %v", err)
	}

	out.Reset()
	if err := (safetyCommand{deleteUser: "rosa.m", yes: true}).run(ctx, stores, &out); err != nil {
		t.Fatal(err)
	}
	if _, err := accountStore.FindAccount(ctx, "rosa.m"); !errors.Is(err, accounts.ErrNoUser) {
		t.Errorf("after -yes: err = %v, want the account gone", err)
	}
	if err := (safetyCommand{suspend: "rosa.m"}).run(ctx, stores, &out); err == nil || !strings.Contains(err.Error(), "no account") {
		t.Errorf("acting on an ID no one has: err = %v", err)
	}
}
