package store

import (
	"context"
	"errors"
	"strings"

	"github.com/jackc/pgx/v5"

	"play-music/internal/model"
)

func (s *Store) CreateCategory(ctx context.Context, name, checkoutURL string) (*model.Category, error) {
	c := &model.Category{Name: name, CheckoutURL: checkoutURL}
	err := s.pool.QueryRow(ctx, `
		WITH ins AS (
			INSERT INTO categories (id, name, checkout_url, created_at)
			VALUES ($1, $2, $3, now())
			ON CONFLICT (lower(name)) DO NOTHING
			RETURNING id
		)
		SELECT id FROM ins
		UNION ALL
		SELECT id FROM categories WHERE lower(name) = lower($2)
		LIMIT 1`, newID(), name, checkoutURL).Scan(&c.ID)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return c, nil
}

// GetOrCreateCategory resolves a category name to its id, creating it if
// needed (used to auto-create categories from song genres).
func (s *Store) GetOrCreateCategory(ctx context.Context, name string) (string, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return "", nil
	}
	var id string
	err := s.pool.QueryRow(ctx, `
		WITH ins AS (
			INSERT INTO categories (id, name, created_at)
			VALUES ($1, $2, now())
			ON CONFLICT (lower(name)) DO NOTHING
			RETURNING id
		)
		SELECT id FROM ins
		UNION ALL
		SELECT id FROM categories WHERE lower(name) = lower($2)
		LIMIT 1`, newID(), name).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", nil
	}
	return id, err
}

