package store

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"

	"play-music/internal/model"
)

const userCols = "id, username, email, phone, name, is_admin, created_at"

func scanUser(row pgx.Row) (*model.User, error) {
	var u model.User
	var username, email, phone *string
	err := row.Scan(&u.ID, &username, &email, &phone, &u.Name, &u.IsAdmin, &u.CreatedAt)
	if err != nil {
		return nil, err
	}
	if username != nil {
		u.Username = *username
	}
	if email != nil {
		u.Email = *email
	}
	if phone != nil {
		u.Phone = *phone
	}
	return &u, nil
}

// CreateUser creates a user with a pre-hashed password and category grants.
// The row's created_at is read back from the INSERT because omitempty does not
// omit a zero time.Time, so an unset value would serialize as year 1.
func (s *Store) CreateUser(ctx context.Context, u *model.User, passwordHash string, categoryIDs []string) error {
	err := dbTx(ctx, s, func(q queryer) error {
		if err := q.QueryRow(ctx, `
			INSERT INTO users (id, username, email, phone, name, password_hash, is_admin, created_at)
			VALUES ($1, NULLIF($2, ''), $3, NULLIF($4, ''), $5, $6, $7, now())
			RETURNING created_at`,
			u.ID, u.Username, u.Email, u.Phone, u.Name, passwordHash, u.IsAdmin).Scan(&u.CreatedAt); err != nil {
			return err
		}
		return setUserCategories(ctx, q, u.ID, categoryIDs)
	})
	if isUniqueViolation(err) {
		return ErrDuplicate
	}
	return err
}

// GetUserByUsernameOrEmail resolves an admin login credential: username or
// e-mail (both are valid admin identifiers) plus the password hash to check.
func (s *Store) GetUserByUsernameOrEmail(ctx context.Context, credential string) (*model.User, string, error) {
	u, err := scanUser(s.pool.QueryRow(ctx,
		"SELECT "+userCols+" FROM users WHERE username=$1 OR (email=$1 AND email <> '')", credential))
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

// GetUserByPhone finds a client account by its normalized phone number. The
// password hash is not loaded: a phone is a client credential, not a password.
func (s *Store) GetUserByPhone(ctx context.Context, phone string) (*model.User, error) {
	u, err := scanUser(s.pool.QueryRow(ctx,
		"SELECT "+userCols+" FROM users WHERE phone=$1", phone))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	return u, err
}

func (s *Store) GetUser(ctx context.Context, id string) (*model.User, error) {
	u, err := scanUser(s.pool.QueryRow(ctx, "SELECT "+userCols+" FROM users WHERE id=$1", id))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	return u, err
}

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
	Phone         *string
	Username      *string
	Email         *string
	PasswordHash  string
	IsAdmin       *bool
	CategoryIDs   []string
	SetCategories bool
}

// UpdateUser applies a partial update; nil pointer fields are ignored, a
// pointer (even "") sets/clears the value (needed when switching a user
// between admin and client). Password/categories are explicit.
func (s *Store) UpdateUser(ctx context.Context, id string, p UserPatch) error {
	err := dbTx(ctx, s, func(q queryer) error {
		if p.Name != "" {
			if _, err := q.Exec(ctx, "UPDATE users SET name=$2 WHERE id=$1", id, p.Name); err != nil {
				return err
			}
		}
		if p.Phone != nil {
			if _, err := q.Exec(ctx, "UPDATE users SET phone=NULLIF($2, '') WHERE id=$1", id, *p.Phone); err != nil {
				return err
			}
		}
		if p.Username != nil {
			if _, err := q.Exec(ctx, "UPDATE users SET username=NULLIF($2, '') WHERE id=$1", id, *p.Username); err != nil {
				return err
			}
		}
		if p.Email != nil {
			if _, err := q.Exec(ctx, "UPDATE users SET email=$2 WHERE id=$1", id, *p.Email); err != nil {
				return err
			}
		}
		if p.IsAdmin != nil {
			if _, err := q.Exec(ctx, "UPDATE users SET is_admin=$2 WHERE id=$1", id, *p.IsAdmin); err != nil {
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
	if isUniqueViolation(err) {
		return ErrDuplicate
	}
	return err
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
	rows, err := s.pool.Query(ctx, "SELECT "+categoryCols+` FROM user_categories uc
		JOIN categories c ON c.id = uc.category_id
		WHERE uc.user_id=$1 ORDER BY c.name`, userID)
	if err != nil {
		return nil, err
	}
	return collectCategories(rows)
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

func (s *Store) HasAdmin(ctx context.Context) (bool, error) {
	var ok bool
	err := s.pool.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM users WHERE is_admin)").Scan(&ok)
	return ok, err
}

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
