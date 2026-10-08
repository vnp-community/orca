package postgres

import (
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5/pgconn"

	"github.com/stablyai/orca-go/services/task-service/internal/usecase"
)

const activePlanIndex = "uq_tasks_active_plan_per_request"

// IsUniqueViolation reports a Postgres unique-constraint failure (SQLSTATE 23505).
func IsUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}

// wrapActivePlanViolation turns a duplicate active Plan into usecase.ErrActivePlanExists
// so CreatePlanTree can re-read the winner instead of failing.
func wrapActivePlanViolation(err error) error {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" && pgErr.ConstraintName == activePlanIndex {
		return fmt.Errorf("%w: %w", usecase.ErrActivePlanExists, err)
	}
	return err
}

// markTxConflict tags serialization failures and deadlocks (40001, 40P01) so callers may retry the whole tx.
func markTxConflict(err error) error {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && (pgErr.Code == "40001" || pgErr.Code == "40P01") {
		return fmt.Errorf("%w: %w", usecase.ErrTxConflict, err)
	}
	return err
}
