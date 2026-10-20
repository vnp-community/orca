package usecase

import (
	"context"
	"time"

	"github.com/stablyai/orca-go/services/request-service/internal/domain"
)

// PlanAutoJobRepository stores the automatic Plan generation of each request. Every method except ClaimDue is scoped to
// the tenant in ctx; ClaimDue spans tenants because the worker serves all of them.
type PlanAutoJobRepository interface {
	// Insert stores a new job. A job with the same (request, anchor) or a second active job for the request is not an
	// error: it returns inserted=false and changes nothing.
	Insert(ctx context.Context, job domain.PlanAutoJob) (inserted bool, err error)
	// LatestByRequest is the job created last for the request, whatever its state.
	LatestByRequest(ctx context.Context, requestID string) (domain.PlanAutoJob, bool, error)
	// ActiveByRequest is the request's pending, running or committing job.
	ActiveByRequest(ctx context.Context, requestID string) (domain.PlanAutoJob, bool, error)
	// ClaimDue leases up to batch active jobs whose next_check_at has passed and whose lease is free or expired.
	// Concurrent callers never receive the same job.
	ClaimDue(ctx context.Context, owner string, ttl time.Duration, batch int) ([]domain.PlanAutoJob, error)
	// Update writes the job's progress (state, attempt, run, reason, commit attempts, schedule) and releases the lease.
	// It returns false, and writes nothing, when owner no longer holds the lease.
	Update(ctx context.Context, job domain.PlanAutoJob, owner string) (bool, error)
}
