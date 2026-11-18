package store

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"

	"play-music/internal/model"
)

type SongFileInfo struct {
	Size  int64
	Mtime time.Time
}

const songCols = `
	s.id, s.path, s.title, COALESCE(NULLIF(s.artist, ''), (SELECT ar.name FROM artists ar WHERE ar.id=s.artist_id), ''), COALESCE(s.artist_id, ''), s.album,
	COALESCE(s.album_id, ''), s.year, s.genre, s.duration, s.format, s.bitrate,
	s.sample_rate, s.track_number, s.disc_number, s.size, s.has_cover,
	s.created_at, s.updated_at, s.play_count,
	FALSE AS liked`

// songDest is the destination list for a songCols row. It is the only place
// the column order is written down; every song query reads through it.
func songDest(s *model.Song) []any {
	return []any{
		&s.ID, &s.Path, &s.Title, &s.Artist, &s.ArtistID, &s.Album, &s.AlbumID,
		&s.Year, &s.Genre, &s.Duration, &s.Format, &s.Bitrate, &s.SampleRate,
		&s.TrackNumber, &s.DiscNumber, &s.Size, &s.HasCover, &s.CreatedAt,
		&s.UpdatedAt, &s.PlayCount, &s.Liked,
	}
}

func scanSong(row scanner) (*model.Song, error) {
	var s model.Song
	if err := row.Scan(songDest(&s)...); err != nil {
		return nil, err
	}
	return &s, nil
}

func collectSongs(rows pgx.Rows) ([]model.Song, error) {
	songs, err := collect(rows, scanSong)
	if err != nil {
		return nil, err
	}
	out := make([]model.Song, len(songs))
	for i := range songs {
		out[i] = *songs[i]
	}
	return out, nil
}

func (s *Store) GetSong(ctx context.Context, id string) (*model.Song, error) {
	song, err := scanSong(s.pool.QueryRow(ctx,
		"SELECT "+songCols+" FROM songs s WHERE s.id=$1", id))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	return song, err
}

func (s *Store) SongExists(ctx context.Context, id string) (bool, error) {
	var ok bool
	err := s.pool.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM songs WHERE id=$1)", id).Scan(&ok)
	return ok, err
}

func (s *Store) UpdateSongMeta(ctx context.Context, id, title, artist string) error {
	return dbTx(ctx, s, func(q queryer) error {
		var album, oldAlbumID string
		var year int
		if err := q.QueryRow(ctx, "SELECT album,year,COALESCE(album_id,'') FROM songs WHERE id=$1 FOR UPDATE", id).Scan(&album, &year, &oldAlbumID); err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return ErrNotFound
			}
			return err
		}
		if artist == "" {
			_, err := q.Exec(ctx, "UPDATE songs SET title=COALESCE(NULLIF($2,''),title),updated_at=now() WHERE id=$1", id, title)
			return err
		}
		var artistID, albumID string
		if err := q.QueryRow(ctx, `INSERT INTO artists(id,name) VALUES($1,$2) ON CONFLICT(lower(name)) DO UPDATE SET name=artists.name RETURNING id`, NewID(), artist).Scan(&artistID); err != nil {
			return err
		}
		if album == "" {
			album = UnknownArtist
		}
		var err error
		if albumID, err = getOrCreateAlbum(ctx, q, album, artist, artistID, year); err != nil {
			return err
		}
		if oldAlbumID != "" && oldAlbumID != albumID {
			if _, err := q.Exec(ctx, `INSERT INTO artworks(entity_type,entity_id,data,updated_at) SELECT '`+EntityAlbum+`',$2,data,now() FROM artworks WHERE entity_type='`+EntityAlbum+`' AND entity_id=$1 ON CONFLICT(entity_type,entity_id) DO NOTHING`, oldAlbumID, albumID); err != nil {
				return err
			}
		}
		_, err = q.Exec(ctx, `UPDATE songs SET title=COALESCE(NULLIF($2,''),title),artist=$3,artist_id=$4,album_id=$5,updated_at=now() WHERE id=$1`, id, title, artist, artistID, albumID)
		return err
	})
}

