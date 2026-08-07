package store

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"

	"play-music/internal/model"
)

// SongFileInfo is the minimal info the scanner uses to detect changes.
type SongFileInfo struct {
	Size  int64
	Mtime time.Time
}

const songCols = `
	s.id, s.path, s.title, s.artist, COALESCE(s.artist_id, ''), s.album,
	COALESCE(s.album_id, ''), s.year, s.genre, s.duration, s.format, s.bitrate,
	s.sample_rate, s.track_number, s.disc_number, s.size, s.has_cover,
	COALESCE(s.lyrics, ''), s.created_at, s.updated_at, s.play_count,
	EXISTS(SELECT 1 FROM user_likes ul WHERE ul.entity_type='song' AND ul.entity_id=s.id) AS liked`

func scanSong(row pgx.Row) (*model.Song, error) {
	var s model.Song
	var size int64
	err := row.Scan(
		&s.ID, &s.Path, &s.Title, &s.Artist, &s.ArtistID, &s.Album, &s.AlbumID,
		&s.Year, &s.Genre, &s.Duration, &s.Format, &s.Bitrate, &s.SampleRate,
		&s.TrackNumber, &s.DiscNumber, &size, new(bool), new(string), &s.CreatedAt,
		&s.UpdatedAt, &s.PlayCount, &s.Liked,
	)
	if err != nil {
		return nil, err
	}
	s.Size = size
	return &s, nil
}

