package usecase

import (
	"context"
	"errors"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/task-service/internal/domain"
)

// ErrInvalidArgument is what adapters return for caller mistakes such as an empty task id list.
var ErrInvalidArgument = errors.New("usecase: invalid argument")

type ListExecutionRecordsInput struct {
	TaskIDs    []string
	LatestOnly bool
	Limit      int
}

// ListExecutionRecords returns contract-run records. A user caller needs read access on every
// task asked for; a sibling-service call (no user identity) is tenant-scoped only, see requireGrantWhenUser.
type ListExecutionRecords struct {
	records TaskExecutionRecordRepository
	perm    taskPermissionChecker
}

func NewListExecutionRecords(records TaskExecutionRecordRepository, perm taskPermissionChecker) *ListExecutionRecords {
	return &ListExecutionRecords{records: records, perm: perm}
}

func (uc *ListExecutionRecords) Execute(ctx context.Context, in ListExecutionRecordsInput) ([]domain.ExecutionRecord, error) {
	tenantID, err := tenant.RequireTenantID(ctx)
	if err != nil {
		return nil, apperrors.New(apperrors.KindUnauthenticated, "TASK_NO_TENANT", "no tenant in request context", err)
	}
	if len(in.TaskIDs) < 1 || len(in.TaskIDs) > MaxExecutionRecordLimit {
		return nil, apperrors.New(apperrors.KindInvalidArgument, "TASK_EXECUTION_RECORD_BAD_REQUEST", "task_ids must hold between 1 and 200 ids", nil)
	}
	seen := make(map[string]bool, len(in.TaskIDs))
	ids := make([]string, 0, len(in.TaskIDs))
	for _, id := range in.TaskIDs {
		if id == "" || seen[id] {
			continue
		}
		seen[id] = true
		ids = append(ids, id)
	}
	if len(ids) == 0 {
		return nil, apperrors.New(apperrors.KindInvalidArgument, "TASK_EXECUTION_RECORD_BAD_REQUEST", "task_ids must not be empty", nil)
	}
	for _, id := range ids {
		if err := requireGrantWhenUser(ctx, uc.perm, id, "read"); err != nil {
			return nil, err
		}
	}
	recs, err := uc.records.ListExecutionRecords(ctx, tenantID, ids, in.LatestOnly, in.Limit)
	if err != nil {
		if errors.Is(err, ErrInvalidArgument) {
			return nil, apperrors.New(apperrors.KindInvalidArgument, "TASK_EXECUTION_RECORD_BAD_REQUEST", "invalid list request", err)
		}
		return nil, apperrors.New(apperrors.KindInternal, "TASK_EXECUTION_RECORD_LIST_FAILED", "failed to list execution records", err)
	}
	return recs, nil
}
