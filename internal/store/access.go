package store

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
)

// visibleSongSet returns a SQL subquery of song ids a user can access: songs
// assigned directly to their granted categories. p is the pgx placeholder for
// the user id (e.g. "$1"). Callers omit the predicate entirely for admins
// (userID = "").
func visibleSongSet(p string) string {
	return `(SELECT cs.song_id FROM category_songs cs
		JOIN user_categories uc ON uc.category_id = cs.category_id AND uc.user_id = ` + p + `)`
}

// visibleAlbumSet returns a SQL subquery of album ids that contain at least
// one accessible song (legacy album/artist endpoints). p is the pgx
// placeholder for the user id.
func visibleAlbumSet(p string) string {
	return `(SELECT DISTINCT s.album_id FROM songs s
		JOIN category_songs cs ON cs.song_id = s.id
		JOIN user_categories uc ON uc.category_id = cs.category_id AND uc.user_id = ` + p + `
		WHERE s.album_id IS NOT NULL)`
}

// HasAccessFilter reports whether the query must be filtered by user access.
// Admin/unspecified users (userID = "") see everything.
func (s *Store) HasAccessFilter(userID string) bool {
	return userID != ""
}

// CanAccessAlbum checks whether the user may see/play an album (any of its
// songs is assigned to a granted category).
func (s *Store) CanAccessAlbum(ctx context.Context, userID, albumID string) (bool, error) {
	if userID == "" {
		return true, nil
	}
	var ok bool
	err := s.pool.QueryRow(ctx,
		"SELECT EXISTS(SELECT 1 FROM songs s WHERE s.album_id=$2 AND s.id IN "+visibleSongSet("$1")+")",
		userID, albumID).Scan(&ok)
	return ok, err
}

// CanAccessSong checks whether the user may stream a song.
func (s *Store) CanAccessSong(ctx context.Context, userID, songID string) (bool, error) {
	if userID == "" {
		return true, nil
	}
	var ok bool
	err := s.pool.QueryRow(ctx,
		"SELECT EXISTS(SELECT 1 FROM songs s WHERE s.id=$2 AND s.id IN "+visibleSongSet("$1")+")",
		userID, songID).Scan(&ok)
	return ok, err
}

// CanAccessCategory checks whether the user has a category granted.
func (s *Store) CanAccessCategory(ctx context.Context, userID, categoryID string) (bool, error) {
	if userID == "" {
		return true, nil
	}
	var ok bool
	err := s.pool.QueryRow(ctx,
		"SELECT EXISTS(SELECT 1 FROM user_categories uc WHERE uc.user_id=$1 AND uc.category_id=$2)",
		userID, categoryID).Scan(&ok)
	return ok, err
}

// CanAccessEntity resolves any entity id (song, album, artist or playlist)
// used by the artwork endpoint.
func (s *Store) CanAccessEntity(ctx context.Context, userID, entityID string) (bool, error) {
	if userID == "" {
		return true, nil
	}
	// Song?
	ok, err := s.CanAccessSong(ctx, userID, entityID)
	if err != nil || ok {
		return ok, err
	}
	// Album?
	ok, err = s.CanAccessAlbum(ctx, userID, entityID)
	if err != nil || ok {
		return ok, err
	}
	// Artist? (any accessible song)
	var artistOK bool
	if err := s.pool.QueryRow(ctx,
		"SELECT EXISTS(SELECT 1 FROM songs s WHERE s.artist_id=$2 AND s.id IN "+visibleSongSet("$1")+")",
		userID, entityID).Scan(&artistOK); err != nil {
		return false, err
	}
	if artistOK {
		return true, nil
	}
	// Playlist? (first song accessible) — access to its songs is enforced
	// elsewhere; artwork here uses the song photo/album.
	var songID string
	err = s.pool.QueryRow(ctx, `
		SELECT s.id FROM playlist_tracks pt
		JOIN songs s ON s.id = pt.song_id
		WHERE pt.playlist_id = $1
		ORDER BY pt.position LIMIT 1`, entityID).Scan(&songID)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return s.CanAccessSong(ctx, userID, songID)
}

// GrantedCategoryIDs returns the category ids granted to a user.
func (s *Store) GrantedCategoryIDs(ctx context.Context, userID string) ([]string, error) {
	rows, err := s.pool.Query(ctx,
		"SELECT category_id FROM user_categories WHERE user_id=$1 ORDER BY created_at", userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}
