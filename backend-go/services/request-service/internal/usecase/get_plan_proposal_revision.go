package usecase

import (
	"context"

	"github.com/stablyai/orca-go/services/request-service/internal/domain"
)

// executeRevision answers for one numbered version of the Plan. The number is the run's place among the Request's
// successful Plan runs, so it works for the native engine too, whose proposal is on the run itself.
func (uc *GetPlanProposal) executeRevision(ctx context.Context, req domain.Request, auto *PlanAutoView, revision int) (PlanProposalView, error) {
	runs, err := uc.runs.ListSucceeded(ctx, req.ID, domain.RunKindPlan)
	if err != nil {
		return PlanProposalView{}, err
	}
	if revision < 1 || revision > len(runs) {
		return PlanProposalView{}, domain.ErrPlanRevisionNotFound(revision)
	}
	run := runs[revision-1]
	proposal, err := DecodePlanProposal(derefStr(run.RawOutput))
	if err != nil {
		return PlanProposalView{}, err
	}
	view := PlanProposalView{RunID: run.ID, Status: string(run.Status), Engine: string(run.Mode.EngineFor()), Stats: run.Stats, AutoPlan: auto, Proposal: &proposal}
	if run.FinishedAt != nil {
		view.ElapsedMS = run.FinishedAt.Sub(run.StartedAt).Milliseconds()
	}
	if run.Mode != domain.AnalysisModeAgentProposal || uc.changes == nil {
		return view, nil
	}
	current := revision == len(runs)
	if err := uc.addRevisionChange(ctx, &view, req.ID, revision, run.ID, current); err != nil {
		return PlanProposalView{}, err
	}
	return view, nil
}

// addRevisionChange attaches the files of one version. Checks and knowledge are columns of the current change row, not
// copied per version, so only the newest version has them.
func (uc *GetPlanProposal) addRevisionChange(ctx context.Context, view *PlanProposalView, requestID string, revision int, runID string, current bool) error {
	change, found, err := uc.changes.GetByRequest(ctx, requestID)
	if err != nil || !found {
		return err
	}
	var row domain.OpenSpecChangeRevision
	stored := false
	if uc.revisions != nil {
		if row, stored, err = uc.revisions.GetByRun(ctx, runID); err != nil {
			return err
		}
	}
	switch {
	case stored:
		var prev []domain.RevisionFile
		hasPrev := false
		if revision > 1 {
			before, ok, err := uc.revisions.Get(ctx, requestID, revision-1)
			if err != nil {
				return err
			}
			prev, hasPrev = before.Files, ok
		}
		view.Bundle = bundleViewOf(change, row.Commit, row.TasksMDDigest, row.Files, prev, hasPrev)
	case current:
		// The newest version of a Request that predates the history: the change row is the version.
		if err := uc.addChange(ctx, view, requestID); err != nil {
			return err
		}
		return nil
	default:
		return nil // an older version of such a Request has no stored documents
	}
	if current {
		view.Knowledge = &PlanKnowledgeView{Available: change.KGAvailable, Snapshot: change.KGSnapshot, Read: change.KGReadIDs, Gaps: change.KGGaps, Conflicts: change.KGConflicts, SkillHash: change.SkillHash}
		view.Checks = planChecksFor(change)
	}
	return nil
}
