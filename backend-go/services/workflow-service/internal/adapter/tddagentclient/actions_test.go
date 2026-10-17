package tddagentclient

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/workflow-service/internal/domain"
	"github.com/stablyai/orca-go/services/workflow-service/internal/usecase"
)

const (
	tenantID = "11111111-1111-1111-1111-111111111111"
	execID   = "22222222-2222-2222-2222-222222222222"
)

// fakeTools scripts tool answers; every call is recorded.
type fakeTools struct {
	mu      sync.Mutex
	calls   []string
	args    map[string][]map[string]any
	answers map[string]func(args map[string]any) (any, error)
}

func newFakeTools() *fakeTools {
	return &fakeTools{args: map[string][]map[string]any{}, answers: map[string]func(map[string]any) (any, error){}}
}

func (f *fakeTools) on(tool string, fn func(args map[string]any) (any, error)) { f.answers[tool] = fn }

func (f *fakeTools) CallTool(_ context.Context, target, projectDir, tool string, args map[string]any, out any) error {
	f.mu.Lock()
	f.calls = append(f.calls, tool)
	f.args[tool] = append(f.args[tool], args)
	fn := f.answers[tool]
	f.mu.Unlock()
	if target != "project:p1" || projectDir != "/srv/projects/repo" {
		return fmt.Errorf("unexpected target %s dir %s", target, projectDir)
	}
	if fn == nil {
		return fmt.Errorf("no answer for %s", tool)
	}
	res, err := fn(args)
	if err != nil {
		return err
	}
	if out != nil {
		raw, _ := json.Marshal(res)
		return json.Unmarshal(raw, out)
	}
	return nil
}

func (f *fakeTools) ResolveProjectDir(_ context.Context, projectID, requested string) (string, error) {
	if requested != "" && requested != "/srv/projects/repo" {
		return "", &BridgeError{Code: "TDD_AGENT_PROJECT_DIR_OUTSIDE_PROJECT", Message: requested}
	}
	return "/srv/projects/repo", nil
}

func (f *fakeTools) count(tool string) int {
	f.mu.Lock()
	defer f.mu.Unlock()
	n := 0
	for _, c := range f.calls {
		if c == tool {
			n++
		}
	}
	return n
}

// fakeClock advances instantly.
type fakeClock struct{ now time.Time }

func (c *fakeClock) Now() time.Time { return c.now }
func (c *fakeClock) Sleep(_ context.Context, d time.Duration) error {
	c.now = c.now.Add(d)
	return nil
}

type fakeExecs struct{ status domain.Status }

func (f *fakeExecs) GetExecution(context.Context, string, string) (domain.WorkflowExecution, error) {
	return domain.WorkflowExecution{ID: execID, Status: f.status}, nil
}

func stepCtx(stepID string) context.Context {
	ctx := tenant.WithTenantID(context.Background(), tenantID)
	ctx = tenant.WithUserID(tenant.WithProjectID(ctx, "p1"), "user-1")
	return usecase.WithStepIdentity(ctx, usecase.StepIdentity{ExecutionID: execID, StepID: stepID})
}

func handlers(tools *fakeTools, store ApprovalStore) (*Handlers, *fakeExecs) {
	execs := &fakeExecs{status: domain.StatusRunning}
	h := NewHandlers(tools, execs, store)
	h.clock = &fakeClock{now: time.Date(2026, 10, 14, 0, 0, 0, 0, time.UTC)}
	return h, execs
}

// view builds a run_get answer.
func view(status, pending, blocked string, alive bool, tasks map[string]any, children ...map[string]any) map[string]any {
	v := map[string]any{
		"run": map[string]any{"run_id": "run-1", "flow": "change-request", "status": status,
			"result": map[string]any{"blockers": []string{}, "pending_gate": pending, "blocked_gate": blocked}},
		"tasks": tasks, "children": children,
	}
	if alive {
		v["process"] = map[string]any{"pid": 42, "alive": true, "heartbeat_interval_s": 15}
	}
	return v
}

func output(t *testing.T, res domain.StepResult) map[string]any {
	t.Helper()
	var out map[string]any
	if err := json.Unmarshal([]byte(res.OutputJSON), &out); err != nil {
		t.Fatalf("output %q: %v", res.OutputJSON, err)
	}
	return out
}

