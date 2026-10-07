package usecase

import (
	"context"
	"time"

	"github.com/stablyai/orca-go/common/apperrors"
	codeintelv1 "github.com/stablyai/orca-go/proto/gen/go/orca/codeintel/v1"
)

const DefaultReindexPollAfter = 10 * time.Second

// GetReindexJobUseCase retrieves reindex job status with staleness refreshing (§2.F).
type GetReindexJobUseCase struct {
	resolver  WorktreeTargetResolver
	store     ReindexJobStore
	refresher ReindexJobRefresher
	pollAfter time.Duration
}

// NewGetReindexJobUseCase creates a new GetReindexJobUseCase.
func NewGetReindexJobUseCase(
	resolver WorktreeTargetResolver,
	store ReindexJobStore,
	refresher ReindexJobRefresher,
	pollAfter time.Duration,
) *GetReindexJobUseCase {
	if pollAfter <= 0 {
		pollAfter = DefaultReindexPollAfter
	}
	return &GetReindexJobUseCase{
		resolver:  resolver,
		store:     store,
		refresher: refresher,
		pollAfter: pollAfter,
	}
}

// Execute retrieves the job, validates tenant/selector binding ownership, and refreshes if stale.
func (u *GetReindexJobUseCase) Execute(ctx context.Context, tenantID string, req *codeintelv1.GetReindexJobRequest) (*ReindexJob, error) {
	if req == nil || req.Selector == nil || req.JobId == "" {
		return nil, apperrors.New(apperrors.KindInvalidArgument, "CODEINTEL_INVALID_PARAMS", "selector and job_id are required", nil)
	}

	target, expectedBindingID, err := u.resolver.ResolveTarget(ctx, tenantID, req.Selector)
	if err != nil {
		return nil, err
	}

	job, err := u.store.Get(ctx, tenantID, req.JobId)
	if err != nil || job == nil {
		return nil, apperrors.New(apperrors.KindNotFound, "CODEINTEL_NOT_FOUND", "reindex job not found", err)
	}

	// Verify the job belongs to the resolved repo binding and tenant
	if job.TenantID != tenantID || (expectedBindingID != "" && job.RepoBindingID != expectedBindingID) {
		return nil, apperrors.New(apperrors.KindNotFound, "CODEINTEL_NOT_FOUND", "reindex job does not belong to specified selector", nil)
	}

	// If job is still queued/running and older than pollAfter, poll agent status to refresh
	if (job.Status == "queued" || job.Status == "running") && u.refresher != nil {
		if time.Since(job.UpdatedAt) >= u.pollAfter {
			job, _ = u.refresher.Refresh(ctx, target, job)
		}
	}

	return job, nil
}