func (s *Store) UpsertSong(ctx context.Context, song *model.Song, mtime time.Time, size int64) (string, error) {
	var id string
	err := s.pool.QueryRow(ctx, `
		INSERT INTO songs (id, path, title, artist, artist_id, album, album_id, year,
			genre, duration, format, bitrate, sample_rate, track_number, disc_number,
			size, mtime, has_cover, created_at, updated_at)
		VALUES ($1, $2, $3, $4, NULLIF($5, ''), $6, NULLIF($7, ''), $8, $9, $10, $11,
			$12, $13, $14, $15, $16, $17, $18, now(), now())
		ON CONFLICT (path) DO UPDATE SET
			title=EXCLUDED.title, artist=EXCLUDED.artist, artist_id=EXCLUDED.artist_id,
			album=EXCLUDED.album, album_id=EXCLUDED.album_id, year=EXCLUDED.year,
			genre=EXCLUDED.genre, duration=EXCLUDED.duration, format=EXCLUDED.format,
			bitrate=EXCLUDED.bitrate, sample_rate=EXCLUDED.sample_rate,
			track_number=EXCLUDED.track_number, disc_number=EXCLUDED.disc_number,
			size=EXCLUDED.size, mtime=EXCLUDED.mtime, has_cover=EXCLUDED.has_cover,
			updated_at=now()
		RETURNING id`,
		song.ID, song.Path, song.Title, song.Artist, song.ArtistID, song.Album, song.AlbumID,
		song.Year, song.Genre, song.Duration, song.Format, song.Bitrate, song.SampleRate,
		song.TrackNumber, song.DiscNumber, size, mtime, song.HasCover).Scan(&id)
	return id, err
}

func (s *Store) LoadSongPaths(ctx context.Context) (map[string]SongFileInfo, error) {
	rows, err := s.pool.Query(ctx, "SELECT path, size, COALESCE(mtime, updated_at) FROM songs")
	if err != nil {
		return nil, err
	}
	type entry struct {
		path string
		info SongFileInfo
	}
	entries, err := collect(rows, func(row scanner) (entry, error) {
		var e entry
		err := row.Scan(&e.path, &e.info.Size, &e.info.Mtime)
		return e, err
	})
	if err != nil {
		return nil, err
	}
	out := make(map[string]SongFileInfo, len(entries))
	for _, e := range entries {
		out[e.path] = e.info
	}
	return out, nil
}

// DeleteMissing also drops albums, artists and likes that no song references any
// more.
func (s *Store) DeleteMissing(ctx context.Context, keep []string) (int64, error) {
	var deleted int64
	err := dbTx(ctx, s, func(q queryer) error {
		if err := q.QueryRow(ctx,
			"WITH d AS (DELETE FROM songs WHERE path <> ALL($1::text[]) RETURNING id) SELECT count(*) FROM d",
			keep).Scan(&deleted); err != nil {
			return err
		}
		if _, err := q.Exec(ctx, `
			DELETE FROM albums a WHERE NOT EXISTS (SELECT 1 FROM songs s WHERE s.album_id=a.id)`); err != nil {
			return err
		}
		if _, err := q.Exec(ctx, `
			DELETE FROM artists a WHERE NOT EXISTS (SELECT 1 FROM songs s WHERE s.artist_id=a.id)
			AND NOT EXISTS (SELECT 1 FROM albums al WHERE al.artist_id=a.id)`); err != nil {
			return err
		}
		_, err := q.Exec(ctx, `
			DELETE FROM user_likes ul
			WHERE (ul.entity_type='`+EntitySong+`' AND NOT EXISTS (SELECT 1 FROM songs s WHERE s.id=ul.entity_id))
			   OR (ul.entity_type='`+EntityAlbum+`' AND NOT EXISTS (SELECT 1 FROM albums a WHERE a.id=ul.entity_id))
			   OR (ul.entity_type='`+EntityArtist+`' AND NOT EXISTS (SELECT 1 FROM artists a WHERE a.id=ul.entity_id))`)
		return err
	})
	return deleted, err
}

