package store

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/jackc/pgx/v5"

	"play-music/internal/model"
)

const albumCols = `
	a.id, a.name, a.artist, COALESCE(a.artist_id, ''), a.year,
	COUNT(DISTINCT s.id)::int AS song_count,
	COALESCE(SUM(s.duration), 0)::float8 AS duration,
	a.created_at`

const albumJoin = ` FROM albums a LEFT JOIN songs s ON s.album_id=a.id`

func scanAlbum(row scanner) (*model.Album, error) {
	var a model.Album
	err := row.Scan(&a.ID, &a.Name, &a.Artist, &a.ArtistID, &a.Year,
		&a.SongCount, &a.Duration, &a.CreatedAt)
	if err != nil {
		return nil, err
	}
	return &a, nil
}

func collectAlbums(rows pgx.Rows) ([]model.Album, error) {
	albums, err := collect(rows, scanAlbum)
	if err != nil {
		return nil, err
	}
	out := make([]model.Album, len(albums))
	for i := range albums {
		out[i] = *albums[i]
	}
	return out, nil
}

func (s *Store) GetOrCreateAlbum(ctx context.Context, name, artist, artistID string, year int) (string, error) {
	return getOrCreateAlbum(ctx, s.pool, name, artist, artistID, year)
}

// getOrCreateAlbum is the one album upsert. Every writer of an albums row goes
// through it so the conflict action (which album fields a repeat preserves)
// cannot diverge between the scanner and a metadata edit.
func getOrCreateAlbum(ctx context.Context, q queryer, name, artist, artistID string, year int) (string, error) {
	if name == "" {
		name = UnknownArtist
	}
	id := NewID()
	err := q.QueryRow(ctx, `
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
	rows, err := s.pool.Query(ctx, "SELECT "+albumCols+albumJoin+
		` GROUP BY a.id ORDER BY a.name COLLATE "C" ASC, a.created_at`)
	if err != nil {
		return nil, err
	}
	return collectAlbums(rows)
}

func (s *Store) GetAlbum(ctx context.Context, id string) (*model.Album, error) {
	album, err := scanAlbum(s.pool.QueryRow(ctx,
		"SELECT "+albumCols+albumJoin+" WHERE a.id=$1 GROUP BY a.id", id))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	return album, err
}

func (s *Store) AlbumExists(ctx context.Context, id string) (bool, error) {
	var ok bool
	err := s.pool.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM albums WHERE id=$1)", id).Scan(&ok)
	return ok, err
}

func (s *Store) LikedAlbums(ctx context.Context, userID string, limit int) ([]model.Album, error) {
	rows, err := s.pool.Query(ctx, "SELECT "+albumCols+albumJoin+
		` JOIN user_likes ul ON ul.entity_type='`+EntityAlbum+`' AND ul.entity_id=a.id AND ul.user_id=$1
		 GROUP BY a.id, ul.created_at ORDER BY ul.created_at DESC LIMIT $2`, userID, limit)
	if err != nil {
		return nil, err
	}
	return collectAlbums(rows)
}

func (s *Store) ApplyAlbumLikes(ctx context.Context, userID string, albums []model.Album) error {
	liked, err := s.likedEntityIDs(ctx, userID, EntityAlbum, idsOf(albums, func(a model.Album) string { return a.ID }))
	if err != nil {
		return err
	}
	for i := range albums {
		albums[i].Liked = liked[albums[i].ID]
	}
	return nil
}

func (s *Store) SearchAlbums(ctx context.Context, q string, limit int) ([]model.Album, error) {
	rows, err := s.pool.Query(ctx, "SELECT "+albumCols+albumJoin+
		` WHERE (unaccent(a.name) ILIKE unaccent($1) ESCAPE '\' OR unaccent(a.artist) ILIKE unaccent($1) ESCAPE '\')
		 GROUP BY a.id ORDER BY a.name LIMIT $2`, likePattern(q), limit)
	if err != nil {
		return nil, err
	}
	return collectAlbums(rows)
}

func (s *Store) AlbumsByArtist(ctx context.Context, artistID string) ([]model.Album, error) {
	rows, err := s.pool.Query(ctx, "SELECT "+albumCols+albumJoin+
		" WHERE a.artist_id=$1 GROUP BY a.id ORDER BY a.year, a.name", artistID)
	if err != nil {
		return nil, err
	}
	return collectAlbums(rows)
}

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

func (s *Store) likedEntityIDs(ctx context.Context, userID, entityType string, ids []string) (map[string]bool, error) {
	liked := map[string]bool{}
	if len(ids) == 0 {
		return liked, nil
	}
	rows, err := s.pool.Query(ctx,
		`SELECT entity_id FROM user_likes WHERE user_id=$1 AND entity_type=$2 AND entity_id=ANY($3)`,
		userID, entityType, ids)
	if err != nil {
		return nil, err
	}
	found, err := collect(rows, func(row scanner) (string, error) {
		var id string
		err := row.Scan(&id)
		return id, err
	})
	if err != nil {
		return nil, err
	}
	for _, id := range found {
		liked[id] = true
	}
	return liked, nil
}

// LikedSet is the single-entity form of the liked-entity lookup, for a detail
// handler that has one row rather than a list.
func (s *Store) LikedSet(ctx context.Context, userID, entityType, entityID string) (bool, error) {
	liked, err := s.likedEntityIDs(ctx, userID, entityType, []string{entityID})
	return liked[entityID], err
}

func (s *Store) SetLike(ctx context.Context, userID, entityType, entityID string, liked bool) error {
	if liked {
		_, err := s.pool.Exec(ctx,
			`INSERT INTO user_likes (user_id, entity_type, entity_id, created_at) VALUES ($1, $2, $3, now())
			 ON CONFLICT (user_id, entity_type, entity_id) DO NOTHING`, userID, entityType, entityID)
		return err
	}
	_, err := s.pool.Exec(ctx,
		"DELETE FROM user_likes WHERE user_id=$1 AND entity_type=$2 AND entity_id=$3",
		userID, entityType, entityID)
	return err
}

var emptyQueue = []byte("[]")

func (s *Store) GetPlayQueue(ctx context.Context, userID string) ([]byte, error) {
	var data []byte
	err := s.pool.QueryRow(ctx, "SELECT data FROM play_queue WHERE user_id=$1", userID).Scan(&data)
	if errors.Is(err, pgx.ErrNoRows) {
		return emptyQueue, nil
	}
	if err != nil {
		return nil, err
	}
	return data, nil
}

func (s *Store) SavePlayQueue(ctx context.Context, userID string, data []byte) error {
	if len(data) == 0 || !json.Valid(data) {
		data = emptyQueue
	}
	_, err := s.pool.Exec(ctx,
		`INSERT INTO play_queue (user_id, data, updated_at) VALUES ($1, $2, now())
		 ON CONFLICT (user_id) DO UPDATE SET data=EXCLUDED.data, updated_at=now()`, userID, data)
	return err
}

func (s *Store) ApplySongLikes(ctx context.Context, userID string, songs []model.Song) error {
	liked, err := s.likedEntityIDs(ctx, userID, EntitySong, idsOf(songs, func(s model.Song) string { return s.ID }))
	if err != nil {
		return err
	}
	for i := range songs {
		songs[i].Liked = liked[songs[i].ID]
	}
	return nil
}
