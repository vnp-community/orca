package usecase

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stablyai/orca-go/services/request-service/internal/domain"
)

func newChoose(w *solWorld) *ChooseSolutionOption {
	return NewChooseSolutionOption(w, fakeSolutions{w}, fakeDigests{w}, allowAll{}, w)
}

func seedProposed(t *testing.T, w *solWorld) (domain.Request, domain.Solution) {
	t.Helper()
	r := w.seedRequest(domain.RequestTypeChangeRequest, domain.RequestStatusAwaitingAnalysisApproval)
	s := domain.Solution{ID: uuid.NewString(), TenantID: "t1", RequestID: r.ID, Kind: domain.SolutionKindSolution, Status: domain.SolutionStatusProposed,
		OptionsJSON: []byte(validSolutionReply(t)), Version: 1, CreatedAt: time.Now()}
	w.solutions[s.ID] = s
	return r, s
}

func TestChooseSolutionOption_RecordsChoiceAndRefreshesTheDigest(t *testing.T) {
	w := newSolWorld()
	r, s := seedProposed(t, w)
	uc := newChoose(w)

	first := must(uc.Execute(w.ctx(), ChooseSolutionOptionInput{RequestID: r.ID, SolutionID: s.ID, OptionID: "opt-2"}))
	if first.Solution.ChosenOption == nil || *first.Solution.ChosenOption != 1 {
		t.Fatalf("chosen = %v", first.Solution.ChosenOption)
	}
	want := must(domain.DigestOptions(s.OptionsJSON, intp(1)))
	if first.ApprovalDigest != want || w.digests["solution:"+s.ID] != want {
		t.Fatalf("digest=%s stored=%s want=%s", first.ApprovalDigest, w.digests["solution:"+s.ID], want)
	}

	// Choosing the same option again is a no-op that returns the same digest and does not bump the version.
	again := must(uc.Execute(w.ctx(), ChooseSolutionOptionInput{RequestID: r.ID, SolutionID: s.ID, OptionID: "opt-2"}))
	if again.ApprovalDigest != first.ApprovalDigest || again.Solution.Version != first.Solution.Version {
		t.Fatalf("repeat changed state: %+v vs %+v", again, first)
	}

	// Another choice gives another digest.
	other := must(uc.Execute(w.ctx(), ChooseSolutionOptionInput{RequestID: r.ID, SolutionID: s.ID, OptionID: "opt-1"}))
	if other.ApprovalDigest == first.ApprovalDigest || other.Solution.Version != first.Solution.Version+1 {
		t.Fatalf("second choice: %+v", other)
	}
	if st := w.requests[r.ID].Status; st != domain.RequestStatusAwaitingAnalysisApproval {
		t.Fatalf("choosing must not move the request: %s", st)
	}
}

func TestChooseSolutionOption_Rejections(t *testing.T) {
	w := newSolWorld()
	r, s := seedProposed(t, w)
	uc := newChoose(w)
	ctx := w.ctx()

	_, err := uc.Execute(ctx, ChooseSolutionOptionInput{RequestID: r.ID, SolutionID: s.ID, OptionID: "opt-9"})
	requireCode(t, err, "REQUEST_SOLUTION_OPTION_NOT_FOUND")
	_, err = uc.Execute(ctx, ChooseSolutionOptionInput{RequestID: r.ID, SolutionID: uuid.NewString(), OptionID: "opt-1"})
	requireCode(t, err, "REQUEST_SOLUTION_NOT_FOUND")
	_, err = uc.Execute(ctx, ChooseSolutionOptionInput{RequestID: "missing", SolutionID: s.ID, OptionID: "opt-1"})
	requireCode(t, err, "REQUEST_SOLUTION_REQUEST_NOT_FOUND")

	otherReq, _ := seedProposed(t, w)
	_, err = uc.Execute(ctx, ChooseSolutionOptionInput{RequestID: otherReq.ID, SolutionID: s.ID, OptionID: "opt-1"})
	requireCode(t, err, "REQUEST_SOLUTION_NOT_FOUND")

	approved := s
	approved.Status = domain.SolutionStatusApproved
	w.solutions[s.ID] = approved
	_, err = uc.Execute(ctx, ChooseSolutionOptionInput{RequestID: r.ID, SolutionID: s.ID, OptionID: "opt-1"})
	requireCode(t, err, "REQUEST_SOLUTION_NOT_PROPOSED")

	diag := domain.Solution{ID: uuid.NewString(), TenantID: "t1", RequestID: r.ID, Kind: domain.SolutionKindDiagnosis, Status: domain.SolutionStatusProposed, OptionsJSON: []byte(`{}`), Version: 1}
	w.solutions[diag.ID] = diag
	_, err = uc.Execute(ctx, ChooseSolutionOptionInput{RequestID: r.ID, SolutionID: diag.ID, OptionID: "opt-1"})
	requireCode(t, err, "REQUEST_SOLUTION_KIND_NOT_ALLOWED")

	_, err = uc.Execute(lcCtxNoTenant(), ChooseSolutionOptionInput{RequestID: r.ID, SolutionID: s.ID, OptionID: "opt-1"})
	requireCode(t, err, "REQUEST_TENANT_REQUIRED")

	denied := NewChooseSolutionOption(w, fakeSolutions{w}, fakeDigests{w}, ReporterOrAdminAuthorizer{}, w)
	w.solutions[s.ID] = s
	_, err = denied.Execute(ctx, ChooseSolutionOptionInput{RequestID: r.ID, SolutionID: s.ID, OptionID: "opt-1"})
	requireCode(t, err, "REQUEST_SOLUTION_FORBIDDEN")
}

func TestChooseSolutionOption_ApproveWithTheNewDigestEndToEnd(t *testing.T) {
	// Choose -> the Approval digest follows -> the handler approves only that digest.
	k := newHandlerKit()
	r, s := k.proposed(domain.RequestTypeChangeRequest, domain.SolutionKindSolution, validSolutionReply(t), nil)
	choose := newChoose(k.w)
	res := must(choose.Execute(k.w.ctx(), ChooseSolutionOptionInput{RequestID: r.ID, SolutionID: s.ID, OptionID: "opt-2"}))
	ctx := k.w.ctx()
	if err := k.solH.OnApproved(ctx, ctx, approvalFor(r, s, domain.SubjectSolution, res.ApprovalDigest)); err != nil {
		t.Fatal(err)
	}
	if k.w.requests[r.ID].Status != domain.RequestStatusPlanning || *k.w.solutions[s.ID].ChosenOption != 1 {
		t.Fatalf("request=%s solution=%+v", k.w.requests[r.ID].Status, k.w.solutions[s.ID])
	}
}