func must(t *testing.T) func(domain.StepResult, error) domain.StepResult {
	return func(res domain.StepResult, err error) domain.StepResult {
		t.Helper()
		if err != nil {
			t.Fatalf("handler error: %v", err)
		}
		return res
	}
}

func TestStartCreatesTheRunWithTheExternalJob(t *testing.T) {
	tools := newFakeTools()
	tools.on("run_list", func(map[string]any) (any, error) { return map[string]any{"runs": []any{}}, nil })
	tools.on("run_start", func(args map[string]any) (any, error) {
		return map[string]any{"run_id": "run-1", "run": map[string]any{"status": "running"}}, nil
	})
	h, _ := handlers(tools, nil)
	res := must(t)(h.Start(stepCtx("start"), `{"flow":"change-request","sources":[{"id":"CR","revision":"1","role":"CR","path":"docs/cr.md"}]}`))
	out := output(t, res)
	if res.Status != domain.ResultStatusCompleted || out["runId"] != "run-1" || out["reused"] != false {
		t.Fatalf("got %s %v", res.Status, out)
	}
	ext := tools.args["run_start"][0]["external"].(map[string]string)
	if ext["executionId"] != execID || ext["stepId"] != "start" || ext["system"] != "orca" || ext["userId"] != "user-1" {
		t.Fatalf("external = %v", ext)
	}
}

func TestStartReusesTheRunOfThisStepAfterARestart(t *testing.T) {
	tools := newFakeTools()
	tools.on("run_list", func(args map[string]any) (any, error) {
		if args["external_execution_id"] != execID || args["external_step_id"] != "start" {
			return nil, errors.New("wrong filter")
		}
		return map[string]any{"runs": []any{map[string]any{"run_id": "run-old", "status": "awaiting_approval"}}}, nil
	})
	h, _ := handlers(tools, nil)
	out := output(t, must(t)(h.Start(stepCtx("start"), `{"flow":"change-request"}`)))
	if out["runId"] != "run-old" || out["reused"] != true || tools.count("run_start") != 0 {
		t.Fatalf("got %v, run_start calls %d", out, tools.count("run_start"))
	}
}

func TestStartNeedsAFlowAndAProjectDirInsideTheProject(t *testing.T) {
	h, _ := handlers(newFakeTools(), nil)
	res := must(t)(h.Start(stepCtx("start"), `{}`))
	if res.Status != domain.ResultStatusFailed || output(t, res)["error"] != CodeInvalidParams {
		t.Fatalf("got %v", res)
	}
	res = must(t)(h.Start(stepCtx("start"), `{"flow":"x","projectDir":"/etc"}`))
	if output(t, res)["error"] != "TDD_AGENT_PROJECT_DIR_OUTSIDE_PROJECT" {
		t.Fatalf("got %v", res)
	}
}

func TestAdvanceDispatchesOnceAndStopsAtTheExpectedGate(t *testing.T) {
	tools := newFakeTools()
	looks := 0
	tools.on("run_get", func(map[string]any) (any, error) {
		looks++
		switch looks {
		case 1:
			return view("running", "", "", false, map[string]any{"G0": map[string]any{"status": "completed"}}), nil
		case 2, 3:
			return view("running", "", "", true, map[string]any{"G0": map[string]any{"status": "completed"}}), nil
		}
		return view("awaiting_approval", "G1", "", false, map[string]any{"G1": map[string]any{"status": "awaiting_approval"}}), nil
	})
	tools.on("run_dispatch", func(map[string]any) (any, error) { return map[string]any{"dispatched": true, "pid": 42}, nil })
	h, _ := handlers(tools, nil)
	res := must(t)(h.Advance(stepCtx("to_g1"), `{"runId":"run-1","expectGate":"G1","pollIntervalSec":1}`))
	out := output(t, res)
	if res.Status != domain.ResultStatusCompleted || out["pendingGate"] != "G1" || tools.count("run_dispatch") != 1 {
		t.Fatalf("got %s %v dispatches %d", res.Status, out, tools.count("run_dispatch"))
	}
}

