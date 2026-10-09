package usecase

import (
	"context"

	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/request-service/internal/domain"
)

// PlanProposalView is the state of one Plan generation run as the caller reads it.
type PlanProposalView struct {
	RunID string
	// Status is running, succeeded or failed.
	Status       string
	ErrorCode    string
	ErrorMessage string
	Proposal     *domain.PlanProposal
	// Raw is the model's text when the run failed, so a person can see what it said.
	Raw string
}

// GetPlanProposal reads the result of a GeneratePlan run; with no run id it answers for the newest run of the Request.
type GetPlanProposal struct {
	requests RequestRepository
	runs     AnalysisRunStore
}

func NewGetPlanProposal(requests RequestRepository, runs AnalysisRunStore) *GetPlanProposal {
	return &GetPlanProposal{requests: requests, runs: runs}
}

func (uc *GetPlanProposal) Execute(ctx context.Context, requestID, runID string) (PlanProposalView, error) {
	if _, err := tenant.RequireTenantID(ctx); err != nil {
		return PlanProposalView{}, domain.ErrRequestTenantRequired()
	}
	req, err := uc.requests.Get(ctx, requestID)
	if err != nil {
		return PlanProposalView{}, err
	}
	run, err := uc.find(ctx, req.ID, runID)
	if err != nil {
		return PlanProposalView{}, err
	}
	view := PlanProposalView{RunID: run.ID, Status: string(run.Status)}
	switch run.Status {
	case domain.RunStatusFailed:
		view.ErrorCode, view.ErrorMessage = derefStr(run.ErrorCode), derefStr(run.ErrorMessage)
		view.Raw = derefStr(run.RawOutput)
	case domain.RunStatusSucceeded:
		p, err := DecodePlanProposal(derefStr(run.RawOutput))
		if err != nil {
			return PlanProposalView{}, err
		}
		view.Proposal = &p
	}
	return view, nil
}

func (uc *GetPlanProposal) find(ctx context.Context, requestID, runID string) (domain.AnalysisRun, error) {
	if runID != "" {
		run, err := uc.runs.Get(ctx, runID)
		if err != nil && !isNotFoundKind(err) {
			return domain.AnalysisRun{}, err
		}
		if err != nil || run.RequestID != requestID || run.Kind != domain.RunKindPlan {
			return domain.AnalysisRun{}, domain.ErrPlanRunNotFound(runID)
		}
		return run, nil
	}
	recent, err := uc.runs.ListRecent(ctx, requestID, domain.RecentRunsLimit)
	if err != nil {
		return domain.AnalysisRun{}, err
	}
	for _, run := range recent {
		if run.Kind == domain.RunKindPlan {
			return run, nil
		}
	}
	return domain.AnalysisRun{}, domain.ErrPlanRunNotFound("")
}

func derefStr(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}
