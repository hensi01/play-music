package store

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"

	"play-music/internal/model"
)

const artistCols = `
	a.id, a.name,
	(SELECT count(*)::int FROM albums al WHERE al.artist_id=a.id) AS album_count,
	(SELECT count(*)::int FROM songs s WHERE s.artist_id=a.id) AS song_count,
	EXISTS(SELECT 1 FROM user_likes ul WHERE ul.entity_type='artist' AND ul.entity_id=a.id) AS liked`

// GetOrCreateArtist resolves a name to an artist id, creating it if needed.
func (s *Store) GetOrCreateArtist(ctx context.Context, name string) (string, error) {
	if name == "" {
		name = "Desconhecido"
	}
	id := newID()
	err := s.pool.QueryRow(ctx, `
		INSERT INTO artists (id, name, created_at)
		VALUES ($1, $2, now())
		ON CONFLICT (lower(name)) DO NOTHING
		RETURNING id`, id, name).Scan(&id)
	if err == nil {
		return id, nil
	}
	err = s.pool.QueryRow(ctx,
		"SELECT id FROM artists WHERE lower(name)=lower($1) LIMIT 1", name).Scan(&id)
	return id, err
}

func (s *Store) GetArtists(ctx context.Context) ([]model.Artist, error) {
	rows, err := s.pool.Query(ctx,
		"SELECT "+artistCols+" FROM artists a ORDER BY a.name COLLATE \"C\" ASC")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return collectArtists(rows)
}

func (s *Store) GetArtist(ctx context.Context, id string) (*model.Artist, error) {
	row := s.pool.QueryRow(ctx, "SELECT "+artistCols+" FROM artists a WHERE a.id=$1", id)
	artist, err := scanArtist(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	return artist, err
}

func (s *Store) SearchArtists(ctx context.Context, q string, limit int) ([]model.Artist, error) {
	like := likePattern(q)
	rows, err := s.pool.Query(ctx,
		"SELECT "+artistCols+` FROM artists a
		 WHERE a.name ILIKE $1 ESCAPE '\' ORDER BY a.name LIMIT $2`, like, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return collectArtists(rows)
}

func scanArtist(row pgx.Row) (*model.Artist, error) {
	var a model.Artist
	err := row.Scan(&a.ID, &a.Name, &a.AlbumCount, &a.SongCount, &a.Liked)
	if err != nil {
		return nil, err
	}
	return &a, nil
}

func collectArtists(rows pgx.Rows) ([]model.Artist, error) {
	defer rows.Close()
	out := []model.Artist{}
	for rows.Next() {
		var a model.Artist
		if err := rows.Scan(&a.ID, &a.Name, &a.AlbumCount, &a.SongCount, &a.Liked); err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}
