package postgres

import (
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// isMalformedUUID reports Postgres rejecting a caller-supplied id at bind time (22P02).
func isMalformedUUID(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "22P02"
}

// isNoRows treats an id that cannot be a uuid like a missing row: it names nothing, so lookups
// answer not-found instead of Internal.
func isNoRows(err error) bool {
	return errors.Is(err, pgx.ErrNoRows) || isMalformedUUID(err)
}
