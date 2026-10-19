package usecase

import (
	"context"
	"strings"
	"testing"

	"github.com/stablyai/orca-go/services/request-service/internal/domain"
)

func planRunKit(t *testing.T, typ domain.RequestType, size domain.RequestSize, replies []string, errs ...error) (*planKit, domain.Request, domain.AnalysisRun, *scriptedCompleter, *RunPlanGeneration) {
	t.Helper()
	k := newPlanKit(nil)
	req := k.w.seedRequest(typ, domain.RequestStatusPlanning)
	r := k.w.requests[req.ID]
	r.Size = size
	k.w.requests[req.ID] = r
	k.w.seedApprovedSolution(req.ID, domain.SolutionKindSolution)
	run := k.startedRun(req.ID)
	completer := &scriptedCompleter{replies: replies, errs: errs}
	return k, r, run, completer, k.worker(completer)
}

func TestRunPlanGeneration_StoresTheValidatedProposalOnTheRun(t *testing.T) {
	k, _, run, completer, worker := planRunKit(t, domain.RequestTypeChangeRequest, domain.RequestSizeM, []string{planJSON(true)})
	if err := worker.Execute(k.w.ctx(), run); err != nil {
		t.Fatalf("execute: %v", err)
	}
	got := k.w.runs[run.ID]
	if got.Status != domain.RunStatusSucceeded || got.RawOutput == nil {
		t.Fatalf("run = %s", describe(got))
	}
	p, err := DecodePlanProposal(*got.RawOutput)
	if err != nil || len(p.Phases) != 1 || p.Phases[0].Tasks[0].Title != "Implement cache" {
		t.Fatalf("stored proposal: %+v %v", p, err)
	}
	if completer.calls() != 1 {
		t.Fatalf("one AI call expected, got %d", completer.calls())
	}
	prompt := completer.prompts[0]
	for _, want := range []string{"Use a cache", "phases", "<request>", "Add an LRU"} {
		if !strings.Contains(prompt, want) {
			t.Errorf("prompt lacks %q", want)
		}
	}
}

func TestRunPlanGeneration_RetriesOnceWithTheReason(t *testing.T) {
	// A flat plan for a type that needs phases is refused, then the corrected one is accepted.
	k, _, run, completer, worker := planRunKit(t, domain.RequestTypeChangeRequest, domain.RequestSizeM, []string{planJSON(false), planJSON(true)})
	if err := worker.Execute(k.w.ctx(), run); err != nil {
		t.Fatalf("execute: %v", err)
	}
	if completer.calls() != 2 || !strings.Contains(completer.prompts[1], "Your previous reply was rejected") {
		t.Fatalf("the second prompt must carry the rejection reason: calls=%d", completer.calls())
	}
	if k.w.runs[run.ID].Status != domain.RunStatusSucceeded {
		t.Fatalf("run = %s", describe(k.w.runs[run.ID]))
	}
}

func TestRunPlanGeneration_GivesUpAfterTheRetryAndKeepsTheRawAnswer(t *testing.T) {
	k, _, run, _, worker := planRunKit(t, domain.RequestTypeChangeRequest, domain.RequestSizeM, []string{"I cannot do that", "still not JSON"})
	err := worker.Execute(k.w.ctx(), run)
	f, ok := asFailure(err)
	if !ok || f.Code != "REQUEST_PLAN_AI_INVALID_JSON" || !strings.Contains(f.Raw, "still not JSON") {
		t.Fatalf("want an invalid-JSON failure carrying the raw text, got %v", err)
	}
}

func TestRunPlanGeneration_AIFailuresKeepTheirCodes(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want string
	}{
		{"not configured", &AIUnavailableError{Code: "REQUEST_AI_NOT_CONFIGURED", Detail: "no key"}, "REQUEST_AI_NOT_CONFIGURED"},
		{"no dev server", ErrNoDevServer, "REQUEST_PLAN_AI_UNAVAILABLE"},
		{"timeout", context.DeadlineExceeded, "REQUEST_AI_COMPLETE_TIMEOUT"},
		{"anything else", errBoom, "REQUEST_PLAN_AI_UNAVAILABLE"},
	}
	for _, c := range cases {
		k, _, run, _, worker := planRunKit(t, domain.RequestTypeTask, domain.RequestSizeS, []string{""}, c.err)
		f, ok := asFailure(worker.Execute(k.w.ctx(), run))
		if !ok || f.Code != c.want {
			t.Errorf("%s: got %+v, want %s", c.name, f, c.want)
		}
	}
}

func TestRunPlanGeneration_RequestMovedOnWhileTheModelWorked(t *testing.T) {
	k, req, run, _, worker := planRunKit(t, domain.RequestTypeTask, domain.RequestSizeS, []string{planJSON(false)})
	cur := k.w.requests[req.ID]
	cur.Status = domain.RequestStatusCancelled
	k.w.requests[req.ID] = cur
	f, ok := asFailure(worker.Execute(k.w.ctx(), run))
	if !ok || f.Code != "REQUEST_PLAN_REQUEST_MOVED" {
		t.Fatalf("got %+v", f)
	}
}

func TestRunPlanGeneration_ProjectStateFailuresAreNotRetried(t *testing.T) {
	// A performance Request without a baseline check fails at once: asking the model again cannot create the baseline.
	k, _, run, completer, worker := planRunKit(t, domain.RequestTypePerformance, domain.RequestSizeM, []string{planJSON(false)})
	f, ok := asFailure(worker.Execute(k.w.ctx(), run))
	if !ok || f.Code != "REQUEST_PERF_BASELINE_MISSING" {
		t.Fatalf("got %+v", f)
	}
	if completer.calls() != 1 {
		t.Fatalf("no retry expected, got %d AI calls", completer.calls())
	}
}

func TestRunPlanGeneration_RedactsSecretsBeforeStoring(t *testing.T) {
	secret := "AKIAIOSFODNN7EXAMPLE"
	reply := strings.Replace(planJSON(false), "done when tests pass", "use key "+secret, 1)
	k, _, run, _, worker := planRunKit(t, domain.RequestTypeTask, domain.RequestSizeS, []string{reply})
	if err := worker.Execute(k.w.ctx(), run); err != nil {
		t.Fatal(err)
	}
	if raw := *k.w.runs[run.ID].RawOutput; strings.Contains(raw, secret) {
		t.Fatalf("a secret reached the stored proposal: %s", raw)
	}
}
