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
	(SELECT count(*)::int FROM songs s WHERE s.artist_id=a.id) AS song_count`

func scanArtist(row scanner) (*model.Artist, error) {
	var a model.Artist
	err := row.Scan(&a.ID, &a.Name, &a.AlbumCount, &a.SongCount)
	if err != nil {
		return nil, err
	}
	return &a, nil
}

func collectArtists(rows pgx.Rows) ([]model.Artist, error) {
	artists, err := collect(rows, scanArtist)
	if err != nil {
		return nil, err
	}
	out := make([]model.Artist, len(artists))
	for i := range artists {
		out[i] = *artists[i]
	}
	return out, nil
}

func (s *Store) GetOrCreateArtist(ctx context.Context, name string) (string, error) {
	if name == "" {
		name = UnknownArtist
	}
	id := NewID()
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
	rows, err := s.pool.Query(ctx, "SELECT "+artistCols+` FROM artists a
		WHERE EXISTS (SELECT 1 FROM songs s WHERE s.artist_id=a.id)
		ORDER BY a.name COLLATE "C" ASC`)
	if err != nil {
		return nil, err
	}
	return collectArtists(rows)
}

func (s *Store) GetArtist(ctx context.Context, id string) (*model.Artist, error) {
	artist, err := scanArtist(s.pool.QueryRow(ctx,
		"SELECT "+artistCols+" FROM artists a WHERE a.id=$1", id))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	return artist, err
}

func (s *Store) SearchArtists(ctx context.Context, q string, limit int) ([]model.Artist, error) {
	rows, err := s.pool.Query(ctx, "SELECT "+artistCols+` FROM artists a
		WHERE unaccent(a.name) ILIKE unaccent($1) ESCAPE '\'
		ORDER BY a.name LIMIT $2`, likePattern(q), limit)
	if err != nil {
		return nil, err
	}
	return collectArtists(rows)
}

func (s *Store) ApplyArtistLikes(ctx context.Context, userID string, artists []model.Artist) error {
	liked, err := s.likedEntityIDs(ctx, userID, EntityArtist, idsOf(artists, func(a model.Artist) string { return a.ID }))
	if err != nil {
		return err
	}
	for i := range artists {
		artists[i].Liked = liked[artists[i].ID]
	}
	return nil
}
