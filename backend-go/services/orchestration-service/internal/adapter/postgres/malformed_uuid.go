package postgres

import (
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// isNoRows treats a caller-supplied id Postgres rejects at bind time (22P02) like a missing row:
// an id that cannot be a uuid names nothing, so lookups answer NotFound instead of Internal.
func isNoRows(err error) bool {
	if errors.Is(err, pgx.ErrNoRows) {
		return true
	}
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "22P02"
}

// referencedRowMissing reports an insert whose foreign key points at a row that does not exist (23503)
// together with the constraint name, so callers can tell which reference was dangling.
func referencedRowMissing(err error) (constraint string, ok bool) {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23503" {
		return pgErr.ConstraintName, true
	}
	return "", false
}