func (s *Store) RegisterPlay(ctx context.Context, userID, songID string) error {
	return dbTx(ctx, s, func(q queryer) error {
		tag, err := q.Exec(ctx, `
			UPDATE songs SET play_count=play_count+1, last_played_at=now() WHERE id=$1`, songID)
		if err != nil {
			return err
		}
		if tag.RowsAffected() == 0 {
			return ErrNotFound
		}
		if _, err = q.Exec(ctx, "INSERT INTO history(song_id, user_id, played_at) VALUES($1, $2, now())", songID, userID); err != nil {
			return err
		}
		return nil
	})
}

func (s *Store) SearchSongs(ctx context.Context, q string, limit int) ([]model.Song, error) {
	rows, err := s.pool.Query(ctx, "SELECT "+songCols+` FROM songs s
		 WHERE (unaccent(s.title) ILIKE unaccent($1) ESCAPE '\'
		     OR unaccent(s.artist) ILIKE unaccent($1) ESCAPE '\'
		     OR unaccent(s.album) ILIKE unaccent($1) ESCAPE '\')
		 ORDER BY s.title LIMIT $2`, likePattern(q), limit)
	if err != nil {
		return nil, err
	}
	return collectSongs(rows)
}

func (s *Store) RecentlyAddedSongs(ctx context.Context, limit int) ([]model.Song, error) {
	rows, err := s.pool.Query(ctx, "SELECT "+songCols+
		" FROM songs s ORDER BY s.created_at DESC, s.title LIMIT $1", limit)
	if err != nil {
		return nil, err
	}
	return collectSongs(rows)
}

func (s *Store) MostPlayedSongs(ctx context.Context, limit int) ([]model.Song, error) {
	rows, err := s.pool.Query(ctx, "SELECT "+songCols+
		" FROM songs s ORDER BY s.play_count DESC, s.updated_at DESC, s.title LIMIT $1", limit)
	if err != nil {
		return nil, err
	}
	return collectSongs(rows)
}

func (s *Store) AllSongs(ctx context.Context) ([]model.Song, error) {
	rows, err := s.pool.Query(ctx, "SELECT "+songCols+
		` FROM songs s ORDER BY s.title COLLATE "C" ASC`)
	if err != nil {
		return nil, err
	}
	return collectSongs(rows)
}

func (s *Store) SongsByAlbum(ctx context.Context, albumID string) ([]model.Song, error) {
	rows, err := s.pool.Query(ctx,
		"SELECT "+songCols+" FROM songs s WHERE s.album_id=$1 ORDER BY s.disc_number, s.track_number, s.title",
		albumID)
	if err != nil {
		return nil, err
	}
	return collectSongs(rows)
}

func (s *Store) TopSongsByArtist(ctx context.Context, artistID string, limit int) ([]model.Song, error) {
	rows, err := s.pool.Query(ctx,
		"SELECT "+songCols+" FROM songs s WHERE s.artist_id=$1"+
			` ORDER BY s.play_count DESC, s.updated_at DESC LIMIT $2`,
		artistID, limit)
	if err != nil {
		return nil, err
	}
	return collectSongs(rows)
}

func (s *Store) LikedSongs(ctx context.Context, userID string, limit int) ([]model.Song, error) {
	rows, err := s.pool.Query(ctx, "SELECT "+songCols+` FROM songs s
		 JOIN user_likes ul ON ul.entity_type='`+EntitySong+`' AND ul.entity_id=s.id AND ul.user_id=$1
		 ORDER BY ul.created_at DESC LIMIT $2`, userID, limit)
	if err != nil {
		return nil, err
	}
	return collectSongs(rows)
}

func (s *Store) HistorySongs(ctx context.Context, userID string, limit int) ([]model.Song, error) {
	rows, err := s.pool.Query(ctx, "SELECT "+songCols+` FROM songs s
		 JOIN (SELECT song_id, max(played_at) AS last FROM history WHERE user_id=$1
		       GROUP BY song_id ORDER BY last DESC LIMIT $2) h ON h.song_id = s.id
		 ORDER BY h.last DESC`, userID, limit)
	if err != nil {
		return nil, err
	}
	return collectSongs(rows)
}

func likePattern(q string) string {
	var b []byte
	b = append(b, '%')
	for i := 0; i < len(q); i++ {
		c := q[i]
		switch c {
		case '\\', '%', '_':
			b = append(b, '\\', c)
		default:
			b = append(b, c)
		}
	}
	b = append(b, '%')
	return string(b)
}
