package mysql

import (
	"errors"
	"fmt"
	"strings"

	"github.com/go-sql-driver/mysql"

	"github.com/stablyai/orca-go/services/task-service/internal/usecase"
)

const activePlanIndex = "uq_tasks_active_plan_per_request"

// IsUniqueViolation reports a MySQL duplicate-key failure (error 1062).
func IsUniqueViolation(err error) bool {
	var myErr *mysql.MySQLError
	return errors.As(err, &myErr) && myErr.Number == 1062
}

// wrapActivePlanViolation turns a duplicate active Plan into usecase.ErrActivePlanExists.
// MySQL only names the index in the message text, so match on it.
func wrapActivePlanViolation(err error) error {
	var myErr *mysql.MySQLError
	if errors.As(err, &myErr) && myErr.Number == 1062 && strings.Contains(myErr.Message, activePlanIndex) {
		return fmt.Errorf("%w: %w", usecase.ErrActivePlanExists, err)
	}
	return err
}

// markTxConflict tags deadlock / lock-wait failures (1213, 1205) so callers may retry the whole tx.
func markTxConflict(err error) error {
	var myErr *mysql.MySQLError
	if errors.As(err, &myErr) && (myErr.Number == 1213 || myErr.Number == 1205) {
		return fmt.Errorf("%w: %w", usecase.ErrTxConflict, err)
	}
	return err
}
