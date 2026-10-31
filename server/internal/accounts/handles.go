package accounts

import (
	"context"
	"fmt"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

// An ID is what others find a user by, so it stays short and easy to say:
// lower-case letters, digits, dots and underscores.
var handlePattern = regexp.MustCompile(`^[a-z0-9_.]{3,20}$`)

const maxHandleLen = 20

// ValidHandle reports whether handle, already in lower case, may be an ID.
func ValidHandle(handle string) bool {
	return handlePattern.MatchString(handle)
}

// HandleFree reports whether userID may take handle: no one has it, or they
// already do.
func (s *Store) HandleFree(ctx context.Context, userID, handle string) (bool, error) {
	var taken bool
	err := s.db.QueryRow(ctx,
		`SELECT EXISTS (SELECT 1 FROM users WHERE handle = $1 AND id <> $2)`, handle, userID,
	).Scan(&taken)
	if err != nil {
		return false, fmt.Errorf("checking a handle: %w", err)
	}
	return !taken, nil
}

// SuggestHandles returns up to n IDs close to handle that no one has, made
// from it and the person's name.
func (s *Store) SuggestHandles(ctx context.Context, handle, name string, n int) ([]string, error) {
	candidates := handleCandidates(handle, name, time.Now().Year())
	rows, err := s.db.Query(ctx, `SELECT handle FROM users WHERE handle = ANY($1)`, candidates)
	if err != nil {
		return nil, fmt.Errorf("checking suggested handles: %w", err)
	}
	taken, err := pgx.CollectRows(rows, pgx.RowTo[string])
	if err != nil {
		return nil, fmt.Errorf("checking suggested handles: %w", err)
	}

	free := make([]string, 0, n)
	for _, c := range candidates {
		if len(free) == n {
			break
		}
		if !slices.Contains(taken, c) {
			free = append(free, c)
		}
	}
	return free, nil
}

// UserByHandle returns the user with this ID, or ErrNoUser. Someone who has
// not yet said which language they speak is not found: nobody could be heard
// in it.
func (s *Store) UserByHandle(ctx context.Context, handle string) (User, error) {
	return scanUser(s.db.QueryRow(ctx, selectUser+` WHERE handle = $1 AND language <> ''`, handle))
}

// handleCandidates lists IDs to suggest in place of handle, best first: the
// initial of the person's last name, the whole of it, the year, then digits.
// Each is a valid ID different from handle.
func handleCandidates(handle, name string, year int) []string {
	var candidates []string
	add := func(suffix string) {
		base := handle
		if len(base)+len(suffix) > maxHandleLen {
			base = base[:maxHandleLen-len(suffix)]
		}
		c := base + suffix
		if c != handle && ValidHandle(c) && !slices.Contains(candidates, c) {
			candidates = append(candidates, c)
		}
	}

	// Only the parts of the name an ID can hold: "Rosa López" gives "rosa"
	// alone, since "lópez" has a letter outside a to z.
	var words []string
	for _, w := range strings.Fields(strings.ToLower(name)) {
		if handlePattern.MatchString(w + "xx") { // padded, so short words pass the length rule
			words = append(words, w)
		}
	}
	if len(words) >= 2 {
		last := words[len(words)-1]
		add("." + last[:1])
		add("_" + last)
	}
	yy := fmt.Sprintf("%02d", year%100)
	add(yy)
	add("_" + yy)
	for i := 1; i <= 9; i++ {
		add(strconv.Itoa(i))
	}
	return candidates
}
