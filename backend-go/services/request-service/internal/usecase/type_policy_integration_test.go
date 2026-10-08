package usecase

import (
	"testing"

	"github.com/stablyai/orca-go/services/request-service/internal/domain"
)

// runOneTaskRequest drives a one-task Request to the point where its only task is done.
func runOneTaskRequest(t *testing.T, r *exRig, typ domain.RequestType) (domain.Request, domain.TaskView) {
	t.Helper()
	req := r.request(typ, domain.RequestSizeS, domain.RequestStatusExecuting)
	var parent domain.TaskView
	if typ != domain.RequestTypeHotfix {
		parent = r.plan(req)
	}
	task := r.leaf(req, parent, "work", domain.TaskStatusOpen)
	advance(t, r, req, "")
	if r.tasks.get(task.ID).Status != domain.TaskStatusInProgress {
		t.Fatalf("%s: the task did not start: %v", typ, r.tasks.calls)
	}
	r.finishRun(req, task, true, "")
	return req, task
}

func record(t *testing.T, r *exRig, req domain.Request, kind domain.CheckKind, source domain.CheckSource, status domain.CheckStatus, metrics string) {
	t.Helper()
	if metrics == "" {
		metrics = `{}`
	}
	if _, err := r.checks.Append(lcCtx(), domain.RequestCheck{RequestID: req.ID, Kind: kind, Status: status, Source: source, Metrics: []byte(metrics), Summary: "recorded"}); err != nil {
		t.Fatal(err)
	}
}

func TestPolicyFlows_NoopTypesRunAsBefore(t *testing.T) {
	for _, typ := range []domain.RequestType{domain.RequestTypeBug, domain.RequestTypeTask, domain.RequestTypeDocs} {
		r := newExRig(t)
		req, _ := runOneTaskRequest(t, r, typ)
		if got := r.reload(req); got.Status != domain.RequestStatusCompleted {
			t.Errorf("%s: no policy means no extra completion condition, got %s", typ, got.Status)
		}
		if len(r.follow.calls) != 0 {
			t.Errorf("%s: no follow-ups expected", typ)
		}
	}
	// change_request has phases: same rule once its last phase is done.
	r := newExRig(t)
	req := r.request(domain.RequestTypeChangeRequest, domain.RequestSizeM, domain.RequestStatusExecuting)
	ph := r.phase(req, r.plan(req), "p")
	task := r.leaf(req, ph, "t", domain.TaskStatusOpen)
	r.approved(req, domain.SubjectPhase, ph.ID)
	if _, err := startPhase(r, req, ph.ID); err != nil {
		t.Fatal(err)
	}
	r.finishRun(req, task, true, "")
	if got := r.reload(req); got.Status != domain.RequestStatusCompleted {
		t.Errorf("change_request: got %s", got.Status)
	}
}

func TestPolicyFlows_Hotfix(t *testing.T) {
	r := newExRig(t)
	req, _ := runOneTaskRequest(t, r, domain.RequestTypeHotfix)
	if got := r.reload(req); got.Status != domain.RequestStatusCompleted {
		t.Fatalf("got %s", got.Status)
	}
	if len(r.follow.calls) != 1 || len(r.follow.calls[0]) != 2 {
		t.Fatalf("hotfix files two follow-ups: %+v", r.follow.calls)
	}
}

func TestPolicyFlows_Security(t *testing.T) {
	r := newExRig(t)
	req, _ := runOneTaskRequest(t, r, domain.RequestTypeSecurity)
	if got := r.reload(req); got.Status != domain.RequestStatusExecuting {
		t.Fatalf("no re-check yet: must wait, got %s", got.Status)
	}
	record(t, r, req, domain.CheckSecurityRecheck, domain.CheckSourceAgent, domain.CheckStatusPassed, "")
	if err := r.evaluate.Run(lcCtx(), r.reload(req)); err != nil {
		t.Fatal(err)
	}
	if got := r.reload(req); got.Status != domain.RequestStatusExecuting {
		t.Fatalf("an agent's own 'passed' must not complete a security request, got %s", got.Status)
	}
	record(t, r, req, domain.CheckSecurityRecheck, domain.CheckSourceManual, domain.CheckStatusPassed, "")
	if err := r.evaluate.Run(lcCtx(), r.reload(req)); err != nil {
		t.Fatal(err)
	}
	if got := r.reload(req); got.Status != domain.RequestStatusCompleted {
		t.Fatalf("a person confirmed the re-check, got %s", got.Status)
	}
}

func TestPolicyFlows_Security_FailedRecheckReturnsBacklog(t *testing.T) {
	r := newExRig(t)
	req, _ := runOneTaskRequest(t, r, domain.RequestTypeSecurity)
	record(t, r, req, domain.CheckSecurityRecheck, domain.CheckSourceManual, domain.CheckStatusFailed, "")
	if err := r.evaluate.Run(lcCtx(), r.reload(req)); err != nil {
		t.Fatal(err)
	}
	if got := r.reload(req); got.Status != domain.RequestStatusRequestBacklog || got.ReturnedCategory != domain.ReturnCategoryOther {
		t.Fatalf("got %+v", got)
	}
}

