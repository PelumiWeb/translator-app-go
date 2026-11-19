package main

import (
	"context"
	"encoding/csv"
	"fmt"
	"io"

	"github.com/PelumiWeb/translator-app-go/server/internal/waitlist"
)

// waitlistCommand is a one-off job on the waitlist, run instead of the server
// (ADR 0030). Only one of its fields is set.
type waitlistCommand struct {
	invite int    // take this many people waiting with an iPhone
	remove string // take this address off the list
	count  bool   // say how many are waiting
}

func (c waitlistCommand) set() bool { return c.invite > 0 || c.remove != "" || c.count }

// run writes what the owner asked for to out. Invitations are written as
// CSV, one "first name,last name,email" line each with no header, the
// layout App Store Connect imports testers from.
func (c waitlistCommand) run(ctx context.Context, store *waitlist.Store, out io.Writer) error {
	switch {
	case c.invite > 0:
		// TestFlight reaches only iPhones; Android sign-ups wait for Android testing.
		entries, err := store.Invite(ctx, waitlist.PhoneIPhone, c.invite)
		if err != nil {
			return err
		}
		w := csv.NewWriter(out)
		for _, e := range entries {
			if err := w.Write([]string{e.Name, "", e.Email}); err != nil {
				return fmt.Errorf("writing the invitations: %w", err)
			}
		}
		w.Flush()
		return w.Error()
	case c.remove != "":
		removed, err := store.Remove(ctx, c.remove)
		if err != nil {
			return err
		}
		if !removed {
			_, err = fmt.Fprintf(out, "%s was not on the list\n", c.remove)
			return err
		}
		_, err = fmt.Fprintf(out, "removed %s\n", c.remove)
		return err
	default:
		counts, err := store.Waiting(ctx)
		if err != nil {
			return err
		}
		_, err = fmt.Fprintf(out, "waiting: %d iPhone, %d Android\n", counts[waitlist.PhoneIPhone], counts[waitlist.PhoneAndroid])
		return err
	}
}
