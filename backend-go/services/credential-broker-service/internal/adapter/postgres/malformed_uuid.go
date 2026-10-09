package postgres

import (
	"errors"

	"github.com/jackc/pgx/v5/pgconn"
)

// isNoMatch is true for an id that cannot be a uuid (SQLSTATE 22P02): it names no credential, so
// lookups answer "not found" like any unknown id instead of an Internal failure.
func isNoMatch(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "22P02"
}
