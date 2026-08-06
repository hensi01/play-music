package store

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
)

// visibleAlbumSet returns a SQL subquery of album ids a user can access:
// albums assigned directly to their granted categories UNION albums whose
// artist is assigned to a granted category. p is the pgx placeholder for the
// user id (e.g. "$1"). Callers omit the predicate entirely for admins
// (userID = "").
func visibleAlbumSet(p string) string {
	return `(SELECT ca.album_id FROM category_albums ca
		JOIN user_categories uc ON uc.category_id = ca.category_id AND uc.user_id = ` + p + `
		UNION
		SELECT al.id FROM albums al
		JOIN category_artists c2 ON c2.artist_id = al.artist_id
		JOIN user_categories uc2 ON uc2.category_id = c2.category_id AND uc2.user_id = ` + p + `)`
}

// HasAccessFilter reports whether the query must be filtered by user access.
// Admin/unspecified users (userID = "") see everything.
func (s *Store) HasAccessFilter(userID string) bool {
	return userID != ""
}

// CanAccessAlbum checks whether the user may see/play an album.
func (s *Store) CanAccessAlbum(ctx context.Context, userID, albumID string) (bool, error) {
	if userID == "" {
		return true, nil
	}
	var ok bool
	err := s.pool.QueryRow(ctx,
		"SELECT EXISTS(SELECT 1 FROM albums a WHERE a.id=$2 AND a.id IN "+visibleAlbumSet("$1")+")",
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
		"SELECT EXISTS(SELECT 1 FROM songs s WHERE s.id=$2 AND s.album_id IN "+visibleAlbumSet("$1")+")",
		userID, songID).Scan(&ok)
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
	// Artist? (any accessible album)
	var artistOK bool
	if err := s.pool.QueryRow(ctx,
		"SELECT EXISTS(SELECT 1 FROM albums a WHERE a.artist_id=$2 AND a.id IN "+visibleAlbumSet("$1")+")",
		userID, entityID).Scan(&artistOK); err != nil {
		return false, err
	}
	if artistOK {
		return true, nil
	}
	// Playlist? (first song accessible) — access to its songs is enforced
	// elsewhere; artwork here uses the album.
	var albumID string
	err = s.pool.QueryRow(ctx, `
		SELECT s.album_id FROM playlist_tracks pt
		JOIN songs s ON s.id = pt.song_id
		WHERE pt.playlist_id = $1 AND s.album_id IS NOT NULL
		ORDER BY pt.position LIMIT 1`, entityID).Scan(&albumID)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return s.CanAccessAlbum(ctx, userID, albumID)
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
