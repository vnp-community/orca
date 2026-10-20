package contracttest

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/request-service/internal/domain"
	"github.com/stablyai/orca-go/services/request-service/internal/usecase"
)

// planGateArtifacts has no artifact side effect: the case is about the Request, not the stored Plan.
type planGateArtifacts struct{}

func (planGateArtifacts) Describe(_ context.Context, _ domain.Request, st domain.SubjectType, hinted string) (string, string, error) {
	return hinted, "d", nil
}
func (planGateArtifacts) Decided(context.Context, domain.Approval, bool) error  { return nil }
func (planGateArtifacts) Closed(context.Context, domain.Approval, string) error { return nil }

// anyoneMayDecide leaves who may approve to the approval contract; this case only needs a decision to happen.
type anyoneMayDecide struct{}

func (anyoneMayDecide) CanDecide(context.Context, domain.Request, domain.Approval) error { return nil }

// decideApproval is the real DecideApproval over the real repositories, with the subject handler that moves the Request.
func (r *flowRig) decideApproval() *usecase.DecideApproval {
	env := r.env
	transition := usecase.NewTransitionRequest(env.Requests, env.TxScope, env.Outbox)
	returner := usecase.NewReturnRequestToBacklog(env.Requests, transition, env.Returns, noopCanceller{}, &usecase.TaskExecutionGuard{Tasks: r.tasks.Client}, env.Tx, env.Outbox)
	registry := usecase.NewSubjectHandlerRegistry()
	registry.Register(domain.SubjectPlan, &usecase.TransitionSubjectHandler{Artifacts: planGateArtifacts{}, Transition: transition, Returner: returner, Requests: env.Locker})
	pinner, ok := env.Requests.(usecase.ExecutionModePinner)
	if !ok {
		r.t.Fatal("the request repository must pin execution modes")
	}
	return &usecase.DecideApproval{
		Repo: env.Approvals, Tx: env.Tx, Locker: env.Locker, Registry: registry, Outbox: env.Outbox, Authorizer: anyoneMayDecide{}, Pinner: pinner,
	}
}

// flowRunAllFromTheDecision is flowContinuousChangeRequest with the plan approved by the real DecideApproval: the mode is pinned by the
// decision itself (not by pinContinuous), and the Phases run one after the other without a Phase approval or a manual start.
func flowRunAllFromTheDecision(t *testing.T, env ExecutionEnv) {
	r := newFlowRig(t, env)
	req := seedExecuting(t, env, r.ctx, func(q *domain.Request) {
		q.Status = domain.RequestStatusAwaitingPlanApproval
	})
	plan := r.tasks.Add(req.ID, "plan", "plan", "", "open")
	p1 := r.tasks.Add(req.ID, "phase", "phase 1", plan, "open")
	p2 := r.tasks.Add(req.ID, "phase", "phase 2", plan, "open")
	p3 := r.tasks.Add(req.ID, "phase", "phase 3", plan, "open")
	t1 := r.tasks.Add(req.ID, "task", "t1", p1, "open")
	t2 := r.tasks.Add(req.ID, "task", "t2", p2, "open")
	t3 := r.tasks.Add(req.ID, "task", "t3", p3, "open")
	r.tasks.DependsOn(p3, p2)
	planApprover := uuid.NewString()
	r.tasks.AllowTreeReaders(planApprover)

	opener := &dbOpener{env: env}
	gate, err := opener.Execute(r.ctx, usecase.OpenApprovalInput{RequestID: req.ID, SubjectType: domain.SubjectPlan, SubjectID: plan, RequestedBy: uuid.NewString()})
	if err != nil {
		t.Fatal(err)
	}
	if got := r.reload(req); got.ExecutionMode != "" || got.Status != domain.RequestStatusAwaitingPlanApproval {
		t.Fatalf("before the decision: %s/%q", got.Status, got.ExecutionMode)
	}
	decideCtx := tenant.WithRole(actingAs(r.ctx, planApprover), "admin")
	res, err := r.decideApproval().Execute(decideCtx, usecase.DecideApprovalInput{
		ID: gate.ID, Decision: "approve", ExpectedDigest: gate.SubjectDigest, ExecutionMode: "continuous",
	})
	if err != nil {
		t.Fatal(err)
	}
	if res.RequestStatus != domain.RequestStatusExecuting || res.ExecutionMode != domain.ExecutionContinuous {
		t.Fatalf("decision result = %+v", res)
	}

	if started, err := r.startExec.Execute(r.ctx, req.ID); err != nil || !started {
		t.Fatalf("StartExecution: started=%v %v", started, err)
	}
	r.settle()
	for _, p := range []string{p1, p2, p3} {
		if got := r.approvals(req, domain.SubjectPhase, p); len(got) != 0 {
			t.Fatalf("no Phase approval is opened after a run-all decision: %+v", got)
		}
	}
	if r.tasks.Task(t1).Status != "in_progress" || r.tasks.Task(t2).Status != "open" {
		t.Fatalf("only phase 1 runs first: %s %s", r.tasks.Task(t1).Status, r.tasks.Task(t2).Status)
	}
	if starts, err := env.PhaseStarts.ListByRequest(r.ctx, req.ID); err != nil || len(starts) != 1 || starts[0].Trigger != domain.PhaseStartAuto || starts[0].StartedBy != planApprover {
		t.Fatalf("phase 1 starts by itself as the approver: %+v %v", starts, err)
	}
	for _, tid := range []string{t1, t2, t3} {
		r.tasks.RunSucceeded(tid)
		r.settle()
	}

	if got := r.reload(req); got.Status != domain.RequestStatusCompleted || got.ExecutionMode != domain.ExecutionContinuous {
		t.Fatalf("status=%s mode=%q, want completed / continuous", got.Status, got.ExecutionMode)
	}
	starts, err := env.PhaseStarts.ListByRequest(r.ctx, req.ID)
	if err != nil || len(starts) != 3 {
		t.Fatalf("three starts: %d %v", len(starts), err)
	}
	for _, s := range starts {
		if s.Trigger != domain.PhaseStartAuto || s.StartedBy != planApprover {
			t.Fatalf("every Phase starts by itself as the approver, none by a manual StartPhase: %+v", s)
		}
	}
}
