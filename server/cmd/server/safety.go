package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/PelumiWeb/translator-app-go/server/internal/accounts"
	"github.com/PelumiWeb/translator-app-go/server/internal/api"
	"github.com/PelumiWeb/translator-app-go/server/internal/rooms"
)

// safetyCommand is a one-off job on reports and accounts, run instead of the
// server (ADR 0035). Only one of its actions is set.
type safetyCommand struct {
	listReports bool
	closeReport string // a report's id
	note        string // with closeReport: what was done
	suspend     string // an account's handle
	deleteUser  string // an account's handle
	yes         bool   // with deleteUser: really delete it
}

func (c safetyCommand) set() bool {
	return c.listReports || c.closeReport != "" || c.suspend != "" || c.deleteUser != ""
}

// safetyStores is what the commands work with.
type safetyStores struct {
	accounts *accounts.Store
	rooms    *rooms.Store
	reports  api.AudioStore // copies of reported recordings
	// api deletes an account the way deleting it in the app does.
	api *api.API
}

func (c safetyCommand) run(ctx context.Context, s safetyStores, out io.Writer) error {
	switch {
	case c.listReports:
		reports, err := s.rooms.OpenReports(ctx)
		if err != nil {
			return err
		}
		return writeReports(out, reports)

	case c.closeReport != "":
		key, err := s.rooms.CloseReport(ctx, c.closeReport, c.note)
		if errors.Is(err, rooms.ErrNoReport) {
			return fmt.Errorf("no open report %s", c.closeReport)
		}
		if err != nil {
			return err
		}
		if key != "" {
			if err := s.reports.Delete(key); err != nil {
				fmt.Fprintf(out, "the copy of the recording, %s, could not be deleted: %v\n", key, err)
			}
		}
		_, err = fmt.Fprintf(out, "closed %s; its copy of the message is deleted\n", c.closeReport)
		return err

	case c.suspend != "":
		user, err := findAccount(ctx, s.accounts, c.suspend)
		if err != nil {
			return err
		}
		if err := s.accounts.Suspend(ctx, user.ID); err != nil {
			return err
		}
		_, err = fmt.Fprintf(out, "suspended %s: signed out everywhere, and cannot sign in or be found\n", who(user))
		return err

	default: // deleteUser
		user, err := findAccount(ctx, s.accounts, c.deleteUser)
		if err != nil {
			return err
		}
		if !c.yes {
			_, err = fmt.Fprintf(out, "this would delete %s, their voice and everything they said in their rooms, for good.\n"+
				"to do it: make account-delete HANDLE=%s YES=1\n", who(user), user.Handle)
			return err
		}
		if err := s.api.DeleteAccount(ctx, user); err != nil {
			return err
		}
		_, err = fmt.Fprintf(out, "deleted %s\n", who(user))
		return err
	}
}

func findAccount(ctx context.Context, store *accounts.Store, handle string) (accounts.User, error) {
	user, err := store.FindAccount(ctx, strings.TrimPrefix(strings.TrimSpace(handle), "@"))
	if errors.Is(err, accounts.ErrNoUser) {
		return accounts.User{}, fmt.Errorf("no account has the ID %q", handle)
	}
	return user, err
}

func who(u accounts.User) string {
	return strings.TrimSpace("@" + u.Handle + " " + u.DisplayName)
}

// writeReports prints the open reports, oldest first, for a terminal. Times
// are in UTC, the machine's clock.
func writeReports(out io.Writer, reports []rooms.Report) error {
	if len(reports) == 0 {
		_, err := fmt.Fprintln(out, "no open reports")
		return err
	}
	var b strings.Builder
	for _, r := range reports {
		fmt.Fprintf(&b, "REPORT %s · %s UTC · %s\n", r.ID, r.CreatedAt.UTC().Format("2006-01-02 15:04"), r.Reason)
		about := r.Reported
		if r.ReportedID == "" {
			about += " (account since deleted)"
		}
		fmt.Fprintf(&b, "  about:  %s\n", about)
		by := r.Reporter
		if by == "" {
			by = "an account since deleted"
		}
		fmt.Fprintf(&b, "  by:     %s\n", by)
		if r.Note != "" {
			fmt.Fprintf(&b, "  note:   %q\n", r.Note)
		}
		if m := r.Message; m != nil {
			fmt.Fprintf(&b, "  said:   %q (%s, %s)\n", m.Transcript, m.Kind, m.Lang)
			if m.Translation != "" {
				fmt.Fprintf(&b, "  heard:  %q\n", m.Translation)
			}
			if m.AudioKey != "" {
				fmt.Fprintf(&b, "  audio:  reports/%s\n", m.AudioKey)
			}
		}
		fmt.Fprintf(&b, "  close:  make report-close ID=%s NOTE=\"what you did\"\n\n", r.ID)
	}
	_, err := io.WriteString(out, b.String())
	return err
}