func (s *Store) GetCategories(ctx context.Context) ([]model.Category, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT c.id, c.name, c.checkout_url,
			(SELECT count(*)::int FROM category_songs cs WHERE cs.category_id = c.id) AS song_count,
			(SELECT count(*)::int FROM category_karaokes ck WHERE ck.category_id = c.id) AS karaoke_count
		FROM categories c ORDER BY c.name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []model.Category
	for rows.Next() {
		var c model.Category
		if err := rows.Scan(&c.ID, &c.Name, &c.CheckoutURL, &c.SongCount, &c.KaraokeCount); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

func (s *Store) GetCategory(ctx context.Context, id string) (*model.Category, error) {
	var c model.Category
	err := s.pool.QueryRow(ctx, `
		SELECT c.id, c.name, c.checkout_url,
			(SELECT count(*)::int FROM category_songs cs WHERE cs.category_id = c.id),
			(SELECT count(*)::int FROM category_karaokes ck WHERE ck.category_id = c.id)
		FROM categories c WHERE c.id=$1`, id).
		Scan(&c.ID, &c.Name, &c.CheckoutURL, &c.SongCount, &c.KaraokeCount)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	return &c, err
}

// CategoryDetail returns the assigned song ids (assignment screen).
func (s *Store) CategoryDetail(ctx context.Context, id string) (songIDs []string, err error) {
	if _, err := s.GetCategory(ctx, id); err != nil {
		return nil, err
	}
	rows, err := s.pool.Query(ctx,
		"SELECT song_id FROM category_songs WHERE category_id=$1 ORDER BY position, song_id", id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	songIDs = []string{}
	for rows.Next() {
		var sid string
		if err := rows.Scan(&sid); err != nil {
			return nil, err
		}
		songIDs = append(songIDs, sid)
	}
	return songIDs, rows.Err()
}

// CategoryKaraokeIDs returns the assigned karaoke ids (assignment screen).
func (s *Store) CategoryKaraokeIDs(ctx context.Context, id string) (karaokeIDs []string, err error) {
	rows, err := s.pool.Query(ctx,
		"SELECT karaoke_id FROM category_karaokes WHERE category_id=$1 ORDER BY position, karaoke_id", id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	karaokeIDs = []string{}
	for rows.Next() {
		var kid string
		if err := rows.Scan(&kid); err != nil {
			return nil, err
		}
		karaokeIDs = append(karaokeIDs, kid)
	}
	return karaokeIDs, rows.Err()
}

// UpdateCategory renames, updates the checkout link and/or replaces the song
// and karaoke assignments. checkoutURL nil keeps the current value; a pointer
// (even "") sets it (allows clearing). songIDs/karaokeIDs nil keep the
// current assignment; a non-nil slice replaces it.
func (s *Store) UpdateCategory(ctx context.Context, id, name string, checkoutURL *string, songIDs []string, karaokeIDs []string) error {
	return dbTx(ctx, s, func(q queryer) error {
		if name != "" {
			if _, err := q.Exec(ctx, "UPDATE categories SET name=$2 WHERE id=$1", id, name); err != nil {
				return err
			}
		}
		if checkoutURL != nil {
			if _, err := q.Exec(ctx, "UPDATE categories SET checkout_url=$2 WHERE id=$1", id, *checkoutURL); err != nil {
				return err
			}
		}
		if songIDs != nil {
			if _, err := q.Exec(ctx, "DELETE FROM category_songs WHERE category_id=$1", id); err != nil {
				return err
			}
			for i, sid := range songIDs {
				if _, err := q.Exec(ctx,
					"INSERT INTO category_songs (category_id, song_id, position) VALUES ($1, $2, $3) ON CONFLICT DO NOTHING",
					id, sid, i); err != nil {
					return err
				}
			}
		}
		if karaokeIDs != nil {
			if _, err := q.Exec(ctx, "DELETE FROM category_karaokes WHERE category_id=$1", id); err != nil {
				return err
			}
			for i, kid := range karaokeIDs {
				if _, err := q.Exec(ctx,
					"INSERT INTO category_karaokes (category_id, karaoke_id, position) VALUES ($1, $2, $3) ON CONFLICT DO NOTHING",
					id, kid, i); err != nil {
					return err
				}
			}
		}
		return nil
	})
}

// AddSongToCategory assigns a single song to a category (used on upload).
func (s *Store) AddSongToCategory(ctx context.Context, categoryID, songID string) error {
	_, err := s.pool.Exec(ctx, `
		INSERT INTO category_songs (category_id, song_id, position)
		VALUES ($1, $2, (SELECT COALESCE(max(position)+1, 0) FROM category_songs WHERE category_id=$1))
		ON CONFLICT DO NOTHING`, categoryID, songID)
	return err
}

func (s *Store) DeleteCategory(ctx context.Context, id string) error {
	tag, err := s.pool.Exec(ctx, "DELETE FROM categories WHERE id=$1", id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// CategorySongs returns the songs assigned to a category, in order.
func (s *Store) CategorySongs(ctx context.Context, categoryID string) ([]model.Song, error) {
	rows, err := s.pool.Query(ctx,
		"SELECT "+songCols+` FROM songs s
		 WHERE s.id IN (SELECT song_id FROM category_songs cs WHERE cs.category_id=$1)
		 ORDER BY (SELECT position FROM category_songs cs2 WHERE cs2.category_id=$1 AND cs2.song_id=s.id),
		          s.title COLLATE "C" ASC`, categoryID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return collectSongs(rows)
}

// CategorySongIDs returns song_id -> category ids for the whole library
// (admin song listing).
func (s *Store) CategorySongIDs(ctx context.Context) (map[string][]string, error) {
	rows, err := s.pool.Query(ctx, "SELECT category_id, song_id FROM category_songs")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string][]string{}
	for rows.Next() {
		var cid, sid string
		if err := rows.Scan(&cid, &sid); err != nil {
			return nil, err
		}
		out[sid] = append(out[sid], cid)
	}
	return out, rows.Err()
}

// AddKaraokeToCategory assigns a single karaoke to a category (used on upload).
func (s *Store) AddKaraokeToCategory(ctx context.Context, categoryID, karaokeID string) error {
	_, err := s.pool.Exec(ctx, `
		INSERT INTO category_karaokes (category_id, karaoke_id, position)
		VALUES ($1, $2, (SELECT COALESCE(max(position)+1, 0) FROM category_karaokes WHERE category_id=$1))
		ON CONFLICT DO NOTHING`, categoryID, karaokeID)
	return err
}

// CategoryKaraokes returns the karaokes assigned to a category, in order.
func (s *Store) CategoryKaraokes(ctx context.Context, categoryID string) ([]model.Karaoke, error) {
	rows, err := s.pool.Query(ctx,
		"SELECT "+karaokeCols+` FROM karaokes k
		 WHERE k.id IN (SELECT karaoke_id FROM category_karaokes ck WHERE ck.category_id=$1)
		 ORDER BY (SELECT position FROM category_karaokes ck2 WHERE ck2.category_id=$1 AND ck2.karaoke_id=k.id),
		          k.title COLLATE "C" ASC`, categoryID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return collectKaraokes(rows)
}

// KaraokeCategoryIDs returns karaoke_id -> category ids for the whole library
// (admin karaoke listing).
func (s *Store) KaraokeCategoryIDs(ctx context.Context) (map[string][]string, error) {
	rows, err := s.pool.Query(ctx, "SELECT category_id, karaoke_id FROM category_karaokes")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string][]string{}
	for rows.Next() {
		var cid, kid string
		if err := rows.Scan(&cid, &kid); err != nil {
			return nil, err
		}
		out[kid] = append(out[kid], cid)
	}
	return out, rows.Err()
}

// SearchCategories searches categories by name. Clients only see the
// categories granted to them; admins (userID "") see all.
func (s *Store) SearchCategories(ctx context.Context, userID, q string, limit int) ([]model.Category, error) {
	like := likePattern(q)
	base := `SELECT c.id, c.name, c.checkout_url,
			(SELECT count(*)::int FROM category_songs cs WHERE cs.category_id = c.id) AS song_count,
			(SELECT count(*)::int FROM category_karaokes ck WHERE ck.category_id = c.id) AS karaoke_count
		FROM categories c WHERE unaccent(c.name) ILIKE unaccent($1) ESCAPE '\'`
	args := []any{like}
	limPh := "$2"
	if s.HasAccessFilter(userID) {
		base += " AND c.id IN (SELECT category_id FROM user_categories WHERE user_id=$2)"
		args = append(args, userID)
		limPh = "$3"
	}
	base += " ORDER BY c.name LIMIT " + limPh
	args = append(args, limit)
	rows, err := s.pool.Query(ctx, base, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []model.Category{}
	for rows.Next() {
		var c model.Category
		if err := rows.Scan(&c.ID, &c.Name, &c.CheckoutURL, &c.SongCount, &c.KaraokeCount); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}
