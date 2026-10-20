package usecase

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/request-service/internal/domain"
)

// refusingPlanAuth refuses every public caller: the system path must never ask it.
type refusingPlanAuth struct{ PlanActorAuthorizer }

func (refusingPlanAuth) AuthorizePlan(context.Context, domain.Request) error {
	return domain.ErrPlanForbidden()
}

func systemKit(conns AnalysisConnectionResolver) (*planKit, domain.Request) {
	k := newPlanKit(conns)
	k.generate.auth = refusingPlanAuth{k.generate.auth}
	req := k.w.seedRequest(domain.RequestTypeBug, domain.RequestStatusPlanning)
	k.w.seedApprovedSolution(req.ID, domain.SolutionKindSolution)
	return k, k.w.requests[req.ID]
}

func systemCtx(k *planKit) context.Context { return lcCtx() }

func TestGeneratePlanExecuteForSystem_RunsAsTheReporterWithoutAnyCaller(t *testing.T) {
	k, req := systemKit(nil)
	res, err := k.generate.ExecuteForSystem(systemCtx(k), GeneratePlanInput{RequestID: req.ID, IdempotencyKey: "plan-auto-sol-1"})
	if err != nil || !res.Created || res.RunID == "" {
		t.Fatalf("system start: %+v %v", res, err)
	}
	run := k.w.runs[res.RunID]
	if run.ActorID != req.ReporterID || run.ActorID == "" {
		t.Fatalf("actor = %q, want the reporter %q", run.ActorID, req.ReporterID)
	}
	if run.IdempotencyKey == nil || *run.IdempotencyKey != "plan-auto-sol-1" {
		t.Fatalf("idempotency key = %v", run.IdempotencyKey)
	}
	if len(k.spawner.runs) != 1 {
		t.Fatalf("spawned %d runs", len(k.spawner.runs))
	}
}

func TestGeneratePlanExecuteForSystem_ResolvesTheDevServerAsTheReporter(t *testing.T) {
	var seen string
	conns := userRecordingConns{seen: &seen}
	k, req := systemKit(conns)
	if _, err := k.generate.ExecuteForSystem(systemCtx(k), GeneratePlanInput{RequestID: req.ID}); err != nil {
		t.Fatal(err)
	}
	if seen != req.ReporterID {
		t.Fatalf("the dev-server lookup ran as %q, want the reporter %q", seen, req.ReporterID)
	}
}

type userRecordingConns struct{ seen *string }

func (c userRecordingConns) ResolveForProject(ctx context.Context, _ string) (AnalysisConnection, error) {
	*c.seen, _ = tenant.UserID(ctx)
	return AnalysisConnection{}, nil
}

func TestGeneratePlanExecuteForSystem_SameKeyReturnsTheSameRunWithoutASecondWorker(t *testing.T) {
	k, req := systemKit(nil)
	in := GeneratePlanInput{RequestID: req.ID, IdempotencyKey: "plan-auto-sol-1"}
	first, err := k.generate.ExecuteForSystem(systemCtx(k), in)
	if err != nil {
		t.Fatal(err)
	}
	// The first run finished meanwhile: only the key (not "a run is running") can return it now.
	run := k.w.runs[first.RunID]
	run.Status = domain.RunStatusSucceeded
	k.w.runs[first.RunID] = run
	second, err := k.generate.ExecuteForSystem(systemCtx(k), in)
	if err != nil || second.Created || second.RunID != first.RunID {
		t.Fatalf("second call: %+v %v", second, err)
	}
	if len(k.spawner.runs) != 1 {
		t.Fatalf("a repeated key must not spawn a second worker, got %d", len(k.spawner.runs))
	}
}

func TestGeneratePlanExecuteForSystem_AFailedRunWithTheSameKeyIsReportedAsFailed(t *testing.T) {
	k, req := systemKit(nil)
	in := GeneratePlanInput{RequestID: req.ID, IdempotencyKey: "plan-auto-sol-1"}
	first, _ := k.generate.ExecuteForSystem(systemCtx(k), in)
	run := k.w.runs[first.RunID]
	code := "REQUEST_AI_COMPLETE_TIMEOUT"
	run.Status, run.ErrorCode = domain.RunStatusFailed, &code
	k.w.runs[first.RunID] = run

	again, err := k.generate.ExecuteForSystem(systemCtx(k), in)
	if err != nil || again.Created || again.Status != domain.RunStatusFailed || again.ErrorCode != code {
		t.Fatalf("same key after a failure: %+v %v", again, err)
	}

	in.IdempotencyKey = "plan-auto-sol-1-2"
	next, err := k.generate.ExecuteForSystem(systemCtx(k), in)
	if err != nil || !next.Created || next.RunID == first.RunID || next.Status != domain.RunStatusRunning {
		t.Fatalf("a new key must start a new run: %+v %v", next, err)
	}
}

func TestGeneratePlanExecuteForSystem_NoDevServerWritesNothing(t *testing.T) {
	k, req := systemKit(fakeConns{err: ErrNoDevServer})
	_, err := k.generate.ExecuteForSystem(systemCtx(k), GeneratePlanInput{RequestID: req.ID})
	requireCode(t, err, "REQUEST_PLAN_AI_UNAVAILABLE")
	if len(k.w.runs) != 0 || len(k.spawner.runs) != 0 {
		t.Fatalf("a refused start left %d run(s) and %d worker(s)", len(k.w.runs), len(k.spawner.runs))
	}
}

func TestGeneratePlanExecuteForSystem_KeepsTheStateAndPlanChecks(t *testing.T) {
	k, req := systemKit(nil)
	r := k.w.requests[req.ID]
	r.Status = domain.RequestStatusAnalyzing
	k.w.requests[req.ID] = r
	_, err := k.generate.ExecuteForSystem(systemCtx(k), GeneratePlanInput{RequestID: req.ID})
	requireCode(t, err, "REQUEST_STATE_STALE")
}

func TestGeneratePlanExecuteForSystem_NeedsAReporter(t *testing.T) {
	k, req := systemKit(nil)
	r := k.w.requests[req.ID]
	r.ReporterID = ""
	k.w.requests[req.ID] = r
	_, err := k.generate.ExecuteForSystem(systemCtx(k), GeneratePlanInput{RequestID: req.ID})
	requireCode(t, err, "REQUEST_REPORTER_REQUIRED")
}

func TestGeneratePlanExecute_StillNeedsACallerAndAuthorization(t *testing.T) {
	k, req := systemKit(nil)
	_, err := k.generate.Execute(lcCtx(), GeneratePlanInput{RequestID: req.ID})
	requireCode(t, err, "REQUEST_REPORTER_REQUIRED")
	_, err = k.generate.Execute(tenant.WithUserID(lcCtx(), uuid.NewString()), GeneratePlanInput{RequestID: req.ID})
	requireCode(t, err, "REQUEST_PLAN_FORBIDDEN")
	if len(k.w.runs) != 0 {
		t.Fatal("a refused public call must not start a run")
	}
}

func TestGeneratePlanExecute_PublicStartCarriesNoKey(t *testing.T) {
	// The RPC never sets the key, and a call without one keeps the "one running run" behaviour.
	k := newPlanKit(nil)
	req := k.w.seedRequest(domain.RequestTypeTask, domain.RequestStatusPlanning)
	res, err := k.generate.Execute(k.w.ctx(), GeneratePlanInput{RequestID: req.ID})
	if err != nil || k.w.runs[res.RunID].IdempotencyKey != nil {
		t.Fatalf("a public start must not carry a key: %+v %v", k.w.runs[res.RunID].IdempotencyKey, err)
	}
}
