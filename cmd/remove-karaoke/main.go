// Command remove-karaoke removes registered karaoke objects before migration 0010.
// Run only while all application instances are stopped, with the normal environment.
package main

import (
	"context"
	"flag"
	"fmt"
	"log"

	"play-music/internal/config"
	"play-music/internal/db"
	"play-music/internal/storage"
)

func main() {
	apply := flag.Bool("apply", false, "delete listed karaoke files and records; stop all app instances first")
	flag.Parse()
	if err := run(context.Background(), *apply); err != nil {
		log.Fatal(err)
	}
}

func run(ctx context.Context, apply bool) error {
	cfg := config.Load()
	pool, err := db.Connect(ctx, cfg.DatabaseURL)
	if err != nil {
		return fmt.Errorf("connect database: %w", err)
	}
	defer pool.Close()
	bucket, err := storage.New(cfg.S3)
	if err != nil {
		return fmt.Errorf("connect storage: %w", err)
	}
	tx, err := pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	// Prevent changes to the catalog until every deletion is checked. All app
	// instances must still be stopped, so writes cannot resume after commit.
	if _, err = tx.Exec(ctx, "LOCK TABLE karaokes, songs IN ACCESS EXCLUSIVE MODE"); err != nil {
		return err
	}
	rows, err := tx.Query(ctx, `SELECT k.path, EXISTS(SELECT 1 FROM songs s WHERE s.path=k.path) FROM karaokes k ORDER BY k.path`)
	if err != nil {
		return err
	}
	var keys []string
	for rows.Next() {
		var key string
		var usedBySong bool
		if err := rows.Scan(&key, &usedBySong); err != nil {
			rows.Close()
			return err
		}
		if err := validateKey(key, usedBySong); err != nil {
			rows.Close()
			return err
		}
		keys = append(keys, key)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return err
	}
	for _, key := range keys {
		fmt.Println(key)
	}
	fmt.Printf("%d registered karaoke objects in bucket %s\n", len(keys), bucket.Bucket())
	if !apply {
		fmt.Println("Preview only; no data removed.")
		return nil
	}
	if err := removeObjects(ctx, bucket, keys); err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, `DELETE FROM artworks WHERE entity_type IN ('karaoke','karaoke_thumb')`); err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, `DELETE FROM karaokes`); err != nil {
		return err
	}
	if err = tx.Commit(ctx); err != nil {
		return err
	}
	fmt.Println("Objects verified absent; karaoke records and artwork removed.")
	return nil
}

// Validate the whole catalog before deleting any object.
func validateKey(key string, usedBySong bool) error {
	if key == "" || usedBySong {
		return fmt.Errorf("unsafe or song-owned path %q", key)
	}
	return nil
}

type objectStore interface {
	Remove(context.Context, string) error
	ObjectExists(context.Context, string) (bool, error)
}

func removeObjects(ctx context.Context, bucket objectStore, keys []string) error {
	for _, key := range keys {
		if err := bucket.Remove(ctx, key); err != nil {
			return fmt.Errorf("remove %q: %w", key, err)
		}
		exists, err := bucket.ObjectExists(ctx, key)
		if err != nil {
			return fmt.Errorf("verify %q: %w", key, err)
		}
		if exists {
			return fmt.Errorf("object still exists: %q", key)
		}
	}
	return nil
}