func TestPolicyFlows_Performance(t *testing.T) {
	baseline := `{"metrics":[{"name":"p95","unit":"ms","direction":"lower_is_better","baseline":200,"target_change_percent":20}],"method":"wrk"}`
	cases := []struct {
		name    string
		after   string
		want    domain.RequestStatus
		backlog domain.ReturnCategory
	}{
		{"target reached", `{"metrics":[{"name":"p95","value":150}]}`, domain.RequestStatusCompleted, ""},
		{"target missed", `{"metrics":[{"name":"p95","value":190}]}`, domain.RequestStatusRequestBacklog, domain.ReturnCategoryInfeasible},
		{"not measured yet", "", domain.RequestStatusExecuting, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r := newExRig(t)
			req := r.request(domain.RequestTypePerformance, domain.RequestSizeS, domain.RequestStatusExecuting)
			task := r.leaf(req, r.plan(req), "work", domain.TaskStatusOpen)
			record(t, r, req, domain.CheckPerfBaseline, domain.CheckSourceManual, domain.CheckStatusPassed, baseline)
			if tc.after != "" {
				record(t, r, req, domain.CheckPerfAfter, domain.CheckSourceAgent, domain.CheckStatusPassed, tc.after)
			}
			advance(t, r, req, "")
			r.finishRun(req, task, true, "")
			got := r.reload(req)
			if got.Status != tc.want || got.ReturnedCategory != tc.backlog {
				t.Fatalf("got %+v", got)
			}
		})
	}
}

func TestPolicyFlows_Refactor(t *testing.T) {
	before := `{"total":40,"passed":40,"failed":0,"command":"go test"}`
	cases := []struct {
		name  string
		after string
		want  domain.RequestStatus
	}{
		{"all three conditions", `{"total":40,"passed":40,"failed":0,"command":"go test","tests_modified":false}`, domain.RequestStatusCompleted},
		{"a test still fails", `{"total":40,"passed":39,"failed":1,"command":"go test","tests_modified":false}`, domain.RequestStatusRequestBacklog},
		{"tests were removed", `{"total":30,"passed":30,"failed":0,"command":"go test","tests_modified":false}`, domain.RequestStatusRequestBacklog},
		{"tests were edited", `{"total":40,"passed":40,"failed":0,"command":"go test","tests_modified":true}`, domain.RequestStatusRequestBacklog},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r := newExRig(t)
			req := r.request(domain.RequestTypeRefactor, domain.RequestSizeS, domain.RequestStatusExecuting)
			task := r.leaf(req, r.plan(req), "work", domain.TaskStatusOpen)
			record(t, r, req, domain.CheckTestsBefore, domain.CheckSourceAgent, domain.CheckStatusPassed, before)
			record(t, r, req, domain.CheckTestsAfter, domain.CheckSourceAgent, domain.CheckStatusPassed, tc.after)
			advance(t, r, req, "")
			r.finishRun(req, task, true, "")
			if got := r.reload(req); got.Status != tc.want {
				t.Fatalf("got %+v", got)
			}
		})
	}
}

func TestPolicyFlows_OpsRequest(t *testing.T) {
	r := newExRig(t)
	req := r.request(domain.RequestTypeOpsRequest, domain.RequestSizeM, domain.RequestStatusExecuting)
	plan := r.plan(req)
	gated := r.leaf(req, plan, "migrate", domain.TaskStatusOpen, domain.PolicyLabelGatePreDeploy)
	advance(t, r, req, "")
	if r.tasks.get(gated.ID).Status != domain.TaskStatusOpen {
		t.Fatal("the gated step waits for its approval")
	}
	r.approved(req, domain.SubjectPreDeploy, gated.ID)
	advance(t, r, req, "")
	r.finishRun(req, gated, true, "")
	if got := r.reload(req); got.Status != domain.RequestStatusExecuting {
		t.Fatalf("without ops_result the request waits, got %s", got.Status)
	}
	record(t, r, req, domain.CheckOpsResult, domain.CheckSourceManual, domain.CheckStatusPassed, "")
	if err := r.evaluate.Run(lcCtx(), r.reload(req)); err != nil {
		t.Fatal(err)
	}
	if got := r.reload(req); got.Status != domain.RequestStatusCompleted {
		t.Fatalf("got %s", got.Status)
	}
}

func TestPolicyFlows_AllTypes(t *testing.T) {
	reg := domain.NewPolicyRegistry(domain.PolicyDeps{})
	with := 0
	for _, typ := range domain.AllFlowTypes() {
		if reg.HasPolicy(typ) {
			with++
		}
		if reg.PolicyFor(typ) == nil {
			t.Errorf("%s has no policy object", typ)
		}
	}
	if with != 5 || len(domain.AllFlowTypes())-with != 6 {
		t.Fatalf("5 types with a policy and 6 on the standard path, got %d/%d", with, len(domain.AllFlowTypes())-with)
	}
}
