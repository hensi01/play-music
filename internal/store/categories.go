package store

import (
	"context"
	"errors"
	"strings"

	"github.com/jackc/pgx/v5"

	"play-music/internal/model"
)

const categoryCols = `
	c.id, c.name,
	(SELECT count(*)::int FROM category_songs cs WHERE cs.category_id = c.id) AS song_count`

func scanCategory(row scanner) (*model.Category, error) {
	var c model.Category
	err := row.Scan(&c.ID, &c.Name, &c.SongCount)
	if err != nil {
		return nil, err
	}
	return &c, nil
}

func collectCategories(rows pgx.Rows) ([]model.Category, error) {
	categories, err := collect(rows, scanCategory)
	if err != nil {
		return nil, err
	}
	out := make([]model.Category, len(categories))
	for i := range categories {
		out[i] = *categories[i]
	}
	return out, nil
}

func upsertCategoryID(ctx context.Context, q queryer, name string) (string, error) {
	var id string
	err := q.QueryRow(ctx, `
		WITH ins AS (
			INSERT INTO categories (id, name, created_at)
			VALUES ($1, $2, now())
			ON CONFLICT (lower(name)) DO NOTHING
			RETURNING id
		)
		SELECT id FROM ins
		UNION ALL
		SELECT id FROM categories WHERE lower(name) = lower($2)
		LIMIT 1`, NewID(), name).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", ErrNotFound
	}
	return id, err
}

func (s *Store) CreateCategory(ctx context.Context, name string) (*model.Category, error) {
	id, err := upsertCategoryID(ctx, s.pool, name)
	if err != nil {
		return nil, err
	}
	return s.GetCategory(ctx, id)
}

// GetOrCreateCategory resolves a genre name to a category id, creating it on
// demand. A blank name resolves to "" so callers skip the assignment.
func (s *Store) GetOrCreateCategory(ctx context.Context, name string) (string, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return "", nil
	}
	id, err := upsertCategoryID(ctx, s.pool, name)
	return id, err
}

func (s *Store) GetCategories(ctx context.Context) ([]model.Category, error) {
	rows, err := s.pool.Query(ctx, "SELECT "+categoryCols+
		" FROM categories c ORDER BY c.name")
	if err != nil {
		return nil, err
	}
	return collectCategories(rows)
}

func (s *Store) GetCategory(ctx context.Context, id string) (*model.Category, error) {
	c, err := scanCategory(s.pool.QueryRow(ctx,
		"SELECT "+categoryCols+" FROM categories c WHERE c.id=$1", id))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	return c, err
}

func (s *Store) CategoryDetail(ctx context.Context, id string) ([]string, error) {
	if _, err := s.GetCategory(ctx, id); err != nil {
		return nil, err
	}
	rows, err := s.pool.Query(ctx,
		"SELECT song_id FROM category_songs WHERE category_id=$1 ORDER BY position, song_id", id)
	if err != nil {
		return nil, err
	}
	return collect(rows, func(row scanner) (string, error) {
		var id string
		err := row.Scan(&id)
		return id, err
	})
}

func (s *Store) UpdateCategory(ctx context.Context, id, name string, songIDs []string) error {
	return dbTx(ctx, s, func(q queryer) error {
		if name != "" {
			if _, err := q.Exec(ctx, "UPDATE categories SET name=$2 WHERE id=$1", id, name); err != nil {
				return err
			}
		}
		if songIDs == nil {
			return nil
		}
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
		return nil
	})
}

func (s *Store) AddSongToCategory(ctx context.Context, categoryID, songID string) error {
	_, err := s.pool.Exec(ctx, `
		INSERT INTO category_songs (category_id, song_id, position)
		VALUES ($1, $2, (SELECT COALESCE(max(position)+1, 0) FROM category_songs WHERE category_id=$1))
		ON CONFLICT DO NOTHING`, categoryID, songID)
	return err
}

func (s *Store) DeleteCategory(ctx context.Context, id string) error {
	tag, err := s.pool.Exec(ctx, "DELETE FROM categories WHERE id=$1", id)
	return notFoundIfEmpty(tag, err)
}

func (s *Store) CategorySongs(ctx context.Context, categoryID string) ([]model.Song, error) {
	rows, err := s.pool.Query(ctx,
		"SELECT "+songCols+` FROM songs s
		 WHERE s.id IN (SELECT song_id FROM category_songs cs WHERE cs.category_id=$1)
		 ORDER BY (SELECT position FROM category_songs cs2 WHERE cs2.category_id=$1 AND cs2.song_id=s.id),
		          s.title COLLATE "C" ASC`, categoryID)
	if err != nil {
		return nil, err
	}
	return collectSongs(rows)
}

// groupSongCategories folds (song_id, value) rows into a map. Both the
// category-id listing and the recommendation labels read the same pair shape
// from a different column.
func groupSongCategories(rows pgx.Rows) (map[string][]string, error) {
	type pair struct{ songID, value string }
	pairs, err := collect(rows, func(row scanner) (pair, error) {
		var p pair
		err := row.Scan(&p.songID, &p.value)
		return p, err
	})
	if err != nil {
		return nil, err
	}
	out := make(map[string][]string, len(pairs))
	for _, p := range pairs {
		out[p.songID] = append(out[p.songID], p.value)
	}
	return out, nil
}

func (s *Store) CategorySongIDs(ctx context.Context) (map[string][]string, error) {
	rows, err := s.pool.Query(ctx, "SELECT song_id, category_id FROM category_songs")
	if err != nil {
		return nil, err
	}
	return groupSongCategories(rows)
}

func (s *Store) SearchCategories(ctx context.Context, q string, limit int) ([]model.Category, error) {
	rows, err := s.pool.Query(ctx, "SELECT "+categoryCols+` FROM categories c
		WHERE unaccent(c.name) ILIKE unaccent($1) ESCAPE '\'
		ORDER BY c.name LIMIT $2`, likePattern(q), limit)
	if err != nil {
		return nil, err
	}
	return collectCategories(rows)
}
