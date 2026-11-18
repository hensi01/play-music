package store

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// UnknownArtist is the placeholder name used for songs, albums and artists
// that carry no tag. It is stored in the catalogue, so every writer and every
// reader that special-cases it must use this value.
const UnknownArtist = "Desconhecido"

// IgnoredGenre is the genre tag that must never become a category. The
// catalogue has no category with this name and must not grow one on rescan:
// migration 0008 deleted it permanently because it carried no meaning.
const IgnoredGenre = "music"

// The failure vocabulary the store hands to the server layer. Each one maps to
// a distinct HTTP status in handleStoreError.
var (
	ErrNotFound     = errors.New("not found")
	ErrForbidden    = errors.New("forbidden")
	ErrDuplicate    = errors.New("duplicate")
	ErrOwnedByOther = errors.New("owned by another user")
	ErrStale        = errors.New("stale")
	ErrNotPending   = errors.New("not pending")
)

func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}

type queryer interface {
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

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

// scanner is the single destination a query row is read into; both pgx.Row and
// pgx.Rows satisfy it.
type scanner interface {
	Scan(dest ...any) error
}

// collect always returns a non-nil slice so an empty result marshals as [] and
// not null.
func collect[T any](rows pgx.Rows, scan func(scanner) (T, error)) ([]T, error) {
	defer rows.Close()
	out := []T{}
	for rows.Next() {
		v, err := scan(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, rows.Err()
}

// idsOf projects the ID out of a slice of entities, in order.
func idsOf[T any](items []T, id func(T) string) []string {
	ids := make([]string, len(items))
	for i := range items {
		ids[i] = id(items[i])
	}
	return ids
}

func notFoundIfEmpty(tag pgconn.CommandTag, err error) error {
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}
