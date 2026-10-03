// Package migrations holds the SQL schema and applies it at startup.
package migrations

import (
	"context"
	"embed"
	"fmt"
	"io/fs"
	"sort"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// The SQL files are compiled into the binary, so deploying the server is one
// file with no migrations directory to ship next to it. go:embed can only
// reach files in or below the package directory, which is why this Go file
// lives beside the SQL.
//
//go:embed *.sql
var files embed.FS

// lockKey is an arbitrary number that identifies "the migration lock" to
// Postgres. Any process that asks for the same key waits its turn.
const lockKey = 7_203_114

// Apply runs every migration that has not been applied yet, in file name
// order. It is safe to call on every start and from several processes at once.
func Apply(ctx context.Context, pool *pgxpool.Pool) error {
	names, err := fs.Glob(files, "*.sql")
	if err != nil {
		return fmt.Errorf("listing migrations: %w", err)
	}
	sort.Strings(names)

	// One transaction for the whole run: either every pending migration is
	// applied or none is. Postgres can roll back CREATE TABLE, unlike MySQL.
	tx, err := pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("starting migration transaction: %w", err)
	}
	// Rollback after a successful Commit does nothing, so deferring it
	// covers every early return without tracking state.
	defer tx.Rollback(ctx)

	// Released automatically when the transaction ends.
	if _, err := tx.Exec(ctx, "SELECT pg_advisory_xact_lock($1)", lockKey); err != nil {
		return fmt.Errorf("taking migration lock: %w", err)
	}

	_, err = tx.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS schema_migrations (
			name       text PRIMARY KEY,
			applied_at timestamptz NOT NULL DEFAULT now()
		)`)
	if err != nil {
		return fmt.Errorf("creating schema_migrations: %w", err)
	}

	applied, err := appliedNames(ctx, tx)
	if err != nil {
		return err
	}

	for _, name := range names {
		if applied[name] {
			continue
		}
		sql, err := files.ReadFile(name)
		if err != nil {
			return fmt.Errorf("reading %s: %w", name, err)
		}
		if _, err := tx.Exec(ctx, string(sql)); err != nil {
			return fmt.Errorf("applying %s: %w", name, err)
		}
		if _, err := tx.Exec(ctx, "INSERT INTO schema_migrations (name) VALUES ($1)", name); err != nil {
			return fmt.Errorf("recording %s: %w", name, err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("committing migrations: %w", err)
	}
	return nil
}

func appliedNames(ctx context.Context, tx pgx.Tx) (map[string]bool, error) {
	rows, err := tx.Query(ctx, "SELECT name FROM schema_migrations")
	if err != nil {
		return nil, fmt.Errorf("reading schema_migrations: %w", err)
	}
	// CollectRows closes rows and returns any error met while iterating.
	names, err := pgx.CollectRows(rows, pgx.RowTo[string])
	if err != nil {
		return nil, fmt.Errorf("reading schema_migrations: %w", err)
	}

	applied := make(map[string]bool, len(names))
	for _, name := range names {
		applied[name] = true
	}
	return applied, nil
}
