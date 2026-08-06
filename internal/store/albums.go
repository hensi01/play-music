package store

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"

	"play-music/internal/model"
)

const albumCols = `
	a.id, a.name, a.artist, COALESCE(a.artist_id, ''), a.year,
	COUNT(DISTINCT s.id)::int AS song_count,
	COALESCE(SUM(s.duration), 0)::float8 AS duration,
	EXISTS(SELECT 1 FROM user_likes ul WHERE ul.entity_type='album' AND ul.entity_id=a.id) AS liked,
	a.created_at`

const albumJoin = ` FROM albums a LEFT JOIN songs s ON s.album_id=a.id`

// GetOrCreateAlbum resolves (name, artist) to an album id, creating it if needed.
func (s *Store) GetOrCreateAlbum(ctx context.Context, name, artist, artistID string, year int) (string, error) {
	if name == "" {
		name = "Desconhecido"
	}
	id := newID()
	err := s.pool.QueryRow(ctx, `
		INSERT INTO albums (id, name, artist, artist_id, year, created_at, updated_at)
		VALUES ($1, $2, $3, NULLIF($4, ''), $5, now(), now())
		ON CONFLICT (lower(name), lower(artist)) DO UPDATE SET
			artist=EXCLUDED.artist, artist_id=COALESCE(EXCLUDED.artist_id, albums.artist_id),
			year=GREATEST(EXCLUDED.year, albums.year), updated_at=now()
		RETURNING id`,
		id, name, artist, artistID, year).Scan(&id)
	return id, err
}

func (s *Store) GetAlbums(ctx context.Context) ([]model.Album, error) {
	rows, err := s.pool.Query(ctx,
		"SELECT "+albumCols+albumJoin+
			` GROUP BY a.id ORDER BY a.name COLLATE "C" ASC, a.created_at`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return collectAlbums(rows)
}

func (s *Store) GetAlbum(ctx context.Context, id string) (*model.Album, error) {
	row := s.pool.QueryRow(ctx,
		"SELECT "+albumCols+albumJoin+" WHERE a.id=$1 GROUP BY a.id", id)
	album, err := scanAlbum(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	return album, err
}

func (s *Store) RecentlyAddedAlbums(ctx context.Context, limit int) ([]model.Album, error) {
	rows, err := s.pool.Query(ctx,
		"SELECT "+albumCols+albumJoin+
			` WHERE EXISTS (SELECT 1 FROM songs s2 WHERE s2.album_id=a.id)
		   GROUP BY a.id ORDER BY a.created_at DESC, a.name LIMIT $1`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return collectAlbums(rows)
}

func (s *Store) MostPlayedAlbums(ctx context.Context, limit int) ([]model.Album, error) {
	rows, err := s.pool.Query(ctx,
		"SELECT "+albumCols+albumJoin+
			` GROUP BY a.id
		   ORDER BY COALESCE(SUM(s.play_count), 0) DESC, a.name LIMIT $1`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return collectAlbums(rows)
}

func (s *Store) LikedAlbums(ctx context.Context, limit int) ([]model.Album, error) {
	rows, err := s.pool.Query(ctx,
		"SELECT "+albumCols+albumJoin+
			` JOIN user_likes ul ON ul.entity_type='album' AND ul.entity_id=a.id
		   GROUP BY a.id, ul.created_at ORDER BY ul.created_at DESC LIMIT $1`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return collectAlbums(rows)
}

func (s *Store) SearchAlbums(ctx context.Context, q string, limit int) ([]model.Album, error) {
	like := likePattern(q)
	rows, err := s.pool.Query(ctx,
		"SELECT "+albumCols+albumJoin+
			` WHERE a.name ILIKE $1 ESCAPE '\' OR a.artist ILIKE $1 ESCAPE '\'
		   GROUP BY a.id ORDER BY a.name LIMIT $2`, like, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return collectAlbums(rows)
}

// AlbumsByArtist returns the albums of an artist (for the artist detail page).
func (s *Store) AlbumsByArtist(ctx context.Context, artistID string) ([]model.Album, error) {
	rows, err := s.pool.Query(ctx,
		"SELECT "+albumCols+albumJoin+
			" WHERE a.artist_id=$1 GROUP BY a.id ORDER BY a.year, a.name",
		artistID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return collectAlbums(rows)
}

// TopAlbumsByArtist: first album of an artist (used for artist artwork).
func (s *Store) FirstAlbumByArtist(ctx context.Context, artistID string) (string, error) {
	var id string
	err := s.pool.QueryRow(ctx,
		"SELECT id FROM albums WHERE artist_id=$1 ORDER BY created_at, name LIMIT 1",
		artistID).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", ErrNotFound
	}
	return id, err
}

func (s *Store) FirstSongAlbum(ctx context.Context, songID string) (string, error) {
	var id string
	err := s.pool.QueryRow(ctx,
		"SELECT COALESCE(album_id, '') FROM songs WHERE id=$1", songID).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", ErrNotFound
	}
	return id, err
}

func (s *Store) Genres(ctx context.Context, limit int) ([]model.Genre, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT genre, count(*)::int AS n FROM songs
		 WHERE genre <> '' GROUP BY genre ORDER BY n DESC, genre LIMIT $1`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []model.Genre{}
	for rows.Next() {
		var g model.Genre
		if err := rows.Scan(&g.Name, &g.SongCount); err != nil {
			return nil, err
		}
		out = append(out, g)
	}
	return out, rows.Err()
}

func scanAlbum(row pgx.Row) (*model.Album, error) {
	var a model.Album
	var created time.Time
	err := row.Scan(&a.ID, &a.Name, &a.Artist, &a.ArtistID, &a.Year,
		&a.SongCount, &a.Duration, &a.Liked, &created)
	if err != nil {
		return nil, err
	}
	return &a, nil
}

func collectAlbums(rows pgx.Rows) ([]model.Album, error) {
	defer rows.Close()
	out := []model.Album{}
	for rows.Next() {
		var a model.Album
		var created time.Time
		if err := rows.Scan(&a.ID, &a.Name, &a.Artist, &a.ArtistID, &a.Year,
			&a.SongCount, &a.Duration, &a.Liked, &created); err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}
