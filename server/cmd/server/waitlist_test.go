package main

import (
	"strings"
	"testing"
	"time"

	"github.com/PelumiWeb/translator-app-go/server/internal/waitlist"
)

func TestWriteList(t *testing.T) {
	joined := time.Date(2026, 11, 19, 13, 5, 0, 0, time.FixedZone("WAT", 3600))
	invited := time.Date(2026, 11, 20, 9, 30, 0, 0, time.UTC)
	tests := []struct {
		name    string
		entries []waitlist.Entry
		want    []string // the lines, with runs of spaces made one
	}{
		{
			name: "nobody yet",
			want: []string{"nobody on the list yet"},
		},
		{
			name: "waiting and invited, times in UTC",
			entries: []waitlist.Entry{
				{Email: "ada@example.com", Name: "Ada", Phone: waitlist.PhoneIPhone, CreatedAt: joined, InvitedAt: &invited},
				{Email: "kenji@example.com", Phone: waitlist.PhoneAndroid, CreatedAt: joined.Add(time.Hour)},
			},
			want: []string{
				"JOINED (UTC) PHONE EMAIL NAME INVITED (UTC)",
				"2026-11-19 12:05 iphone ada@example.com Ada 2026-11-20 09:30",
				"2026-11-19 13:05 android kenji@example.com waiting",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var out strings.Builder
			if err := writeList(&out, tt.entries); err != nil {
				t.Fatal(err)
			}
			lines := strings.Split(strings.TrimRight(out.String(), "\n"), "\n")
			if len(lines) != len(tt.want) {
				t.Fatalf("got %d lines, want %d:\n%s", len(lines), len(tt.want), out.String())
			}
			for i, line := range lines {
				if got := strings.Join(strings.Fields(line), " "); got != tt.want[i] {
					t.Errorf("line %d = %q, want %q", i, got, tt.want[i])
				}
			}
		})
	}
}
