package usecase

import (
	"strings"
	"testing"

	"github.com/stablyai/orca-go/services/request-service/internal/domain"
)

func TestGetPlanProposal_ReportsEachRunState(t *testing.T) {
	k, req, run, _, worker := planRunKit(t, domain.RequestTypeTask, domain.RequestSizeS, []string{planJSON(false)})
	get := NewGetPlanProposal(k.w, fakeRuns{k.w})

	v, err := get.Execute(k.w.ctx(), req.ID, run.ID, 0)
	if err != nil || v.Status != "running" || v.Proposal != nil {
		t.Fatalf("running: %+v %v", v, err)
	}
	if err := worker.Execute(k.w.ctx(), run); err != nil {
		t.Fatal(err)
	}
	v, err = get.Execute(k.w.ctx(), req.ID, "", 0) // no run id: the newest plan run
	if err != nil || v.Status != "succeeded" || v.Proposal == nil || v.Proposal.Tasks[0].Title != "Implement cache" || v.RunID != run.ID {
		t.Fatalf("succeeded: %+v %v", v, err)
	}
}

func TestGetPlanProposal_FailedRunCarriesTheReasonAndRawText(t *testing.T) {
	k, req, run, _, worker := planRunKit(t, domain.RequestTypeTask, domain.RequestSizeS, []string{"nope", "still nope"})
	err := worker.Execute(k.w.ctx(), run)
	f, _ := asFailure(err)
	k.w.runs[run.ID] = func() domain.AnalysisRun {
		failed := k.w.runs[run.ID]
		failed.Fail(f.Code, f.Message, k.w.now)
		raw := RedactRaw(f.Raw)
		failed.RawOutput = &raw
		return failed
	}()
	v, err := NewGetPlanProposal(k.w, fakeRuns{k.w}).Execute(k.w.ctx(), req.ID, run.ID, 0)
	if err != nil || v.Status != "failed" || v.ErrorCode != "REQUEST_PLAN_AI_INVALID_JSON" || !strings.Contains(v.Raw, "still nope") {
		t.Fatalf("failed: %+v %v", v, err)
	}
}

func TestGetPlanProposal_RefusesAnotherRequestsRunAndAnUnknownRun(t *testing.T) {
	k, _, run, _, _ := planRunKit(t, domain.RequestTypeTask, domain.RequestSizeS, []string{planJSON(false)})
	other := k.w.seedRequest(domain.RequestTypeTask, domain.RequestStatusPlanning)
	get := NewGetPlanProposal(k.w, fakeRuns{k.w})

	_, err := get.Execute(k.w.ctx(), other.ID, run.ID, 0)
	requireCode(t, err, "REQUEST_PLAN_RUN_NOT_FOUND")
	_, err = get.Execute(k.w.ctx(), other.ID, "", 0)
	requireCode(t, err, "REQUEST_PLAN_RUN_NOT_FOUND")
	_, err = get.Execute(k.w.ctx(), other.ID, "00000000-0000-4000-8000-000000000000", 0)
	requireCode(t, err, "REQUEST_PLAN_RUN_NOT_FOUND")
}
