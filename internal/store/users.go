package store

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"

	"play-music/internal/model"
)

const userCols = "id, username, phone, name, is_admin, created_at"

func scanUser(row pgx.Row) (*model.User, error) {
	var u model.User
	var username, phone *string
	err := row.Scan(&u.ID, &username, &phone, &u.Name, &u.IsAdmin, &u.CreatedAt)
	if err != nil {
		return nil, err
	}
	if username != nil {
		u.Username = *username
	}
	if phone != nil {
		u.Phone = *phone
	}
	return &u, nil
}

// CreateUser creates a user with a pre-hashed password and category grants.
func (s *Store) CreateUser(ctx context.Context, u *model.User, passwordHash string, categoryIDs []string) error {
	return dbTx(ctx, s, func(q queryer) error {
		if _, err := q.Exec(ctx, `
			INSERT INTO users (id, username, phone, name, password_hash, is_admin, created_at)
			VALUES ($1, NULLIF($2, ''), NULLIF($3, ''), $4, $5, $6, now())`,
			u.ID, u.Username, u.Phone, u.Name, passwordHash, u.IsAdmin); err != nil {
			return err
		}
		return setUserCategories(ctx, q, u.ID, categoryIDs)
	})
}

// GetUserByUsername returns the user with hash for username login.
func (s *Store) GetUserByUsername(ctx context.Context, username string) (*model.User, string, error) {
	u, err := scanUser(s.pool.QueryRow(ctx,
		"SELECT "+userCols+" FROM users WHERE username=$1", username))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, "", ErrNotFound
	}
	if err != nil {
		return nil, "", err
	}
	var hash string
	if err := s.pool.QueryRow(ctx,
		"SELECT password_hash FROM users WHERE id=$1", u.ID).Scan(&hash); err != nil {
		return nil, "", err
	}
	return u, hash, nil
}

// GetUserByPhone returns the user with hash for phone login.
func (s *Store) GetUserByPhone(ctx context.Context, phone string) (*model.User, string, error) {
	u, err := scanUser(s.pool.QueryRow(ctx,
		"SELECT "+userCols+" FROM users WHERE phone=$1", phone))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, "", ErrNotFound
	}
	if err != nil {
		return nil, "", err
	}
	var hash string
	if err := s.pool.QueryRow(ctx,
		"SELECT password_hash FROM users WHERE id=$1", u.ID).Scan(&hash); err != nil {
		return nil, "", err
	}
	return u, hash, nil
}

func (s *Store) GetUser(ctx context.Context, id string) (*model.User, error) {
	u, err := scanUser(s.pool.QueryRow(ctx, "SELECT "+userCols+" FROM users WHERE id=$1", id))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	return u, err
}

// ListUsers returns all users with their granted categories.
func (s *Store) ListUsers(ctx context.Context) ([]model.User, error) {
	rows, err := s.pool.Query(ctx, "SELECT "+userCols+" FROM users ORDER BY is_admin DESC, name")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []model.User
	for rows.Next() {
		u, err := scanUser(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *u)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	for i := range out {
		cats, err := s.UserCategories(ctx, out[i].ID)
		if err != nil {
			return nil, err
		}
		out[i].Categories = cats
	}
	return out, nil
}

type UserPatch struct {
	Name          string
	Phone         string
	Username      string
	PasswordHash  string
	CategoryIDs   []string
	SetCategories bool
}

// UpdateUser applies a partial update; empty patch fields are ignored unless
// explicitly requested (password, categories).
func (s *Store) UpdateUser(ctx context.Context, id string, p UserPatch) error {
	return dbTx(ctx, s, func(q queryer) error {
		if p.Name != "" {
			if _, err := q.Exec(ctx, "UPDATE users SET name=$2 WHERE id=$1", id, p.Name); err != nil {
				return err
			}
		}
		if p.Phone != "" {
			if _, err := q.Exec(ctx, "UPDATE users SET phone=$2 WHERE id=$1", id, p.Phone); err != nil {
				return err
			}
		}
		if p.Username != "" {
			if _, err := q.Exec(ctx, "UPDATE users SET username=$2 WHERE id=$1", id, p.Username); err != nil {
				return err
			}
		}
		if p.PasswordHash != "" {
			if _, err := q.Exec(ctx, "UPDATE users SET password_hash=$2 WHERE id=$1", id, p.PasswordHash); err != nil {
				return err
			}
		}
		if p.SetCategories {
			if err := setUserCategories(ctx, q, id, p.CategoryIDs); err != nil {
				return err
			}
		}
		return nil
	})
}

func (s *Store) DeleteUser(ctx context.Context, id string) error {
	tag, err := s.pool.Exec(ctx, "DELETE FROM users WHERE id=$1", id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *Store) UserCategories(ctx context.Context, userID string) ([]model.Category, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT c.id, c.name,
			(SELECT count(*)::int FROM category_songs cs WHERE cs.category_id = c.id) AS song_count
		FROM user_categories uc
		JOIN categories c ON c.id = uc.category_id
		WHERE uc.user_id=$1 ORDER BY c.name`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []model.Category
	for rows.Next() {
		var c model.Category
		if err := rows.Scan(&c.ID, &c.Name, &c.SongCount); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

func setUserCategories(ctx context.Context, q queryer, userID string, categoryIDs []string) error {
	if _, err := q.Exec(ctx, "DELETE FROM user_categories WHERE user_id=$1", userID); err != nil {
		return err
	}
	for _, cid := range categoryIDs {
		if _, err := q.Exec(ctx,
			"INSERT INTO user_categories (user_id, category_id, created_at) VALUES ($1, $2, now()) ON CONFLICT DO NOTHING",
			userID, cid); err != nil {
			return err
		}
	}
	return nil
}

// HasAdmin reports whether an admin user exists.
func (s *Store) HasAdmin(ctx context.Context) (bool, error) {
	var ok bool
	err := s.pool.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM users WHERE is_admin)").Scan(&ok)
	return ok, err
}

// GetAdminID returns the id of the (first) admin user.
func (s *Store) GetAdminID(ctx context.Context) (string, error) {
	var id string
	err := s.pool.QueryRow(ctx, "SELECT id FROM users WHERE is_admin ORDER BY created_at LIMIT 1").Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", ErrNotFound
	}
	return id, err
}

// BackfillLegacy assigns legacy rows (playlists, likes, history, queue) to
// the admin user. Runs once on bootstrap.
func (s *Store) BackfillLegacy(ctx context.Context, adminID string) error {
	return dbTx(ctx, s, func(q queryer) error {
		stmts := []string{
			"UPDATE playlists SET user_id=$1 WHERE user_id IS NULL",
			"UPDATE user_likes SET user_id=$1 WHERE user_id IS NULL",
			"UPDATE history SET user_id=$1 WHERE user_id IS NULL",
			"UPDATE play_queue SET user_id=$1 WHERE user_id IS NULL",
		}
		for _, stmt := range stmts {
			if _, err := q.Exec(ctx, stmt, adminID); err != nil {
				return err
			}
		}
		return nil
	})
}
