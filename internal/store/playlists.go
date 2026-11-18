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

func scanPlaylist(row scanner) (*model.Playlist, error) {
	var p model.Playlist
	err := row.Scan(&p.ID, &p.Name, &p.Comment, &p.Owner, &p.SongCount)
	if err != nil {
		return nil, err
	}
	return &p, nil
}

func collectPlaylists(rows pgx.Rows) ([]model.Playlist, error) {
	playlists, err := collect(rows, scanPlaylist)
	if err != nil {
		return nil, err
	}
	out := make([]model.Playlist, len(playlists))
	for i := range playlists {
		out[i] = *playlists[i]
	}
	return out, nil
}

func (s *Store) GetPlaylists(ctx context.Context, userID string) ([]model.Playlist, error) {
	rows, err := s.pool.Query(ctx,
		"SELECT "+playlistCols+" FROM playlists p WHERE p.user_id=$1 ORDER BY p.name COLLATE \"C\" ASC",
		userID)
	if err != nil {
		return nil, err
	}
	return collectPlaylists(rows)
}

func (s *Store) SearchPlaylists(ctx context.Context, userID, q string, limit int) ([]model.Playlist, error) {
	rows, err := s.pool.Query(ctx,
		"SELECT "+playlistCols+` FROM playlists p
		 WHERE p.user_id=$1 AND unaccent(p.name) ILIKE unaccent($2) ESCAPE '\' ORDER BY p.name LIMIT $3`,
		userID, likePattern(q), limit)
	if err != nil {
		return nil, err
	}
	return collectPlaylists(rows)
}

func (s *Store) GetPlaylist(ctx context.Context, userID, id string) (*model.Playlist, error) {
	p, err := scanPlaylist(s.pool.QueryRow(ctx,
		"SELECT "+playlistCols+" FROM playlists p WHERE p.id=$1 AND p.user_id=$2", id, userID))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}

	rows, err := s.pool.Query(ctx,
		"SELECT pt.id, "+songCols+` FROM playlist_tracks pt
		 JOIN songs s ON s.id=pt.song_id
		 WHERE pt.playlist_id=$1 ORDER BY pt.position`, id)
	if err != nil {
		return nil, err
	}
	p.Songs, err = collect(rows, func(row scanner) (model.PlaylistEntry, error) {
		var entryID string
		var song model.Song
		if err := row.Scan(append([]any{&entryID}, songDest(&song)...)...); err != nil {
			return model.PlaylistEntry{}, err
		}
		return model.PlaylistEntry{EntryID: entryID, Song: song}, nil
	})
	if err != nil {
		return nil, err
	}
	for _, e := range p.Songs {
		p.Duration += e.Song.Duration
	}
	return p, nil
}

func (s *Store) CreatePlaylist(ctx context.Context, userID, name, comment string, songIDs []string) (*model.Playlist, error) {
	if err := s.validateSongIDs(ctx, songIDs); err != nil {
		return nil, err
	}
	id := NewID()
	err := dbTx(ctx, s, func(q queryer) error {
		if _, err := q.Exec(ctx,
			"INSERT INTO playlists (id, name, comment, owner, user_id, created_at, updated_at) VALUES ($1, $2, $3, $4, $4, now(), now())",
			id, name, comment, userID); err != nil {
			return err
		}
		return insertPlaylistTracks(ctx, q, id, songIDs)
	})
	if err != nil {
		return nil, err
	}
	return s.GetPlaylist(ctx, userID, id)
}

func (s *Store) UpdatePlaylist(ctx context.Context, userID, id string, patch *model.Playlist) error {
	tag, err := s.pool.Exec(ctx, `
		UPDATE playlists SET name=COALESCE(NULLIF($3, ''), name),
			comment=COALESCE(NULLIF($4, ''), comment), updated_at=now()
		WHERE id=$1 AND user_id=$2`, id, userID, patch.Name, patch.Comment)
	return notFoundIfEmpty(tag, err)
}

func (s *Store) DeletePlaylist(ctx context.Context, userID, id string) error {
	tag, err := s.pool.Exec(ctx, "DELETE FROM playlists WHERE id=$1 AND user_id=$2", id, userID)
	return notFoundIfEmpty(tag, err)
}

func (s *Store) AddPlaylistTracks(ctx context.Context, userID, id string, songIDs []string) error {
	if err := s.validateSongIDs(ctx, songIDs); err != nil {
		return err
	}
	owned, err := s.playlistOwned(ctx, userID, id)
	if err != nil {
		return err
	}
	if !owned {
		return ErrNotFound
	}
	return dbTx(ctx, s, func(q queryer) error {
		return insertPlaylistTracks(ctx, q, id, songIDs)
	})
}

