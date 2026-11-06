package db

import (
	"context"
	"os"
	"testing"
)

func TestRemoveKaraokeMigrationRepeatable(t *testing.T) {
	url := os.Getenv("PM_TEST_DATABASE_URL")
	if url == "" {
		t.Skip("PM_TEST_DATABASE_URL not configured")
	}
	ctx := context.Background()
	pool, err := Connect(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(ctx)
	for _, q := range []string{
		`CREATE TEMP TABLE artworks(entity_type text)`,
		`CREATE TEMP TABLE karaokes(id text)`,
		`CREATE TEMP TABLE category_karaokes(id text)`,
		`SET LOCAL search_path = pg_temp`,
		`INSERT INTO karaokes VALUES ('video')`,
		`SAVEPOINT before_guard`,
	} {
		if _, err := tx.Exec(ctx, q); err != nil {
			t.Fatal(err)
		}
	}
	migration, err := migrationsFS.ReadFile("migrations/0010_remove_karaokes.sql")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = tx.Exec(ctx, string(migration)); err == nil {
		t.Fatal("migration accepted remaining records")
	}
	if _, err = tx.Exec(ctx, "ROLLBACK TO SAVEPOINT before_guard"); err != nil {
		t.Fatal(err)
	}
	if _, err = tx.Exec(ctx, "DELETE FROM karaokes"); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 2; i++ {
		if _, err = tx.Exec(ctx, string(migration)); err != nil {
			t.Fatalf("run %d: %v", i+1, err)
		}
	}
}