func TestAdvanceFailsOnAnUnexpectedGateAndOnBlockers(t *testing.T) {
	tools := newFakeTools()
	tools.on("run_get", func(map[string]any) (any, error) { return view("awaiting_approval", "G2", "", false, nil), nil })
	h, _ := handlers(tools, nil)
	if out := output(t, must(t)(h.Advance(stepCtx("a"), `{"runId":"run-1","expectGate":"G1"}`))); out["error"] != CodeUnexpectedGate {
		t.Fatalf("got %v", out)
	}

	tools = newFakeTools()
	looks := 0
	tools.on("run_get", func(map[string]any) (any, error) {
		looks++
		if looks == 1 {
			return view("running", "", "", false, nil), nil
		}
		v := view("blocked", "", "G3", false, nil)
		v["run"].(map[string]any)["result"].(map[string]any)["blockers"] = []string{"SAD_READONLY"}
		return v, nil
	})
	tools.on("run_dispatch", func(map[string]any) (any, error) { return map[string]any{"dispatched": true}, nil })
	h, _ = handlers(tools, nil)
	res := must(t)(h.Advance(stepCtx("a"), `{"runId":"run-1"}`))
	out := output(t, res)
	if res.Status != domain.ResultStatusFailed || out["error"] != CodeBlocked || out["blockedGate"] != "G3" {
		t.Fatalf("got %v", out)
	}
	if b := out["blockers"].([]any); len(b) != 1 || b[0] != "SAD_READONLY" {
		t.Fatalf("blockers %v", b)
	}
}

func TestAdvanceRetriesABlockedRunMetAtTheFirstLook(t *testing.T) {
	tools := newFakeTools()
	looks := 0
	tools.on("run_get", func(map[string]any) (any, error) {
		looks++
		if looks == 1 {
			return view("blocked", "", "G3", false, nil), nil
		}
		return view("completed", "", "", false, nil), nil
	})
	tools.on("run_dispatch", func(map[string]any) (any, error) { return map[string]any{"dispatched": true}, nil })
	h, _ := handlers(tools, nil)
	res := must(t)(h.Advance(stepCtx("a"), `{"runId":"run-1","stopAt":["completed"]}`))
	if res.Status != domain.ResultStatusCompleted || tools.count("run_dispatch") != 1 {
		t.Fatalf("got %v dispatches %d", res, tools.count("run_dispatch"))
	}
}

func TestAdvanceRedispatchesADeadProcessUpToALimit(t *testing.T) {
	tools := newFakeTools()
	tools.on("run_get", func(map[string]any) (any, error) { return view("running", "", "", false, nil), nil })
	tools.on("run_dispatch", func(map[string]any) (any, error) { return map[string]any{"dispatched": true}, nil })
	h, _ := handlers(tools, nil)
	out := output(t, must(t)(h.Advance(stepCtx("a"), `{"runId":"run-1"}`)))
	if out["error"] != CodeAdvanceCrashed || tools.count("run_dispatch") != maxRedispatch {
		t.Fatalf("got %v dispatches %d", out, tools.count("run_dispatch"))
	}
}

func TestAdvanceToleratesShortOutagesButNotLongOnes(t *testing.T) {
	tools := newFakeTools()
	looks := 0
	tools.on("run_get", func(map[string]any) (any, error) {
		looks++
		if looks <= MaxTransportErrors-1 {
			return nil, fmt.Errorf("%w: relay down", ErrTransport)
		}
		return view("completed", "", "", false, nil), nil
	})
	h, _ := handlers(tools, nil)
	if res := must(t)(h.Advance(stepCtx("a"), `{"runId":"run-1","stopAt":["completed"]}`)); res.Status != domain.ResultStatusCompleted {
		t.Fatalf("a short outage failed the step: %v", res)
	}
	tools.on("run_get", func(map[string]any) (any, error) { return nil, fmt.Errorf("%w: relay down", ErrTransport) })
	h, _ = handlers(tools, nil)
	if out := output(t, must(t)(h.Advance(stepCtx("a"), `{"runId":"run-1"}`))); out["error"] != CodeUnreachable {
		t.Fatalf("got %v", out)
	}
}

