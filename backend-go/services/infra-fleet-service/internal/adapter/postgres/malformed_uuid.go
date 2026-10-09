package postgres

import (
	"errors"

	"github.com/jackc/pgx/v5/pgconn"
)

// isMalformedUUID reports Postgres rejecting a client-supplied id at bind time (SQLSTATE 22P02).
// An id that cannot be a uuid names nothing, so lookups answer "not found" instead of Internal.
func isMalformedUUID(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "22P02"
}
