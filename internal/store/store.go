package store

import (
	"context"
	"crypto/rand"
	"encoding/hex"

	"github.com/jackc/pgx/v5/pgxpool"
)

type Store struct {
	pool *pgxpool.Pool
}

func New(pool *pgxpool.Pool) *Store {
	return &Store{pool: pool}
}

func (s *Store) Pool() *pgxpool.Pool { return s.pool }

// newID returns a random hex id (32 chars), no external UUID dependency.
func newID() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		panic("crypto/rand failure: " + err.Error())
	}
	return hex.EncodeToString(b)
}

// NewID returns a fresh random id (exported for the scanner).
func NewID() string { return newID() }

// ---------- settings ----------

func (s *Store) GetSetting(ctx context.Context, key string) (string, bool, error) {
	var v string
	err := s.pool.QueryRow(ctx, "SELECT value FROM settings WHERE key=$1", key).Scan(&v)
	if err != nil {
		if err.Error() == "no rows in result set" {
			return "", false, nil
		}
		return "", false, err
	}
	return v, true, nil
}

func (s *Store) SetSetting(ctx context.Context, key, value string) error {
	_, err := s.pool.Exec(ctx,
		`INSERT INTO settings(key, value) VALUES($1, $2)
		 ON CONFLICT (key) DO UPDATE SET value=EXCLUDED.value`, key, value)
	return err
}

// GetOrCreateSecret returns the persistent JWT signing secret, generating and
// storing it on first boot so tokens survive restarts.
func (s *Store) GetOrCreateSecret(ctx context.Context) ([]byte, error) {
	if v, ok, err := s.GetSetting(ctx, "jwt_secret"); err == nil && ok && v != "" {
		return []byte(v), nil
	}
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return nil, err
	}
	secret := hex.EncodeToString(b)
	if err := s.SetSetting(ctx, "jwt_secret", secret); err != nil {
		return nil, err
	}
	return []byte(secret), nil
}

// ---------- artwork ----------

type Art struct {
	Data []byte
	Mime string
}

func (s *Store) GetArt(ctx context.Context, entityType, entityID string) (*Art, bool, error) {
	var a Art
	err := s.pool.QueryRow(ctx,
		"SELECT data, mime FROM artworks WHERE entity_type=$1 AND entity_id=$2",
		entityType, entityID).Scan(&a.Data, &a.Mime)
	if err != nil {
		if err.Error() == "no rows in result set" {
			return nil, false, nil
		}
		return nil, false, err
	}
	return &a, true, nil
}

func (s *Store) UpsertArt(ctx context.Context, entityType, entityID string, data []byte, mime string) error {
	_, err := s.pool.Exec(ctx,
		`INSERT INTO artworks(entity_type, entity_id, data, mime, updated_at)
		 VALUES($1, $2, $3, $4, now())
		 ON CONFLICT (entity_type, entity_id) DO UPDATE
		   SET data=EXCLUDED.data, mime=EXCLUDED.mime, updated_at=now()`,
		entityType, entityID, data, mime)
	return err
}

func (s *Store) AlbumsWithoutArt(ctx context.Context) ([]string, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT a.id FROM albums a
		 WHERE NOT EXISTS (SELECT 1 FROM artworks w WHERE w.entity_type='album' AND w.entity_id=a.id)
		   AND EXISTS (SELECT 1 FROM songs s WHERE s.album_id=a.id)`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

func (s *Store) AlbumsWithCoverInFolder(ctx context.Context) ([]string, error) {
	return s.AlbumsWithoutArt(ctx)
}

// AlbumFolder returns the S3 folder (path prefix) of the first song of an album.
func (s *Store) AlbumFolder(ctx context.Context, albumID string) (string, error) {
	var path string
	err := s.pool.QueryRow(ctx,
		"SELECT path FROM songs WHERE album_id=$1 ORDER BY disc_number, track_number, title LIMIT 1",
		albumID).Scan(&path)
	if err != nil {
		return "", err
	}
	return folderOf(path), nil
}

func folderOf(path string) string {
	for i := len(path) - 1; i >= 0; i-- {
		if path[i] == '/' {
			return path[:i+1]
		}
	}
	return ""
}