func (s *Store) GetSong(ctx context.Context, id string) (*model.Song, error) {
	row := s.pool.QueryRow(ctx,
		"SELECT "+songCols+" FROM songs s WHERE s.id=$1", id)
	song, err := scanSong(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	return song, err
}

func (s *Store) GetSongByPath(ctx context.Context, path string) (*model.Song, error) {
	row := s.pool.QueryRow(ctx,
		"SELECT "+songCols+" FROM songs s WHERE s.path=$1", path)
	song, err := scanSong(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	return song, err
}

// SongExists reports whether the song id exists (admin photo upload).
func (s *Store) SongExists(ctx context.Context, id string) (bool, error) {
	var ok bool
	err := s.pool.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM songs WHERE id=$1)", id).Scan(&ok)
	return ok, err
}

// UpdateSongMeta overrides the title/artist of a song (admin upload form).
func (s *Store) UpdateSongMeta(ctx context.Context, id, title, artist string) error {
	_, err := s.pool.Exec(ctx, `
		UPDATE songs SET
			title=COALESCE(NULLIF($2, ''), title),
			artist=COALESCE(NULLIF($3, ''), artist),
			updated_at=now()
		WHERE id=$1`, id, title, artist)
	return err
}

// UpsertSong inserts or updates a song (matched by path) and returns its id.
func (s *Store) UpsertSong(ctx context.Context, song *model.Song, mtime time.Time, size int64) (string, error) {
	var id string
	err := s.pool.QueryRow(ctx, `
		INSERT INTO songs (id, path, title, artist, artist_id, album, album_id, year,
			genre, duration, format, bitrate, sample_rate, track_number, disc_number,
			size, mtime, has_cover, lyrics, created_at, updated_at)
		VALUES ($1, $2, $3, $4, NULLIF($5, ''), $6, NULLIF($7, ''), $8, $9, $10, $11,
			$12, $13, $14, $15, $16, $17, $18, NULLIF($19, ''), now(), now())
		ON CONFLICT (path) DO UPDATE SET
			title=EXCLUDED.title, artist=EXCLUDED.artist, artist_id=EXCLUDED.artist_id,
			album=EXCLUDED.album, album_id=EXCLUDED.album_id, year=EXCLUDED.year,
			genre=EXCLUDED.genre, duration=EXCLUDED.duration, format=EXCLUDED.format,
			bitrate=EXCLUDED.bitrate, sample_rate=EXCLUDED.sample_rate,
			track_number=EXCLUDED.track_number, disc_number=EXCLUDED.disc_number,
			size=EXCLUDED.size, mtime=EXCLUDED.mtime, has_cover=EXCLUDED.has_cover,
			lyrics=EXCLUDED.lyrics, updated_at=now()
		RETURNING id`,
		song.ID, song.Path, song.Title, song.Artist, song.ArtistID, song.Album, song.AlbumID,
		song.Year, song.Genre, song.Duration, song.Format, song.Bitrate, song.SampleRate,
		song.TrackNumber, song.DiscNumber, size, mtime, song.HasCover, song.Lyrics).Scan(&id)
	if err != nil {
		return "", err
	}
	return id, nil
}

// LoadSongPaths returns path -> file info for change detection.
func (s *Store) LoadSongPaths(ctx context.Context) (map[string]SongFileInfo, error) {
	rows, err := s.pool.Query(ctx, "SELECT path, size, COALESCE(mtime, updated_at) FROM songs")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make(map[string]SongFileInfo)
	for rows.Next() {
		var p string
		var f SongFileInfo
		if err := rows.Scan(&p, &f.Size, &f.Mtime); err != nil {
			return nil, err
		}
		out[p] = f
	}
	return out, rows.Err()
}

// DeleteMissing removes songs whose path is no longer in the bucket, plus
// orphaned albums, artists and likes.
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
			WHERE (ul.entity_type='song' AND NOT EXISTS (SELECT 1 FROM songs s WHERE s.id=ul.entity_id))
			   OR (ul.entity_type='album' AND NOT EXISTS (SELECT 1 FROM albums a WHERE a.id=ul.entity_id))
			   OR (ul.entity_type='artist' AND NOT EXISTS (SELECT 1 FROM artists a WHERE a.id=ul.entity_id))`)
		return err
	})
	return deleted, err
}

// RegisterPlay records a play event and increments the play counter.
// The song must be accessible to the user.
// RegisterPlay bumps the play count and records history for userID.
// accessUserID drives the song access check ("" for admins/unspecified).
func (s *Store) RegisterPlay(ctx context.Context, userID, accessUserID, songID string) error {
	ok, err := s.CanAccessSong(ctx, accessUserID, songID)
	if err != nil {
		return err
	}
	if !ok {
		return ErrForbidden
	}
	return dbTx(ctx, s, func(q queryer) error {
		tag, err := q.Exec(ctx, `
			UPDATE songs SET play_count=play_count+1, last_played_at=now() WHERE id=$1`, songID)
		if err != nil {
			return err
		}
		if tag.RowsAffected() == 0 {
			return ErrNotFound
		}
		// Admin plays (userID "") are recorded without an owner.
		if userID == "" {
			_, err = q.Exec(ctx, "INSERT INTO history(song_id, user_id, played_at) VALUES($1, NULL, now())", songID)
		} else {
			_, err = q.Exec(ctx, "INSERT INTO history(song_id, user_id, played_at) VALUES($1, $2, now())", songID, userID)
		}
		return err
	})
}

func (s *Store) SearchSongs(ctx context.Context, userID, q string, limit int) ([]model.Song, error) {
	like := likePattern(q)
	base := "SELECT " + songCols + ` FROM songs s
		 WHERE (unaccent(s.title) ILIKE unaccent($1) ESCAPE '\'
		     OR unaccent(s.artist) ILIKE unaccent($1) ESCAPE '\'
		     OR unaccent(s.album) ILIKE unaccent($1) ESCAPE '\')`
	args := []any{like}
	limPh := "$2"
	if s.HasAccessFilter(userID) {
		base += " AND s.id IN " + visibleSongSet("$2")
		args = append(args, userID)
		limPh = "$3"
	}
	base += " ORDER BY s.title LIMIT " + limPh
	args = append(args, limit)
	rows, err := s.pool.Query(ctx, base, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return collectSongs(rows)
}

// RecentlyAddedSongs returns the newest songs (home "Adicionadas recentemente").
func (s *Store) RecentlyAddedSongs(ctx context.Context, userID string, limit int) ([]model.Song, error) {
	base := "SELECT " + songCols + " FROM songs s"
	args := []any{}
	limPh := "$1"
	if s.HasAccessFilter(userID) {
		base += " WHERE s.id IN " + visibleSongSet("$1")
		args = append(args, userID)
		limPh = "$2"
	}
	base += " ORDER BY s.created_at DESC, s.title LIMIT " + limPh
	args = append(args, limit)
	rows, err := s.pool.Query(ctx, base, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return collectSongs(rows)
}

// MostPlayedSongs returns the most played songs (home "Mais ouvidas").
func (s *Store) MostPlayedSongs(ctx context.Context, userID string, limit int) ([]model.Song, error) {
	base := "SELECT " + songCols + " FROM songs s"
	args := []any{}
	limPh := "$1"
	if s.HasAccessFilter(userID) {
		base += " WHERE s.id IN " + visibleSongSet("$1")
		args = append(args, userID)
		limPh = "$2"
	}
	base += " ORDER BY s.play_count DESC, s.updated_at DESC, s.title LIMIT " + limPh
	args = append(args, limit)
	rows, err := s.pool.Query(ctx, base, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return collectSongs(rows)
}

// AllSongs returns every song (admin listing; userID "" or filtered).
func (s *Store) AllSongs(ctx context.Context, userID string) ([]model.Song, error) {
	base := "SELECT " + songCols + " FROM songs s"
	args := []any{}
	if s.HasAccessFilter(userID) {
		base += " WHERE s.id IN " + visibleSongSet("$1")
		args = append(args, userID)
	}
	base += ` ORDER BY s.title COLLATE "C" ASC`
	rows, err := s.pool.Query(ctx, base, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return collectSongs(rows)
}

func (s *Store) SongsByAlbum(ctx context.Context, albumID string) ([]model.Song, error) {
	rows, err := s.pool.Query(ctx,
		"SELECT "+songCols+" FROM songs s WHERE s.album_id=$1 ORDER BY s.disc_number, s.track_number, s.title",
		albumID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return collectSongs(rows)
}

func (s *Store) TopSongsByArtist(ctx context.Context, userID, artistID string, limit int) ([]model.Song, error) {
	base := "SELECT " + songCols + ` FROM songs s WHERE s.artist_id=$1`
	args := []any{artistID}
	limPh := "$2"
	if s.HasAccessFilter(userID) {
		base += " AND s.id IN " + visibleSongSet("$2")
		args = append(args, userID)
		limPh = "$3"
	}
	base += ` ORDER BY s.play_count DESC, s.updated_at DESC LIMIT ` + limPh
	args = append(args, limit)
	rows, err := s.pool.Query(ctx, base, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return collectSongs(rows)
}

func (s *Store) LikedSongs(ctx context.Context, userID string, limit int) ([]model.Song, error) {
	base := "SELECT " + songCols + ` FROM songs s
		 JOIN user_likes ul ON ul.entity_type='song' AND ul.entity_id=s.id AND ul.user_id=$1`
	args := []any{userID, limit}
	if s.HasAccessFilter(userID) {
		base += " AND s.id IN " + visibleSongSet("$1")
	}
	base += " ORDER BY ul.created_at DESC LIMIT $2"
	rows, err := s.pool.Query(ctx, base, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return collectSongs(rows)
}

func (s *Store) HistorySongs(ctx context.Context, userID string, limit int) ([]model.Song, error) {
	base := "SELECT " + songCols + ` FROM songs s
		 JOIN (SELECT song_id, max(played_at) AS last FROM history WHERE user_id=$1
		       GROUP BY song_id ORDER BY last DESC LIMIT $2) h ON h.song_id = s.id`
	args := []any{userID, limit}
	if s.HasAccessFilter(userID) {
		base += " AND s.id IN " + visibleSongSet("$1")
	}
	base += " ORDER BY h.last DESC"
	rows, err := s.pool.Query(ctx, base, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return collectSongs(rows)
}

func collectSongs(rows pgx.Rows) ([]model.Song, error) {
	defer rows.Close()
	out := []model.Song{}
	for rows.Next() {
		var s model.Song
		var size int64
		if err := rows.Scan(
			&s.ID, &s.Path, &s.Title, &s.Artist, &s.ArtistID, &s.Album, &s.AlbumID,
			&s.Year, &s.Genre, &s.Duration, &s.Format, &s.Bitrate, &s.SampleRate,
			&s.TrackNumber, &s.DiscNumber, &size, new(bool), new(string), &s.CreatedAt,
			&s.UpdatedAt, &s.PlayCount, &s.Liked,
		); err != nil {
			return nil, err
		}
		s.Size = size
		out = append(out, s)
	}
	return out, rows.Err()
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
