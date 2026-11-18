package store

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Store struct {
	pool *pgxpool.Pool
}

func New(pool *pgxpool.Pool) *Store {
	return &Store{pool: pool}
}

func NewID() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		panic("crypto/rand failure: " + err.Error())
	}
	return hex.EncodeToString(b)
}

func (s *Store) GetSetting(ctx context.Context, key string) (string, bool, error) {
	var v string
	err := s.pool.QueryRow(ctx, "SELECT value FROM settings WHERE key=$1", key).Scan(&v)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", false, nil
	}
	if err != nil {
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

// Art is a stored cover. The artworks table also has a mime column from the
// original schema; nothing reads it, because every cover is re-encoded to JPEG
// before it is served.
type Art struct {
	Data []byte
}

// The entity types that can own artwork or a like. These strings are the
// entity_type column values, so the store is where they are defined.
const (
	EntitySong     = "song"
	EntityAlbum    = "album"
	EntityArtist   = "artist"
	EntityCategory = "category"
)

// GetArt returns the stored cover bytes. The mime column is not selected: every
// cover is re-encoded to JPEG before serving, so it cannot affect a response.
func (s *Store) GetArt(ctx context.Context, entityType, entityID string) (*Art, bool, error) {
	var a Art
	err := s.pool.QueryRow(ctx,
		"SELECT data FROM artworks WHERE entity_type=$1 AND entity_id=$2",
		entityType, entityID).Scan(&a.Data)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	return &a, true, nil
}

// UpsertArt writes cover bytes. The mime column is left to its schema default:
// every cover is re-encoded to JPEG before it is served, so the value could
// never reach a response.
func (s *Store) UpsertArt(ctx context.Context, entityType, entityID string, data []byte) error {
	_, err := s.pool.Exec(ctx,
		`INSERT INTO artworks(entity_type, entity_id, data, updated_at)
		 VALUES($1, $2, $3, now())
		 ON CONFLICT (entity_type, entity_id) DO UPDATE
		   SET data=EXCLUDED.data, updated_at=now()`,
		entityType, entityID, data)
	return err
}

func (s *Store) DeleteArt(ctx context.Context, entityType, entityID string) error {
	_, err := s.pool.Exec(ctx,
		"DELETE FROM artworks WHERE entity_type=$1 AND entity_id=$2", entityType, entityID)
	return err
}

func (s *Store) AlbumsWithoutArt(ctx context.Context) ([]string, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT a.id FROM albums a
		 WHERE NOT EXISTS (SELECT 1 FROM artworks w WHERE w.entity_type='`+EntityAlbum+`' AND w.entity_id=a.id)
		   AND EXISTS (SELECT 1 FROM songs s WHERE s.album_id=a.id)`)
	if err != nil {
		return nil, err
	}
	return collect(rows, func(row scanner) (string, error) {
		var id string
		err := row.Scan(&id)
		return id, err
	})
}

func (s *Store) AlbumFolder(ctx context.Context, albumID string) (string, error) {
	var path string
	err := s.pool.QueryRow(ctx,
		"SELECT path FROM songs WHERE album_id=$1 ORDER BY disc_number, track_number, title LIMIT 1",
		albumID).Scan(&path)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", ErrNotFound
	}
	if err != nil {
		return "", err
	}
	return FolderOf(path), nil
}

// FolderOf returns the S3 folder prefix of an object key, trailing slash
// included, or "" for a key at the bucket root.
func FolderOf(path string) string {
	for i := len(path) - 1; i >= 0; i-- {
		if path[i] == '/' {
			return path[:i+1]
		}
	}
	return ""
}
