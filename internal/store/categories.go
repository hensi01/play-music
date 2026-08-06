package store

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"

	"play-music/internal/model"
)

func (s *Store) CreateCategory(ctx context.Context, name string) (*model.Category, error) {
	c := &model.Category{ID: newID(), Name: name}
	_, err := s.pool.Exec(ctx,
		"INSERT INTO categories (id, name, created_at) VALUES ($1, $2, now()) ON CONFLICT (lower(name)) DO NOTHING",
		c.ID, name)
	if err != nil {
		return nil, err
	}
	return c, nil
}

func (s *Store) GetCategories(ctx context.Context) ([]model.Category, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT c.id, c.name,
			(SELECT count(*)::int FROM category_albums ca WHERE ca.category_id = c.id) AS album_count,
			(SELECT count(*)::int FROM category_artists ca WHERE ca.category_id = c.id) AS artist_count
		FROM categories c ORDER BY c.name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []model.Category
	for rows.Next() {
		var c model.Category
		if err := rows.Scan(&c.ID, &c.Name, &c.AlbumCount, &c.ArtistCount); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

func (s *Store) GetCategory(ctx context.Context, id string) (*model.Category, error) {
	var c model.Category
	err := s.pool.QueryRow(ctx, `
		SELECT c.id, c.name,
			(SELECT count(*)::int FROM category_albums ca WHERE ca.category_id = c.id),
			(SELECT count(*)::int FROM category_artists ca WHERE ca.category_id = c.id)
		FROM categories c WHERE c.id=$1`, id).
		Scan(&c.ID, &c.Name, &c.AlbumCount, &c.ArtistCount)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	return &c, err
}

// CategoryDetail returns the assigned album/artist ids (assignment screen).
func (s *Store) CategoryDetail(ctx context.Context, id string) (albumIDs, artistIDs []string, err error) {
	if _, err := s.GetCategory(ctx, id); err != nil {
		return nil, nil, err
	}
	rows, err := s.pool.Query(ctx, "SELECT album_id FROM category_albums WHERE category_id=$1", id)
	if err != nil {
		return nil, nil, err
	}
	albumIDs = []string{}
	for rows.Next() {
		var aid string
		if err := rows.Scan(&aid); err != nil {
			rows.Close()
			return nil, nil, err
		}
		albumIDs = append(albumIDs, aid)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, nil, err
	}
	rows, err = s.pool.Query(ctx, "SELECT artist_id FROM category_artists WHERE category_id=$1", id)
	if err != nil {
		return nil, nil, err
	}
	artistIDs = []string{}
	for rows.Next() {
		var aid string
		if err := rows.Scan(&aid); err != nil {
			rows.Close()
			return nil, nil, err
		}
		artistIDs = append(artistIDs, aid)
	}
	rows.Close()
	return albumIDs, artistIDs, rows.Err()
}

// UpdateCategory renames and/or replaces the album/artist assignments.
func (s *Store) UpdateCategory(ctx context.Context, id, name string, albumIDs, artistIDs []string) error {
	return dbTx(ctx, s, func(q queryer) error {
		if name != "" {
			if _, err := q.Exec(ctx, "UPDATE categories SET name=$2 WHERE id=$1", id, name); err != nil {
				return err
			}
		}
		if albumIDs != nil {
			if _, err := q.Exec(ctx, "DELETE FROM category_albums WHERE category_id=$1", id); err != nil {
				return err
			}
			for _, aid := range albumIDs {
				if _, err := q.Exec(ctx,
					"INSERT INTO category_albums (category_id, album_id) VALUES ($1, $2) ON CONFLICT DO NOTHING",
					id, aid); err != nil {
					return err
				}
			}
		}
		if artistIDs != nil {
			if _, err := q.Exec(ctx, "DELETE FROM category_artists WHERE category_id=$1", id); err != nil {
				return err
			}
			for _, aid := range artistIDs {
				if _, err := q.Exec(ctx,
					"INSERT INTO category_artists (category_id, artist_id) VALUES ($1, $2) ON CONFLICT DO NOTHING",
					id, aid); err != nil {
					return err
				}
			}
		}
		return nil
	})
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

// CategoryAlbums returns the albums of a category (direct + via artists).
func (s *Store) CategoryAlbums(ctx context.Context, categoryID string) ([]model.Album, error) {
	rows, err := s.pool.Query(ctx,
		"SELECT "+albumCols+albumJoin+`
		 WHERE a.id IN (
			SELECT album_id FROM category_albums WHERE category_id = $1
			UNION
			SELECT al.id FROM albums al
			JOIN category_artists c ON c.artist_id = al.artist_id AND c.category_id = $1
		 )
		 GROUP BY a.id ORDER BY a.name`, categoryID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return collectAlbums(rows)
}