func TestAdvanceGivesUpAtTheDeadlineWithoutCancellingTheRun(t *testing.T) {
	tools := newFakeTools()
	tools.on("run_get", func(map[string]any) (any, error) { return view("running", "", "", true, nil), nil })
	h, _ := handlers(tools, nil)
	out := output(t, must(t)(h.Advance(stepCtx("a"), `{"runId":"run-1","deadlineMin":1,"pollIntervalSec":10}`)))
	if out["error"] != CodeDeadline || tools.count("run_cancel") != 0 {
		t.Fatalf("got %v", out)
	}
}

func TestCancelledExecutionCancelsTheRun(t *testing.T) {
	tools := newFakeTools()
	tools.on("run_get", func(map[string]any) (any, error) { return view("running", "", "", true, nil), nil })
	tools.on("run_cancel", func(args map[string]any) (any, error) { return map[string]any{"cancelled": []string{"run-1"}}, nil })
	h, execs := handlers(tools, nil)
	execs.status = domain.StatusCancelled
	out := output(t, must(t)(h.Advance(stepCtx("a"), `{"runId":"run-1"}`)))
	if out["error"] != CodeCancelled || tools.count("run_cancel") != 1 {
		t.Fatalf("got %v cancels %d", out, tools.count("run_cancel"))
	}
	if reason := tools.args["run_cancel"][0]["reason"].(string); !strings.Contains(reason, execID) {
		t.Fatalf("reason %q", reason)
	}
}

func TestWaitChildReportsTheNewestChildAndItsOutputs(t *testing.T) {
	tools := newFakeTools()
	childLooks := 0
	tools.on("run_get", func(args map[string]any) (any, error) {
		if args["run_id"] == "run-1" {
			return view("awaiting_approval", "G5", "", false, nil,
				map[string]any{"run_id": "child-old", "flow": "testspec", "status": "failed", "created_at": "2026-10-14T01:00:00Z"},
				map[string]any{"run_id": "child-new", "flow": "testspec", "status": "running", "created_at": "2026-10-14T02:00:00Z"}), nil
		}
		childLooks++
		status := "running"
		if childLooks > 1 {
			status = "completed"
		}
		v := view(status, "", "", false, map[string]any{"TS1": map[string]any{"status": "completed", "output": map[string]any{"files": []string{"a.md"}}}})
		v["run"].(map[string]any)["run_id"] = "child-new"
		return v, nil
	})
	h, _ := handlers(tools, nil)
	res := must(t)(h.WaitChild(stepCtx("testspecs"), `{"runId":"run-1"}`))
	out := output(t, res)
	if res.Status != domain.ResultStatusCompleted || out["childRunId"] != "child-new" {
		t.Fatalf("got %v", out)
	}
	if _, ok := out["outputs"].(map[string]any)["TS1"]; !ok {
		t.Fatalf("outputs %v", out["outputs"])
	}
}

func TestWaitChildFailsWhenTheRunEndsWithoutOne(t *testing.T) {
	tools := newFakeTools()
	tools.on("run_get", func(map[string]any) (any, error) { return view("completed", "", "", false, nil), nil })
	h, _ := handlers(tools, nil)
	if out := output(t, must(t)(h.WaitChild(stepCtx("t"), `{"runId":"run-1"}`))); out["error"] != CodeChildMissing {
		t.Fatalf("got %v", out)
	}
}

func TestWaitEvidenceDispatchesUntilTheGatePasses(t *testing.T) {
	tools := newFakeTools()
	looks := 0
	tools.on("run_get", func(map[string]any) (any, error) {
		looks++
		if looks < 3 {
			return view("awaiting_evidence", "", "G6", false, nil), nil
		}
		return view("awaiting_approval", "G7", "", false, nil), nil
	})
	tools.on("run_dispatch", func(map[string]any) (any, error) { return map[string]any{"dispatched": true}, nil })
	h, _ := handlers(tools, nil)
	res := must(t)(h.WaitEvidence(stepCtx("evidence"), `{"runId":"run-1"}`))
	if res.Status != domain.ResultStatusCompleted || tools.count("run_dispatch") != 2 {
		t.Fatalf("got %v dispatches %d", res, tools.count("run_dispatch"))
	}
}

