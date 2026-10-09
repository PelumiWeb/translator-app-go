package migrations_test

import (
	"context"
	"testing"

	"github.com/PelumiWeb/translator-app-go/server/internal/testdb"
	"github.com/PelumiWeb/translator-app-go/server/migrations"
)

func TestApplyCreatesTables(t *testing.T) {
	pool := testdb.New(t) // already applies the migrations once
	ctx := context.Background()

	for _, table := range []string{"jobs", "job_events", "schema_migrations"} {
		var exists bool
		// to_regclass returns NULL when the name does not resolve to a table.
		err := pool.QueryRow(ctx, "SELECT to_regclass($1) IS NOT NULL", table).Scan(&exists)
		if err != nil {
			t.Fatalf("checking table %s: %v", table, err)
		}
		if !exists {
			t.Errorf("table %s was not created", table)
		}
	}
}

func TestApplyIsIdempotent(t *testing.T) {
	pool := testdb.New(t)
	ctx := context.Background()

	if err := migrations.Apply(ctx, pool); err != nil {
		t.Fatalf("second Apply: %v", err)
	}

	var count int
	if err := pool.QueryRow(ctx, "SELECT count(*) FROM schema_migrations").Scan(&count); err != nil {
		t.Fatalf("counting applied migrations: %v", err)
	}
	if count != 3 {
		t.Errorf("schema_migrations has %d rows, want 3", count)
	}
}
