package usecase

import (
	"context"
	"fmt"

	"github.com/stablyai/orca-go/services/request-service/internal/domain"
)

// appendRevision records the change the run just wrote as the next version of the Plan, inside the transaction that stores
// the change: a change row without its version row would hide it from the history. The number counts the Plan runs that
// succeeded before this one; they run one at a time (analysis_runs_one_running), so it cannot race, and the unique
// (request, revision) key stops a mistake from writing two versions with one number.
func (w *RunProposalGeneration) appendRevision(ctx context.Context, run domain.AnalysisRun, change domain.OpenSpecChange) error {
	if w.revisions == nil {
		return fmt.Errorf("the change revision repository is not configured")
	}
	done, err := w.runs.ListSucceeded(ctx, run.RequestID, domain.RunKindPlan)
	if err != nil {
		return err
	}
	return w.revisions.Append(ctx, domain.OpenSpecChangeRevision{
		ID: w.newID(), RequestID: run.RequestID, Revision: len(done) + 1, RunID: run.ID, Commit: change.Commit,
		TasksMDDigest: change.TasksMDDigest, BundleDigest: change.BundleDigest(), Files: domain.NewRevisionFiles(change.Bundle),
		Bundle: change.Bundle, CreatedAt: w.now(),
	}, domain.PlanRevisionTextKept)
}
