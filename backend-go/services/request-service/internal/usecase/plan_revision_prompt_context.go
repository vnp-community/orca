package usecase

import (
	"context"
	"sort"

	"github.com/stablyai/orca-go/services/request-service/internal/domain"
)

// maxPreviousChangeFiles bounds the file list a revision prompt carries; the bundle itself holds at most this many.
const maxPreviousChangeFiles = 200

// previousChangeFiles names the files of the version a regenerate will edit. The worktree is reset to the branch tip before
// the agent runs, so these files are on disk only for a change that finished (ready); any other state starts from nothing.
func previousChangeFiles(existing domain.OpenSpecChange, hasChange bool) []string {
	if !hasChange || existing.Status != domain.ChangeStatusReady || len(existing.Bundle) == 0 {
		return nil
	}
	names := make([]string, 0, len(existing.Bundle))
	for p := range existing.Bundle {
		names = append(names, p)
	}
	sort.Strings(names)
	if len(names) > maxPreviousChangeFiles {
		names = names[:maxPreviousChangeFiles]
	}
	return names
}

// priorPlanText is the Plan the last succeeded run produced, as stored (already masked), or "" when this is the first. A
// failed or cancelled run never replaces it: the model revises the version the reviewer saw.
func priorPlanText(ctx context.Context, runs AnalysisRunStore, requestID string) (string, error) {
	done, err := runs.ListSucceeded(ctx, requestID, domain.RunKindPlan)
	if err != nil || len(done) == 0 {
		return "", err
	}
	return derefStr(done[len(done)-1].RawOutput), nil
}
