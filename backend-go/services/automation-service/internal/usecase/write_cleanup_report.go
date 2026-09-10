package usecase

import (
	"context"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/automation-service/internal/domain"
)

// WriteCleanupReportInput mirrors WriteCleanupReportRequest
// (automation.proto) — the reverse-direction call
// workflow-service.CleanupWorktreesStepExecutor makes after a bulk-delete
// run, to record BR-AT-14's per-worktree audit trail.
type WriteCleanupReportInput struct {
	RunID   string
	Entries []domain.CleanupLogEntry
}

// WriteCleanupReport persists one worktree_cleanup_log row per entry.
type WriteCleanupReport struct {
	runs AutomationRunRepository
}

func NewWriteCleanupReport(runs AutomationRunRepository) *WriteCleanupReport {
	return &WriteCleanupReport{runs: runs}
}

func (uc *WriteCleanupReport) Execute(ctx context.Context, in WriteCleanupReportInput) error {
	tenantID, err := tenant.RequireTenantID(ctx)
	if err != nil {
		return apperrors.New(apperrors.KindUnauthenticated, "AUTOMATION_NO_TENANT", "no tenant in request context", err)
	}
	if in.RunID == "" {
		return apperrors.New(apperrors.KindInvalidArgument, "AUTOMATION_MISSING_RUN_ID", "run_id is required", nil)
	}
	if err := uc.runs.WriteCleanupReport(ctx, tenantID, in.RunID, in.Entries); err != nil {
		return apperrors.New(apperrors.KindInternal, "AUTOMATION_CLEANUP_REPORT_FAILED", "failed to persist cleanup report", err)
	}
	return nil
}
