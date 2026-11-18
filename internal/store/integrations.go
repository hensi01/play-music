package store

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"

	"play-music/internal/model"
)

// TouchAPIKey validates a key hash against its current admin owner, bumps
// last_used_at and returns the owner id. ErrNotFound for an unknown, revoked
// or non-admin key.
func (s *Store) TouchAPIKey(ctx context.Context, keyHash []byte) (string, error) {
	var owner string
	err := s.pool.QueryRow(ctx, `
		UPDATE api_integration_keys k SET last_used_at=now()
		FROM users u
		WHERE k.key_hash=$1 AND k.revoked_at IS NULL AND u.id=k.owner_id AND u.is_admin=true
		RETURNING k.owner_id`, keyHash).Scan(&owner)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", ErrNotFound
	}
	return owner, err
}

func (s *Store) CreateAPIKey(ctx context.Context, id, name string, keyHash []byte, ownerID string) error {
	_, err := s.pool.Exec(ctx, `
		INSERT INTO api_integration_keys(id,name,key_hash,owner_id) VALUES($1,$2,$3,$4)`,
		id, name, keyHash, ownerID)
	return err
}

func (s *Store) ListAPIKeys(ctx context.Context) ([]model.APIKey, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id,name,created_at,last_used_at FROM api_integration_keys
		WHERE revoked_at IS NULL ORDER BY created_at DESC`)
	if err != nil {
		return nil, err
	}
	return collect(rows, func(row scanner) (model.APIKey, error) {
		var k model.APIKey
		err := row.Scan(&k.ID, &k.Name, &k.CreatedAt, &k.LastUsedAt)
		return k, err
	})
}

func (s *Store) RevokeAPIKey(ctx context.Context, id string) error {
	tag, err := s.pool.Exec(ctx, `
		UPDATE api_integration_keys SET revoked_at=now()
		WHERE id=$1 AND revoked_at IS NULL`, id)
	return notFoundIfEmpty(tag, err)
}

func (s *Store) RecordDownload(ctx context.Context, userID, songID, title, artist string) error {
	_, err := s.pool.Exec(ctx, `
		INSERT INTO download_history(user_id,song_id,title,artist) VALUES($1,$2,$3,$4)`,
		userID, songID, title, artist)
	return err
}

func (s *Store) Downloads(ctx context.Context, userID string) ([]model.Download, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT COALESCE(song_id,''), title, artist, downloaded_at
		FROM download_history WHERE user_id=$1
		ORDER BY downloaded_at DESC, id DESC LIMIT 500`, userID)
	if err != nil {
		return nil, err
	}
	return collect(rows, func(row scanner) (model.Download, error) {
		var d model.Download
		err := row.Scan(&d.ID, &d.Title, &d.Artist, &d.DownloadedAt)
		return d, err
	})
}

func (s *Store) CountAdmins(ctx context.Context) (int, error) {
	var n int
	err := s.pool.QueryRow(ctx, "SELECT count(*) FROM users WHERE is_admin").Scan(&n)
	return n, err
}
