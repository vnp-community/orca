package usecase

import (
	"context"

	"github.com/stablyai/orca-go/services/request-service/internal/domain"
)

// inheritedPlanFeedback is the feedback of the latest Plan run when it failed: a failed revision leaves the Request in
// planning with its approval already cancelled, so a plain Regenerate must carry the ask forward instead of silently
// dropping it. A running or finished run has nothing to retry.
func inheritedPlanFeedback(ctx context.Context, runs AnalysisRunStore, requestID string) (string, error) {
	recent, err := runs.ListRecent(ctx, requestID, domain.RecentRunsLimit)
	if err != nil {
		return "", err
	}
	for _, run := range recent {
		if run.Kind != domain.RunKindPlan {
			continue
		}
		if run.Status == domain.RunStatusFailed {
			return run.Feedback, nil
		}
		return "", nil
	}
	return "", nil
}

// refuseSecondPlanRevision reports a change request that StartRun answered with somebody else's running run. A retry
// carrying the same idempotency key is the same call and gets that run back; a start without feedback keeps the quiet reuse.
func refuseSecondPlanRevision(in GeneratePlanInput, existing domain.AnalysisRun) error {
	if in.Feedback == "" {
		return nil
	}
	if in.IdempotencyKey != "" && existing.IdempotencyKey != nil && *existing.IdempotencyKey == in.IdempotencyKey {
		return nil
	}
	return domain.ErrPlanRevisionInProgress()
}

// isPlanRevision tells a change to a Plan that exists (feedback at the gate, or a retry in planning) from the first Plan.
func isPlanRevision(req domain.Request, feedback string) bool {
	if req.PlanTaskID == "" {
		return false
	}
	return (req.Status == domain.RequestStatusAwaitingPlanApproval && feedback != "") || req.Status == domain.RequestStatusPlanning
}
