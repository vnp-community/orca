package usecase

import (
	"context"
	"time"

	"github.com/stablyai/orca-go/services/task-service/internal/domain"
)

// TaskSpecRepository persists task.task_specs. Adapters return the domain.ErrTaskSpec* errors
// and always filter by tenant.
type TaskSpecRepository interface {
	// Upsert writes s when the stored version equals expectedVersion (0 = no row yet) and the
	// row is not locked; otherwise ErrTaskSpecLocked, ErrTaskSpecVersionConflict or ErrTaskSpecNotFound
	// (the task itself is gone).
	Upsert(ctx context.Context, s domain.TaskSpec, expectedVersion int64) (domain.TaskSpec, error)
	// GetMany returns the specs that exist for taskIDs (max 200 per call), in no fixed order.
	GetMany(ctx context.Context, tenantID string, taskIDs []string) ([]domain.TaskSpec, error)
	// LockSubtree locks every unlocked spec among taskIDs and reports how many it changed.
	LockSubtree(ctx context.Context, tenantID string, taskIDs []string, at time.Time) (int, error)
	IsLocked(ctx context.Context, tenantID, taskID string) (bool, error)
	HasSpec(ctx context.Context, tenantID, taskID string) (bool, error)
}

// TaskSpecLookup is the narrow view ExecuteTask needs to route spec tasks to Engine 1.
type TaskSpecLookup interface {
	HasSpec(ctx context.Context, tenantID, taskID string) (bool, error)
}

// TaskSpecLockChecker is the narrow view UpdateTask needs to freeze titles.
type TaskSpecLockChecker interface {
	IsLocked(ctx context.Context, tenantID, taskID string) (bool, error)
}

// SpecTxRunner is TxRunner plus a TaskSpecRepository in the same transaction. A separate method
// (not a wider RunInTx) so AIApply and every existing RunInTx caller keep compiling untouched.
type SpecTxRunner interface {
	RunInTxWithSpecs(ctx context.Context, fn func(ctx context.Context, tasks TaskRepository, edges EdgeRepository, specs TaskSpecRepository) error) error
}
