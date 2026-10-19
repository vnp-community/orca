package usecase

import (
	"context"

	"github.com/stablyai/orca-go/services/request-service/internal/domain"
)

// inheritedFeedback is the feedback of the latest analysis run of this kind when it failed: a failed revision leaves the
// Request in analyzing with the old proposal already superseded, so a plain Regenerate must carry the ask forward instead
// of silently dropping it. A running run is left alone (StartRun returns it), and a finished one has nothing to retry.
func inheritedFeedback(ctx context.Context, runs AnalysisRunStore, requestID string, kind domain.SolutionKind) (string, error) {
	recent, err := runs.ListRecent(ctx, requestID, domain.RecentRunsLimit)
	if err != nil {
		return "", err
	}
	for _, run := range recent {
		if run.Kind != domain.RunKind(kind) {
			continue
		}
		if run.Status == domain.RunStatusFailed {
			return run.Feedback, nil
		}
		return "", nil
	}
	return "", nil
}

// refuseSecondRevision reports a change request that StartRun answered with somebody else's running run. A retry carrying
// the same idempotency key is the same call and gets that run back; system starts keep the old quiet reuse.
func refuseSecondRevision(in GenerateSolutionInput, system bool, status domain.RequestStatus, existing domain.AnalysisRun) error {
	if system || in.Feedback == "" {
		return nil
	}
	if in.IdempotencyKey != "" && existing.IdempotencyKey != nil && *existing.IdempotencyKey == in.IdempotencyKey {
		return nil
	}
	return domain.ErrSolutionRevisionInProgress(status)
}
