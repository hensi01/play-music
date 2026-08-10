package store

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"

	"play-music/internal/model"
)

// KaraokeFileInfo is the minimal info used to detect changes (upload flow).
type KaraokeFileInfo struct {
	Size  int64
	Mtime time.Time
}

const karaokeCols = `
	k.id, k.path, k.title, k.artist, k.duration, k.format, k.size,
	k.created_at, k.updated_at, k.play_count`

func scanKaraoke(row pgx.Row) (*model.Karaoke, error) {
	var k model.Karaoke
	var size int64
	err := row.Scan(
		&k.ID, &k.Path, &k.Title, &k.Artist, &k.Duration, &k.Format, &size,
		&k.CreatedAt, &k.UpdatedAt, &k.PlayCount,
	)
	if err != nil {
		return nil, err
	}
	k.Size = size
	return &k, nil
}

func (s *Store) GetKaraoke(ctx context.Context, id string) (*model.Karaoke, error) {
	row := s.pool.QueryRow(ctx,
		"SELECT "+karaokeCols+" FROM karaokes k WHERE k.id=$1", id)
	k, err := scanKaraoke(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	return k, err
}

func (s *Store) GetKaraokeByPath(ctx context.Context, path string) (*model.Karaoke, error) {
	row := s.pool.QueryRow(ctx,
		"SELECT "+karaokeCols+" FROM karaokes k WHERE k.path=$1", path)
	k, err := scanKaraoke(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	return k, err
}

// KaraokeExists reports whether the karaoke id exists (photo upload).
func (s *Store) KaraokeExists(ctx context.Context, id string) (bool, error) {
	var ok bool
	err := s.pool.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM karaokes WHERE id=$1)", id).Scan(&ok)
	return ok, err
}

// UpdateKaraokeMeta overrides the title/artist of a karaoke (admin upload form).
func (s *Store) UpdateKaraokeMeta(ctx context.Context, id, title, artist string) error {
	_, err := s.pool.Exec(ctx, `
		UPDATE karaokes SET
			title=COALESCE(NULLIF($2, ''), title),
			artist=COALESCE(NULLIF($3, ''), artist),
			updated_at=now()
		WHERE id=$1`, id, title, artist)
	return err
}

// UpsertKaraoke inserts or updates a karaoke (matched by path) and returns its id.
func (s *Store) UpsertKaraoke(ctx context.Context, k *model.Karaoke, mtime time.Time, size int64) (string, error) {
	var id string
	err := s.pool.QueryRow(ctx, `
		INSERT INTO karaokes (id, path, title, artist, duration, format, size, mtime, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, now(), now())
		ON CONFLICT (path) DO UPDATE SET
			title=EXCLUDED.title, artist=EXCLUDED.artist, duration=EXCLUDED.duration,
			format=EXCLUDED.format, size=EXCLUDED.size, mtime=EXCLUDED.mtime,
			updated_at=now()
		RETURNING id`,
		k.ID, k.Path, k.Title, k.Artist, k.Duration, k.Format, size, mtime).Scan(&id)
	if err != nil {
		return "", err
	}
	return id, nil
}

// RegisterKaraokePlay increments the play counter of a karaoke.
func (s *Store) RegisterKaraokePlay(ctx context.Context, userID, accessUserID, karaokeID string) error {
	ok, err := s.CanAccessKaraoke(ctx, accessUserID, karaokeID)
	if err != nil {
		return err
	}
	if !ok {
		return ErrForbidden
	}
	tag, err := s.pool.Exec(ctx,
		"UPDATE karaokes SET play_count=play_count+1 WHERE id=$1", karaokeID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// AllKaraokes returns every karaoke the user can access (admin: all).
func (s *Store) AllKaraokes(ctx context.Context, userID string) ([]model.Karaoke, error) {
	base := "SELECT " + karaokeCols + " FROM karaokes k"
	args := []any{}
	if s.HasAccessFilter(userID) {
		base += " WHERE k.id IN " + visibleKaraokeSet("$1")
		args = append(args, userID)
	}
	base += ` ORDER BY k.title COLLATE "C" ASC`
	rows, err := s.pool.Query(ctx, base, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return collectKaraokes(rows)
}

// RecentlyAddedKaraokes returns the newest karaokes (home section).
func (s *Store) RecentlyAddedKaraokes(ctx context.Context, userID string, limit int) ([]model.Karaoke, error) {
	base := "SELECT " + karaokeCols + " FROM karaokes k"
	args := []any{}
	limPh := "$1"
	if s.HasAccessFilter(userID) {
		base += " WHERE k.id IN " + visibleKaraokeSet("$1")
		args = append(args, userID)
		limPh = "$2"
	}
	base += " ORDER BY k.created_at DESC, k.title LIMIT " + limPh
	args = append(args, limit)
	rows, err := s.pool.Query(ctx, base, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return collectKaraokes(rows)
}

func collectKaraokes(rows pgx.Rows) ([]model.Karaoke, error) {
	defer rows.Close()
	out := []model.Karaoke{}
	for rows.Next() {
		var k model.Karaoke
		var size int64
		if err := rows.Scan(
			&k.ID, &k.Path, &k.Title, &k.Artist, &k.Duration, &k.Format, &size,
			&k.CreatedAt, &k.UpdatedAt, &k.PlayCount,
		); err != nil {
			return nil, err
		}
		k.Size = size
		out = append(out, k)
	}
	return out, rows.Err()
}
