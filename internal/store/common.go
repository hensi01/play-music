package store

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

var ErrNotFound = errors.New("not found")

// ErrForbidden is returned when the user has no access to the entity or owns
// neither the resource (e.g. someone else's playlist).
var ErrForbidden = errors.New("forbidden")

// ErrDuplicate is returned when a write violates a unique constraint
// (username, e-mail or phone already taken).
var ErrDuplicate = errors.New("duplicate")

// isUniqueViolation reports whether err is a Postgres unique constraint
// violation (SQLSTATE 23505), even when wrapped.
func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}

type queryer interface {
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

// dbTx runs fn inside a transaction on a queryer interface.
func dbTx(ctx context.Context, s *Store, fn func(q queryer) error) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(context.Background())
	if err := fn(tx); err != nil {
		return err
	}
	return tx.Commit(ctx)
}
