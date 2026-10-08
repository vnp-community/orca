package usecase

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/request-service/internal/domain"
)

func TestListSolutions_HidesDraftsAndReturnsRuns(t *testing.T) {
	w := newSolWorld()
	run, r := startRun(t, w, domain.RequestTypeChangeRequest, AnalysisConnection{ConnectionID: "c"})
	done := domain.Solution{ID: uuid.NewString(), TenantID: "t1", RequestID: r.ID, Kind: domain.SolutionKindSolution, Status: domain.SolutionStatusSuperseded,
		OptionsJSON: []byte(`{}`), Version: 2, CreatedAt: time.Now().Add(-time.Hour)}
	w.solutions[done.ID] = done
	failed := domain.AnalysisRun{ID: uuid.NewString(), TenantID: "t1", RequestID: r.ID, Kind: domain.RunKindSolution, Status: domain.RunStatusFailed, StartedAt: time.Now().Add(-time.Hour)}
	w.runs[failed.ID] = failed

	uc := NewListSolutions(w, fakeSolutions{w}, fakeRuns{w})
	res := must(uc.Execute(w.ctx(), ListSolutionsInput{RequestID: r.ID}))
	if len(res.Solutions) != 1 || res.Solutions[0].ID != done.ID {
		t.Fatalf("solutions = %+v (the draft is an unfinished run, not a solution)", res.Solutions)
	}
	if len(res.Runs) != 2 || res.Runs[0].ID != run.ID {
		t.Fatalf("runs = %+v", res.Runs)
	}
	filtered := must(uc.Execute(w.ctx(), ListSolutionsInput{RequestID: r.ID, Status: domain.SolutionStatusSuperseded}))
	if len(filtered.Solutions) != 1 {
		t.Fatalf("filter by status: %+v", filtered.Solutions)
	}
	none := must(uc.Execute(w.ctx(), ListSolutionsInput{RequestID: r.ID, Kind: domain.SolutionKindAnswer}))
	if len(none.Solutions) != 0 {
		t.Fatalf("filter by kind: %+v", none.Solutions)
	}
	_, err := uc.Execute(w.ctx(), ListSolutionsInput{RequestID: "missing"})
	requireCode(t, err, "REQUEST_SOLUTION_REQUEST_NOT_FOUND")
	_, err = uc.Execute(lcCtxNoTenant(), ListSolutionsInput{RequestID: r.ID})
	requireCode(t, err, "REQUEST_TENANT_REQUIRED")
}

func TestReporterOrAdminAuthorizer(t *testing.T) {
	w := newSolWorld()
	r := w.seedRequest(domain.RequestTypeChangeRequest, domain.RequestStatusAnalyzing)
	a := ReporterOrAdminAuthorizer{}
	if err := a.AuthorizeGenerate(w.ctx(), r); err == nil {
		t.Fatal("a stranger must be refused")
	}
	if err := a.AuthorizeChoose(w.ctx(), r); err == nil {
		t.Fatal("a stranger must be refused")
	}
}

type fakePendingApprovals struct {
	ApprovalRepository
	pending []domain.Approval
}

func (f fakePendingApprovals) List(_ context.Context, _ string, flt ApprovalListFilter) ([]domain.Approval, string, error) {
	if flt.Status != domain.ApprovalStatusPending || flt.SubjectType != domain.SubjectSolution {
		return nil, "", nil
	}
	return f.pending, "", nil
}

type deciderFor struct{ allowed string }

func (d deciderFor) CanDecide(ctx context.Context, _ domain.Request, _ domain.Approval) error {
	if uid, _ := tenant.UserID(ctx); uid == d.allowed {
		return nil
	}
	return domain.ErrApprovalForbidden
}

func TestApproverAwareAuthorizer(t *testing.T) {
	w := newSolWorld()
	r := w.seedRequest(domain.RequestTypeChangeRequest, domain.RequestStatusAwaitingAnalysisApproval)
	approver := "approver-1"
	a := ApproverAwareAuthorizer{Approvals: fakePendingApprovals{pending: []domain.Approval{{ID: "a1"}}}, Decider: deciderFor{allowed: approver}}

	if err := a.AuthorizeChoose(tenant.WithUserID(lcCtx(), approver), r); err != nil {
		t.Fatalf("a pending approver may choose: %v", err)
	}
	if err := a.AuthorizeChoose(tenant.WithUserID(lcCtx(), "stranger"), r); err == nil {
		t.Fatal("someone who cannot decide must not choose")
	}
	if err := a.AuthorizeChoose(tenant.WithUserID(lcCtx(), r.ReporterID), r); err != nil {
		t.Fatalf("the reporter may choose: %v", err)
	}
	// Generating is never opened to approvers.
	if err := a.AuthorizeGenerate(tenant.WithUserID(lcCtx(), approver), r); err == nil {
		t.Fatal("approvers may not trigger generation")
	}
	none := ApproverAwareAuthorizer{Approvals: fakePendingApprovals{}, Decider: deciderFor{allowed: approver}}
	if err := none.AuthorizeChoose(tenant.WithUserID(lcCtx(), approver), r); err == nil {
		t.Fatal("without a pending approval nobody but the reporter and admins may choose")
	}
}
