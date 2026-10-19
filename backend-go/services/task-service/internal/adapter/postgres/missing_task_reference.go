package postgres

import (
	"errors"

	"github.com/jackc/pgx/v5/pgconn"
)

// isMalformedUUID reports Postgres rejecting an id at bind time (SQLSTATE 22P02). An id that cannot be a uuid
// names no row, so lookups answer "not found" like any unknown id instead of an Internal failure.
func isMalformedUUID(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "22P02"
}

// isMissingTaskReference reports an insert naming a task that does not exist: FK violation (23503)
// or an id that is not a uuid (22P02).
func isMissingTaskReference(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && (pgErr.Code == "23503" || pgErr.Code == "22P02")
}
