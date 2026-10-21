package usecase

import (
	"context"
	"time"

	"github.com/stablyai/orca-go/services/workflow-service/internal/domain"
)

// StepApprovalRepository stores the decisions workflow steps wait for (migration 0014, Postgres and MySQL). The step that waits
// uses Upsert, Latest and MarkSubmitted; the person's side (StepApprovals) uses List, Get and Resolve.
type StepApprovalRepository interface {
	// UpsertPendingStepApproval records the decision a step waits for; the same execution and step keep one row.
	UpsertPendingStepApproval(ctx context.Context, a domain.StepApproval) (domain.StepApproval, error)
	GetStepApproval(ctx context.Context, tenantID, id string) (domain.StepApproval, error)
	LatestStepApproval(ctx context.Context, tenantID, executionID, stepID string) (domain.StepApproval, error)
	ListPendingStepApprovals(ctx context.Context, tenantID, projectID, executionID string, limit int) ([]domain.StepApproval, error)
	// ResolveStepApproval stores the person's decision; false means someone resolved it first.
	ResolveStepApproval(ctx context.Context, a domain.StepApproval) (bool, error)
	MarkStepApprovalSubmitted(ctx context.Context, tenantID, id string, at time.Time) error
}