func (s *Store) RemovePlaylistTrack(ctx context.Context, userID, id, entryID string) error {
	return dbTx(ctx, s, func(q queryer) error {
		tag, err := q.Exec(ctx,
			"DELETE FROM playlist_tracks WHERE playlist_id=$1 AND id=$2 AND playlist_id IN (SELECT id FROM playlists WHERE user_id=$3)",
			id, entryID, userID)
		if err := notFoundIfEmpty(tag, err); err != nil {
			return err
		}
		ids, err := playlistTrackIDs(ctx, q, id)
		if err != nil {
			return err
		}
		return renumberPlaylist(ctx, q, ids)
	})
}

func (s *Store) ReorderPlaylistTracks(ctx context.Context, userID, id string, from, to int) error {
	owned, err := s.playlistOwned(ctx, userID, id)
	if err != nil {
		return err
	}
	if !owned {
		return ErrNotFound
	}
	return dbTx(ctx, s, func(q queryer) error {
		ids, err := playlistTrackIDs(ctx, q, id)
		if err != nil {
			return err
		}
		if from < 0 || from >= len(ids) || to < 0 || to >= len(ids) {
			return ErrNotFound
		}
		item := ids[from]
		ids = append(ids[:from], ids[from+1:]...)
		ids = append(ids[:to], append([]string{item}, ids[to:]...)...)
		return renumberPlaylist(ctx, q, ids)
	})
}

// ImportPlaylist creates or replaces the admin playlist named after an imported
// .m3u file. playlists has no uniqueness on (name, owner, user_id), so "replace"
// is an explicit lookup rather than an ON CONFLICT branch that can never fire.
func (s *Store) ImportPlaylist(ctx context.Context, userID, name, owner string, songPaths []string) error {
	return dbTx(ctx, s, func(q queryer) error {
		var plID string
		err := q.QueryRow(ctx,
			"SELECT id FROM playlists WHERE lower(name)=lower($1) AND owner=$2 AND user_id=$3 LIMIT 1",
			name, owner, userID).Scan(&plID)
		if errors.Is(err, pgx.ErrNoRows) {
			err = q.QueryRow(ctx,
				`INSERT INTO playlists (id, name, comment, owner, user_id, created_at, updated_at)
				 VALUES ($1, $2, '', $3, $4, now(), now()) RETURNING id`,
				NewID(), name, owner, userID).Scan(&plID)
		}
		if err != nil {
			return err
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
		rows, err := q.Query(ctx, "SELECT path, id FROM songs WHERE path = ANY($1)", songPaths)
		if err != nil {
			return err
		}
		pairs, err := collect(rows, func(row scanner) (struct{ path, id string }, error) {
			var p struct{ path, id string }
			err := row.Scan(&p.path, &p.id)
			return p, err
		})
		if err != nil {
			return err
		}
		byPath := make(map[string]string, len(pairs))
		for _, p := range pairs {
			byPath[p.path] = p.id
		}
		// The track list was cleared above, so the insert starts at position 0.
		ids := make([]string, 0, len(songPaths))
		for _, path := range songPaths {
			if id, ok := byPath[path]; ok {
				ids = append(ids, id)
			}
		}
		return insertPlaylistTracks(ctx, q, plID, ids)
	})
}

func (s *Store) playlistOwned(ctx context.Context, userID, id string) (bool, error) {
	var exists bool
	err := s.pool.QueryRow(ctx,
		"SELECT EXISTS(SELECT 1 FROM playlists WHERE id=$1 AND user_id=$2)", id, userID).Scan(&exists)
	return exists, err
}

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

func (s *Store) validateSongIDs(ctx context.Context, songIDs []string) error {
	if len(songIDs) == 0 {
		return nil
	}
	var n int
	if err := s.pool.QueryRow(ctx,
		"SELECT count(*) FROM songs WHERE id = ANY($1)", songIDs).Scan(&n); err != nil {
		return err
	}
	if n != len(songIDs) {
		return ErrForbidden
	}
	return nil
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
			NewID(), playlistID, sid, start+i); err != nil {
			return err
		}
	}
	return nil
}

func playlistTrackIDs(ctx context.Context, q queryer, playlistID string) ([]string, error) {
	rows, err := q.Query(ctx,
		"SELECT id FROM playlist_tracks WHERE playlist_id=$1 ORDER BY position", playlistID)
	if err != nil {
		return nil, err
	}
	return collect(rows, func(row scanner) (string, error) {
		var id string
		err := row.Scan(&id)
		return id, err
	})
}

func renumberPlaylist(ctx context.Context, q queryer, ids []string) error {
	for i, tid := range ids {
		if _, err := q.Exec(ctx,
			"UPDATE playlist_tracks SET position=$2 WHERE id=$1", tid, i); err != nil {
			return err
		}
	}
	return nil
}
