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
	out.Runs, err = uc.runs.ListRecent(ctx, req.ID, domain.RecentRunsLimit)
	return out, err
}
