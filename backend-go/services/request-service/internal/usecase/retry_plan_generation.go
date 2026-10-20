package usecase

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/request-service/internal/domain"
)

// RetryPlanGeneration is the person's way back after an automatic Plan failed or gave up (CR-REQ-044). It queues a job
// for the worker instead of doing the work, so it answers at once and the same worker, with the same rules, does the rest.
type RetryPlanGeneration struct {
	requests RequestRepository
	runs     AnalysisRunStore
	jobs     PlanAutoJobRepository
	auth     PlanActorAuthorizer
	sw       PlanAutoSwitch
	now      func() time.Time
	newID    func() string
}

func NewRetryPlanGeneration(requests RequestRepository, runs AnalysisRunStore, jobs PlanAutoJobRepository, auth PlanActorAuthorizer, sw PlanAutoSwitch) *RetryPlanGeneration {
	return &RetryPlanGeneration{requests: requests, runs: runs, jobs: jobs, auth: auth, sw: sw,
		now: func() time.Time { return time.Now().UTC() }, newID: uuid.NewString}
}

func (uc *RetryPlanGeneration) Execute(ctx context.Context, requestID string) (PlanAutoView, error) {
	tenantID, err := tenant.RequireTenantID(ctx)
	if err != nil {
		return PlanAutoView{}, domain.ErrRequestTenantRequired()
	}
	actor, _ := tenant.UserID(ctx)
	if actor == "" {
		return PlanAutoView{}, domain.ErrRequestReporterRequired()
	}
	req, err := uc.requests.Get(ctx, requestID)
	if err != nil {
		return PlanAutoView{}, err
	}
	if err := uc.auth.AuthorizePlan(ctx, req); err != nil {
		return PlanAutoView{}, err
	}
	if !uc.sw.GloballyAvailable() {
		return PlanAutoView{}, domain.ErrPlanAutoDisabled()
	}
	if req.Status != domain.RequestStatusPlanning {
		return PlanAutoView{}, domain.ErrPlanWrongState(req.Status)
	}
	// Pressing Retry twice, or while a job is working, shows that job instead of starting another.
	if job, found, err := uc.jobs.ActiveByRequest(ctx, req.ID); err != nil || found {
		return uc.view(ctx, job, req), err
	}
	job, err := uc.newJob(ctx, tenantID, actor, req)
	if err != nil {
		return PlanAutoView{}, err
	}
	inserted, err := uc.jobs.Insert(ctx, job)
	if err != nil {
		return PlanAutoView{}, err
	}
	if !inserted {
		// A job appeared between the check and the insert: the one-active-job rule turned this call into a read of it.
		if active, found, err := uc.jobs.ActiveByRequest(ctx, req.ID); err != nil || found {
			return uc.view(ctx, active, req), err
		}
	}
	return uc.view(ctx, job, req), nil
}

// newJob continues from the commit when the last run already produced a proposal (the AI is not paid for twice),
// otherwise it asks for one run, with no automatic extra attempt: the person is watching.
func (uc *RetryPlanGeneration) newJob(ctx context.Context, tenantID, actor string, req domain.Request) (domain.PlanAutoJob, error) {
	now := uc.now()
	job := domain.PlanAutoJob{
		TenantID: tenantID, RequestID: req.ID, Anchor: domain.RetryAnchor(uc.newID()), Origin: domain.PlanAutoOriginRetry, ProjectID: req.ProjectID,
		State: domain.PlanAutoPending, Attempt: 1, MaxAttempts: 1, NextCheckAt: now, RequestedBy: actor, CreatedAt: now, UpdatedAt: now,
	}
	recent, err := uc.runs.ListRecent(ctx, req.ID, planAutoRecentRuns)
	if err != nil {
		return domain.PlanAutoJob{}, err
	}
	for _, run := range recent {
		if run.Kind != domain.RunKindPlan {
			continue
		}
		if run.Status == domain.RunStatusSucceeded && req.PlanTaskID == "" {
			job.State, job.RunID = domain.PlanAutoCommitting, run.ID
		}
		break
	}
	return job, nil
}

func (uc *RetryPlanGeneration) view(ctx context.Context, job domain.PlanAutoJob, req domain.Request) PlanAutoView {
	enabled, err := uc.sw.Enabled(ctx, req.ProjectID)
	return planAutoViewOf(job, err == nil && enabled)
}
