// Package testdb gives tests a real, empty Postgres schema to work in.
package testdb

import (
	"context"
	"fmt"
	"math/rand/v2"
	"os"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/PelumiWeb/translator-app-go/server/migrations"
)

const defaultURL = "postgres://localhost:5432/voice_translation_test?sslmode=disable"

// New returns a pool connected to a fresh schema with all migrations applied.
// The schema is dropped when the test ends.
//
// Each test gets its own schema instead of sharing tables and truncating
// them, because `go test ./...` runs packages in parallel and they would
// delete each other's rows.
func New(t *testing.T) *pgxpool.Pool {
	t.Helper()
	ctx := context.Background()

	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		url = defaultURL
	}

	cfg, err := pgxpool.ParseConfig(url)
	if err != nil {
		t.Fatalf("parsing TEST_DATABASE_URL: %v", err)
	}

	schema := fmt.Sprintf("test_%d", rand.Uint64())

	// A single short-lived connection creates the schema, before the pool
	// (whose connections all point at that schema) exists.
	admin, err := pgx.ConnectConfig(ctx, cfg.ConnConfig)
	if err != nil {
		t.Fatalf("connecting to test database (is Postgres running? try `make db-up`): %v", err)
	}
	defer admin.Close(ctx)
	if _, err := admin.Exec(ctx, "CREATE SCHEMA "+schema); err != nil {
		t.Fatalf("creating schema: %v", err)
	}

	// search_path makes unqualified names like "jobs" resolve to this schema
	// on every connection the pool opens.
	cfg.ConnConfig.RuntimeParams["search_path"] = schema
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		t.Fatalf("opening pool: %v", err)
	}

	// Cleanup runs even when the test fails, in reverse order of registration.
	t.Cleanup(func() {
		pool.Close()
		conn, err := pgx.ConnectConfig(ctx, cfg.ConnConfig)
		if err != nil {
			t.Logf("dropping schema %s: %v", schema, err)
			return
		}
		defer conn.Close(ctx)
		if _, err := conn.Exec(ctx, "DROP SCHEMA "+schema+" CASCADE"); err != nil {
			t.Logf("dropping schema %s: %v", schema, err)
		}
	})

	if err := migrations.Apply(ctx, pool); err != nil {
		t.Fatalf("applying migrations: %v", err)
	}
	return pool
}
