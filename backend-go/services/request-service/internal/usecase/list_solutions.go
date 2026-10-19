package usecase

import (
	"context"

	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/request-service/internal/domain"
)

type ListSolutionsInput struct {
	RequestID string
	Kind      domain.SolutionKind
	Status    domain.SolutionStatus
	PageSize  int
}

type ListSolutionsResult struct {
	Solutions []domain.Solution
	// Runs are the latest analysis runs: the only way the UI learns that a run failed.
	Runs []domain.AnalysisRun
}

// ListSolutions hides drafts' placeholder content: a draft is an unfinished run, not something to show as a solution.
type ListSolutions struct {
	requests  RequestRepository
	solutions SolutionStore
	runs      AnalysisRunStore
}

func NewListSolutions(requests RequestRepository, solutions SolutionStore, runs AnalysisRunStore) *ListSolutions {
	return &ListSolutions{requests: requests, solutions: solutions, runs: runs}
}

func (uc *ListSolutions) Execute(ctx context.Context, in ListSolutionsInput) (ListSolutionsResult, error) {
	if _, err := tenant.RequireTenantID(ctx); err != nil {
		return ListSolutionsResult{}, domain.ErrRequestTenantRequired()
	}
	req, err := uc.requests.Get(ctx, in.RequestID)
	if err != nil {
		if isNotFound(err) {
			return ListSolutionsResult{}, domain.ErrSolutionRequestNotFound(in.RequestID)
		}
		return ListSolutionsResult{}, err
	}
	limit := in.PageSize
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	all, err := uc.solutions.ListByRequest(ctx, SolutionListFilter{RequestID: req.ID, Kind: in.Kind, Status: in.Status, Limit: limit})
	if err != nil {
		return ListSolutionsResult{}, err
	}
	out := ListSolutionsResult{}
	for _, s := range all {
		if s.Status == domain.SolutionStatusDraft && in.Status != domain.SolutionStatusDraft {
			continue
		}
		out.Solutions = append(out.Solutions, s)
	}
	if err := uc.fillRevisionFeedback(ctx, out.Solutions); err != nil {
		return ListSolutionsResult{}, err
	}
	runs, err := uc.runs.ListRecent(ctx, req.ID, domain.RecentRunsLimit)
	for _, run := range runs {
		if run.Kind != domain.RunKindPlan { // plan runs are read through GetPlanProposal
			out.Runs = append(out.Runs, run)
		}
	}
	return out, err
}

// fillRevisionFeedback sets RevisionFeedback on every version with one batched read of its generation runs, so a long
// history costs one query, not one per version.
func (uc *ListSolutions) fillRevisionFeedback(ctx context.Context, sols []domain.Solution) error {
	ids := make([]string, 0, len(sols))
	for _, s := range sols {
		ids = append(ids, s.GenerationRunID)
	}
	if len(ids) == 0 {
		return nil
	}
	feedback, err := uc.solutions.RunFeedbackByIDs(ctx, ids)
	if err != nil {
		return err
	}
	for i := range sols {
		sols[i].RevisionFeedback = feedback[sols[i].GenerationRunID]
	}
	return nil
}
