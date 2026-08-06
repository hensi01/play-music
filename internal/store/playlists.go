package store

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"

	"play-music/internal/model"
)

const playlistCols = `
	p.id, p.name, p.comment, p.owner,
	(SELECT count(*)::int FROM playlist_tracks pt WHERE pt.playlist_id=p.id) AS song_count`

func (s *Store) GetPlaylists(ctx context.Context) ([]model.Playlist, error) {
	rows, err := s.pool.Query(ctx,
		"SELECT "+playlistCols+" FROM playlists p ORDER BY p.name COLLATE \"C\" ASC")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []model.Playlist{}
	for rows.Next() {
		var p model.Playlist
		if err := rows.Scan(&p.ID, &p.Name, &p.Comment, &p.Owner, &p.SongCount); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

func (s *Store) SearchPlaylists(ctx context.Context, q string, limit int) ([]model.Playlist, error) {
	like := likePattern(q)
	rows, err := s.pool.Query(ctx,
		"SELECT "+playlistCols+` FROM playlists p
		 WHERE p.name ILIKE $1 ESCAPE '\' ORDER BY p.name LIMIT $2`, like, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []model.Playlist{}
	for rows.Next() {
		var p model.Playlist
		if err := rows.Scan(&p.ID, &p.Name, &p.Comment, &p.Owner, &p.SongCount); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

func (s *Store) GetPlaylist(ctx context.Context, id string) (*model.Playlist, error) {
	var p model.Playlist
	err := s.pool.QueryRow(ctx,
		"SELECT "+playlistCols+" FROM playlists p WHERE p.id=$1", id).
		Scan(&p.ID, &p.Name, &p.Comment, &p.Owner, &p.SongCount)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}

	rows, err := s.pool.Query(ctx, `
		SELECT pt.id, `+songCols+` FROM playlist_tracks pt
		JOIN songs s ON s.id=pt.song_id
		WHERE pt.playlist_id=$1 ORDER BY pt.position`, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var e model.PlaylistEntry
		var hasCover bool
		var lyrics string
		if err := rows.Scan(&e.EntryID, &e.Song.ID, &e.Song.Path, &e.Song.Title, &e.Song.Artist,
			&e.Song.ArtistID, &e.Song.Album, &e.Song.AlbumID, &e.Song.Year, &e.Song.Genre,
			&e.Song.Duration, &e.Song.Format, &e.Song.Bitrate, &e.Song.SampleRate,
			&e.Song.TrackNumber, &e.Song.DiscNumber, &e.Song.Size, &hasCover, &lyrics,
			&e.Song.CreatedAt, &e.Song.UpdatedAt, &e.Song.PlayCount, &e.Song.Liked); err != nil {
			return nil, err
		}
		p.Duration += e.Song.Duration
		p.Songs = append(p.Songs, e)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return &p, nil
}

func (s *Store) CreatePlaylist(ctx context.Context, name, comment string, songIDs []string) (*model.Playlist, error) {
	id := newID()
	err := dbTx(ctx, s, func(q queryer) error {
		if _, err := q.Exec(ctx,
			"INSERT INTO playlists (id, name, comment, owner, created_at, updated_at) VALUES ($1, $2, $3, 'admin', now(), now())",
			id, name, comment); err != nil {
			return err
		}
		return insertPlaylistTracks(ctx, q, id, songIDs)
	})
	if err != nil {
		return nil, err
	}
	return s.GetPlaylist(ctx, id)
}

func (s *Store) UpdatePlaylist(ctx context.Context, id string, patch *model.Playlist) error {
	tag, err := s.pool.Exec(ctx, `
		UPDATE playlists SET name=COALESCE(NULLIF($2, ''), name),
			comment=COALESCE(NULLIF($3, ''), comment), updated_at=now()
		WHERE id=$1`, id, patch.Name, patch.Comment)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *Store) DeletePlaylist(ctx context.Context, id string) error {
	tag, err := s.pool.Exec(ctx, "DELETE FROM playlists WHERE id=$1", id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *Store) AddPlaylistTracks(ctx context.Context, id string, songIDs []string) error {
	exists, err := s.playlistExists(ctx, id)
	if err != nil {
		return err
	}
	if !exists {
		return ErrNotFound
	}
	return dbTx(ctx, s, func(q queryer) error {
		return insertPlaylistTracks(ctx, q, id, songIDs)
	})
}

func (s *Store) RemovePlaylistTrack(ctx context.Context, id, entryID string) error {
	return dbTx(ctx, s, func(q queryer) error {
		tag, err := q.Exec(ctx,
			"DELETE FROM playlist_tracks WHERE playlist_id=$1 AND id=$2", id, entryID)
		if err != nil {
			return err
		}
		if tag.RowsAffected() == 0 {
			return ErrNotFound
		}
		return renumberPlaylist(ctx, q, id)
	})
}

// ReorderPlaylistTracks moves the track at index `from` to index `to`
// (0-based indices over the visible ordered list).
func (s *Store) ReorderPlaylistTracks(ctx context.Context, id string, from, to int) error {
	return dbTx(ctx, s, func(q queryer) error {
		rows, err := q.Query(ctx,
			"SELECT id FROM playlist_tracks WHERE playlist_id=$1 ORDER BY position", id)
		if err != nil {
			return err
		}
		var ids []string
		for rows.Next() {
			var tid string
			if err := rows.Scan(&tid); err != nil {
				rows.Close()
				return err
			}
			ids = append(ids, tid)
		}
		rows.Close()
		if err := rows.Err(); err != nil {
			return err
		}
		if from < 0 || from >= len(ids) || to < 0 || to >= len(ids) {
			return ErrNotFound
		}
		item := ids[from]
		ids = append(ids[:from], ids[from+1:]...)
		ids = append(ids[:to], append([]string{item}, ids[to:]...)...)
		for i, tid := range ids {
			if _, err := q.Exec(ctx,
				"UPDATE playlist_tracks SET position=$2 WHERE id=$1", tid, i); err != nil {
				return err
			}
		}
		return nil
	})
}

// ImportPlaylist creates or replaces a playlist (by owner+name) from imported
// .m3u files, matching tracks by path.
func (s *Store) ImportPlaylist(ctx context.Context, name, owner string, songPaths []string) error {
	return dbTx(ctx, s, func(q queryer) error {
		var plID string
		err := q.QueryRow(ctx,
			`INSERT INTO playlists (id, name, comment, owner, created_at, updated_at)
			 VALUES ($1, $2, '', $3, now(), now())
			 ON CONFLICT DO NOTHING RETURNING id`,
			newID(), name, owner).Scan(&plID)
		if err != nil && !errors.Is(err, pgx.ErrNoRows) {
			return err
		}
		if plID == "" {
			if err := q.QueryRow(ctx,
				"SELECT id FROM playlists WHERE lower(name)=lower($1) AND owner=$2 LIMIT 1",
				name, owner).Scan(&plID); err != nil {
				return err
			}
		}
		if _, err := q.Exec(ctx, "DELETE FROM playlist_tracks WHERE playlist_id=$1", plID); err != nil {
			return err
		}
		if _, err := q.Exec(ctx,
			"UPDATE playlists SET updated_at=now() WHERE id=$1", plID); err != nil {
			return err
		}
		if len(songPaths) == 0 {
			return nil
		}
		rows, err := q.Query(ctx,
			"SELECT path, id FROM songs WHERE path = ANY($1)", songPaths)
		if err != nil {
			return err
		}
		type pair struct{ path, id string }
		var pairs []pair
		for rows.Next() {
			var p pair
			if err := rows.Scan(&p.path, &p.id); err != nil {
				rows.Close()
				return err
			}
			pairs = append(pairs, p)
		}
		rows.Close()
		if err := rows.Err(); err != nil {
			return err
		}
		byPath := make(map[string]string, len(pairs))
		for _, p := range pairs {
			byPath[p.path] = p.id
		}
		pos := 0
		for _, path := range songPaths {
			songID, ok := byPath[path]
			if !ok {
				continue
			}
			if _, err := q.Exec(ctx,
				"INSERT INTO playlist_tracks (id, playlist_id, song_id, position) VALUES ($1, $2, $3, $4)",
				newID(), plID, songID, pos); err != nil {
				return err
			}
			pos++
		}
		return nil
	})
}

func (s *Store) playlistExists(ctx context.Context, id string) (bool, error) {
	var exists bool
	err := s.pool.QueryRow(ctx,
		"SELECT EXISTS(SELECT 1 FROM playlists WHERE id=$1)", id).Scan(&exists)
	return exists, err
}

// FirstPlaylistAlbum returns the album id of the first track of a playlist
// (used for playlist artwork).
func (s *Store) FirstPlaylistAlbum(ctx context.Context, playlistID string) (string, error) {
	var id string
	err := s.pool.QueryRow(ctx, `
		SELECT s.album_id FROM playlist_tracks pt
		JOIN songs s ON s.id=pt.song_id
		WHERE pt.playlist_id=$1 AND s.album_id IS NOT NULL
		ORDER BY pt.position LIMIT 1`, playlistID).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", ErrNotFound
	}
	return id, err
}

func insertPlaylistTracks(ctx context.Context, q queryer, playlistID string, songIDs []string) error {
	if len(songIDs) == 0 {
		return nil
	}
	var start int
	if err := q.QueryRow(ctx,
		"SELECT COALESCE(max(position)+1, 0) FROM playlist_tracks WHERE playlist_id=$1",
		playlistID).Scan(&start); err != nil {
		return err
	}
	for i, sid := range songIDs {
		if _, err := q.Exec(ctx,
			"INSERT INTO playlist_tracks (id, playlist_id, song_id, position) VALUES ($1, $2, $3, $4)",
			newID(), playlistID, sid, start+i); err != nil {
			return err
		}
	}
	return nil
}

func renumberPlaylist(ctx context.Context, q queryer, playlistID string) error {
	rows, err := q.Query(ctx,
		"SELECT id FROM playlist_tracks WHERE playlist_id=$1 ORDER BY position", playlistID)
	if err != nil {
		return err
	}
	var ids []string
	for rows.Next() {
		var tid string
		if err := rows.Scan(&tid); err != nil {
			rows.Close()
			return err
		}
		ids = append(ids, tid)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return err
	}
	for i, tid := range ids {
		if _, err := q.Exec(ctx,
			"UPDATE playlist_tracks SET position=$2 WHERE id=$1", tid, i); err != nil {
			return err
		}
	}
	return nil
}