func TestAdvanceNeedsARunID(t *testing.T) {
	h, _ := handlers(newFakeTools(), nil)
	if out := output(t, must(t)(h.Advance(stepCtx("a"), `{}`))); out["error"] != CodeInvalidParams {
		t.Fatalf("got %v", out)
	}
}

func TestRegisterSkipsApprovalWithoutAStore(t *testing.T) {
	var names []string
	reg := func(name string, _ func(context.Context, string) (domain.StepResult, error)) {
		names = append(names, name)
	}
	h, _ := handlers(newFakeTools(), nil)
	h.Register(reg)
	if len(names) != 4 {
		t.Fatalf("registered %v", names)
	}
	names = nil
	h, _ = handlers(newFakeTools(), newFakeStore())
	h.Register(reg)
	if len(names) != 5 || names[4] != ActionApproval {
		t.Fatalf("registered %v", names)
	}
}

func TestStartAcceptsRequestAndSourcesAsJSONStringsAndIgnoresUnsetInputs(t *testing.T) {
	tools := newFakeTools()
	tools.on("run_list", func(map[string]any) (any, error) { return map[string]any{"runs": []any{}}, nil })
	tools.on("run_start", func(map[string]any) (any, error) { return map[string]any{"run_id": "run-1"}, nil })
	h, _ := handlers(tools, nil)
	params := `{"flow":"testspec","projectDir":"{{projectDir}}","request":"{\"params\":{\"tdd\":\"TDD-1\"}}","sources":"{{sources}}"}`
	res := must(t)(h.Start(stepCtx("start"), params))
	if res.Status != domain.ResultStatusCompleted {
		t.Fatalf("got %s", res.OutputJSON)
	}
	args := tools.args["run_start"][0]
	if args["request"].(map[string]any)["params"].(map[string]any)["tdd"] != "TDD-1" {
		t.Fatalf("request %v", args["request"])
	}
	if _, ok := args["sources"]; ok {
		t.Fatalf("sources should be absent: %v", args["sources"])
	}
}

func TestStartModesCallTheProjectTools(t *testing.T) {
	tools := newFakeTools()
	tools.on("run_list", func(map[string]any) (any, error) { return map[string]any{"runs": []any{}}, nil })
	tools.on("generate_start", func(args map[string]any) (any, error) {
		return map[string]any{"status": "started", "run_id": "gen-3"}, nil
	})
	tools.on("change_start", func(args map[string]any) (any, error) {
		if args["name"] != "keep-evidence" || args["cr_path"] != "docs/CR-1.md" {
			return nil, &ToolError{Code: "BAD", Message: "args"}
		}
		return map[string]any{"run_id": "change-keep-evidence", "status": "running"}, nil
	})
	h, _ := handlers(tools, nil)
	if out := output(t, must(t)(h.Start(stepCtx("start"), `{"mode":"generate"}`))); out["runId"] != "gen-3" || out["flow"] != "tdd-generation" {
		t.Fatalf("generate %v", out)
	}
	if out := output(t, must(t)(h.Start(stepCtx("start"), `{"mode":"change","changeName":"keep-evidence","crPath":"docs/CR-1.md"}`))); out["runId"] != "change-keep-evidence" {
		t.Fatalf("change %v", out)
	}
	if ext := tools.args["change_start"][0]["external"].(map[string]string); ext["stepId"] != "start" {
		t.Fatalf("external %v", ext)
	}
	if out := output(t, must(t)(h.Start(stepCtx("start"), `{"mode":"change"}`))); out["error"] != CodeInvalidParams {
		t.Fatalf("change without name %v", out)
	}
}

func TestStepsAfterAnUpToDateGenerateSucceedWithoutARun(t *testing.T) {
	tools := newFakeTools()
	h, _ := handlers(tools, nil)
	out := output(t, must(t)(h.Advance(stepCtx("advance"), `{"runId":"","allowNoRun":true}`)))
	if out["skipped"] == nil || len(tools.calls) != 0 {
		t.Fatalf("got %v calls %v", out, tools.calls)
	}
	if out := output(t, must(t)(h.Advance(stepCtx("advance"), `{"runId":""}`))); out["error"] != CodeInvalidParams {
		t.Fatalf("without allowNoRun %v", out)
	}
}
