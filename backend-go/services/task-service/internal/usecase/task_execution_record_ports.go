package usecase

import (
	"context"

	"github.com/stablyai/orca-go/services/task-service/internal/domain"
)

const (
	DefaultExecutionRecordLimit = 50
	MaxExecutionRecordLimit     = 200
)

// TaskExecutionRecordRepository persists task.task_execution_records. Method names carry the
// entity because one adapter struct implements many ports.
type TaskExecutionRecordRepository interface {
	// InsertExecutionRecord stores rec (a new id is minted when empty) and returns it with id and created_at set.
	InsertExecutionRecord(ctx context.Context, rec domain.ExecutionRecord) (domain.ExecutionRecord, error)
	// ListExecutionRecords returns newest first; latestOnly keeps one row per task. taskIDs must be
	// non-empty (ErrInvalidArgument) and limit is clamped to [1,200], default 50.
	ListExecutionRecords(ctx context.Context, tenantID string, taskIDs []string, latestOnly bool, limit int) ([]domain.ExecutionRecord, error)
}
