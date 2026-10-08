package usecase

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/services/task-service/internal/domain"
)

const (
	ptTenant  = "tenant-1"
	ptRequest = "req-1"
	ptProject = "proj-1"
)

func planTreeCtx() context.Context { return withIdentity(context.Background(), ptTenant, "system") }

type planTreeEnv struct {
	tasks  *fakeTaskRepository
	edges  *fakeEdgeRepository
	grants *fakeGrantRepository
	uc     *CreatePlanTree
}

func newPlanTreeEnv() planTreeEnv {
	tasks := newFakeTaskRepository()
	edges := &fakeEdgeRepository{}
	grants := &fakeGrantRepository{}
	return planTreeEnv{tasks, edges, grants, NewCreatePlanTree(newFakeTxRunner(tasks, edges), tasks, grants)}
}

func basePlanTreeInput() CreatePlanTreeInput {
	return CreatePlanTreeInput{RequestID: ptRequest, ProjectID: ptProject, Title: "Plan", CreatorID: "user-9", AIPlanJSON: `{"v":1}`}
}

func errCode(err error) string {
	var ae *apperrors.AppError
	if errors.As(err, &ae) {
		return ae.Code
	}
	return ""
}

func countByType(tasks *fakeTaskRepository, typ string) int {
	n := 0
	for _, t := range tasks.tasks {
		if t.Type == typ {
			n++
		}
	}
	return n
}

func TestCreatePlanTree_PlanPhasesTasks_EdgesAndLabels(t *testing.T) {
	env := newPlanTreeEnv()
	in := basePlanTreeInput()
	in.Phases = []PlanTreePhase{
		{Title: "P1", Tasks: []PlanTreeTask{{Title: "a", Labels: []string{"backend"}}, {Title: "b", DependsOnIndices: []int{0}}}},
		{Title: "P2", DependsOnPhaseIndices: []int{0}, Tasks: []PlanTreeTask{{Title: "c"}}},
	}
	res, err := env.uc.Execute(planTreeCtx(), in)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if res.AlreadyExists || res.Plan.Type != domain.TypePlan || res.Plan.RequestID != ptRequest {
		t.Fatalf("bad plan: %+v", res)
	}
	if len(res.Phases) != 2 || len(res.Tasks) != 3 {
		t.Fatalf("want 2 phases and 3 tasks, got %d and %d", len(res.Phases), len(res.Tasks))
	}
	for _, task := range res.Tasks {
		if task.RequestID != ptRequest || task.ProjectID != ptProject {
			t.Errorf("task %q lost request/project: %+v", task.Title, task)
		}
	}
	if res.Tasks[0].Labels[0] != "backend" {
		t.Errorf("labels not kept: %v", res.Tasks[0].Labels)
	}
	var parentEdges, depEdges int
	for _, e := range env.edges.edges {
		if e.Kind == domain.EdgeKindParentChild {
			parentEdges++
		} else {
			depEdges++
		}
	}
	if parentEdges != 5 || depEdges != 2 { // plan->2 phases, phases->3 tasks; b->a, P2->P1
		t.Errorf("edges parent=%d depends=%d", parentEdges, depEdges)
	}
	if env.tasks.tasks[res.Plan.ID].AIPlanJSON != `{"v":1}` {
		t.Errorf("ai_plan_json not persisted")
	}
}

func TestCreatePlanTree_DirectTasksUnderPlan(t *testing.T) {
	env := newPlanTreeEnv()
	in := basePlanTreeInput()
	in.Tasks = []PlanTreeTask{{Title: "only"}}
	res, err := env.uc.Execute(planTreeCtx(), in)
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Phases) != 0 || len(res.Tasks) != 1 || res.Tasks[0].ParentID != res.Plan.ID {
		t.Fatalf("unexpected tree: %+v", res)
	}
}

func TestCreatePlanTree_MixedChildren_Rejected(t *testing.T) {
	env := newPlanTreeEnv()
	in := basePlanTreeInput()
	in.Tasks = []PlanTreeTask{{Title: "x"}}
	in.Phases = []PlanTreePhase{{Title: "p"}}
	if _, err := env.uc.Execute(planTreeCtx(), in); errCode(err) != "TASK_PLAN_TREE_MIXED_CHILDREN" {
		t.Fatalf("want MIXED_CHILDREN, got %v", err)
	}
}

func TestCreatePlanTree_InvalidInput_Rejected(t *testing.T) {
	env := newPlanTreeEnv()
	cases := map[string]func(*CreatePlanTreeInput){
		"no request": func(in *CreatePlanTreeInput) { in.RequestID = "" },
		"no project": func(in *CreatePlanTreeInput) { in.ProjectID = "" },
		"no title":   func(in *CreatePlanTreeInput) { in.Title = "" },
		"bad index":  func(in *CreatePlanTreeInput) { in.Tasks = []PlanTreeTask{{Title: "a", DependsOnIndices: []int{3}}} },
		"self dep":   func(in *CreatePlanTreeInput) { in.Tasks = []PlanTreeTask{{Title: "a", DependsOnIndices: []int{0}}} },
	}
	for name, mutate := range cases {
		in := basePlanTreeInput()
		mutate(&in)
		if _, err := env.uc.Execute(planTreeCtx(), in); errCode(err) != "TASK_PLAN_TREE_INVALID" {
			t.Errorf("%s: want TASK_PLAN_TREE_INVALID, got %v", name, err)
		}
	}
	if len(env.tasks.tasks) != 0 {
		t.Errorf("invalid input wrote %d tasks", len(env.tasks.tasks))
	}
}

func TestCreatePlanTree_MidTreeFailure_RollsBackAll(t *testing.T) {
	env := newPlanTreeEnv()
	n := 0
	env.tasks.createHook = func(task domain.Task) error {
		n++
		if n == 4 {
			return errors.New("disk full")
		}
		return nil
	}
	in := basePlanTreeInput()
	in.Phases = []PlanTreePhase{{Title: "P1", Tasks: []PlanTreeTask{{Title: "a"}, {Title: "b"}}}}
	if _, err := env.uc.Execute(planTreeCtx(), in); err == nil {
		t.Fatal("expected failure")
	}
	if len(env.tasks.tasks) != 0 || len(env.edges.edges) != 0 {
		t.Fatalf("rollback left %d tasks and %d edges", len(env.tasks.tasks), len(env.edges.edges))
	}
	if len(env.grants.grants) != 0 {
		t.Errorf("grant written for a failed tree")
	}
}

func TestCreatePlanTree_SecondCall_AlreadyExists(t *testing.T) {
	env := newPlanTreeEnv()
	in := basePlanTreeInput()
	in.Phases = []PlanTreePhase{{Title: "P1", Tasks: []PlanTreeTask{{Title: "a"}}}}
	first, err := env.uc.Execute(planTreeCtx(), in)
	if err != nil {
		t.Fatal(err)
	}
	second, err := env.uc.Execute(planTreeCtx(), in)
	if err != nil {
		t.Fatal(err)
	}
	if !second.AlreadyExists || second.Plan.ID != first.Plan.ID || len(second.Phases) != 1 || len(second.Tasks) != 1 {
		t.Fatalf("second call should return the existing tree: %+v", second)
	}
	if countByType(env.tasks, domain.TypePlan) != 1 {
		t.Errorf("a second plan was created")
	}
}

// racingRunner simulates losing the unique-index race: the winner commits while our tx fails.
type racingRunner struct {
	*fakeTxRunner
	winner func()
}

func (r *racingRunner) RunInTx(ctx context.Context, fn func(context.Context, TaskRepository, EdgeRepository) error) error {
	err := r.fakeTxRunner.RunInTx(ctx, fn)
	if err != nil && r.winner != nil {
		r.winner()
		r.winner = nil
	}
	return err
}

func TestCreatePlanTree_UniqueViolation_RereadsExisting(t *testing.T) {
	env := newPlanTreeEnv()
	winner := domain.Task{ID: "winner", TenantID: ptTenant, Title: "Plan", Type: domain.TypePlan, RequestID: ptRequest, ProjectID: ptProject, Status: domain.StatusOpen}
	env.tasks.createHook = func(task domain.Task) error {
		if task.Type == domain.TypePlan {
			return fmt.Errorf("pg: %w", ErrActivePlanExists)
		}
		return nil
	}
	runner := &racingRunner{fakeTxRunner: newFakeTxRunner(env.tasks, env.edges), winner: func() {
		env.tasks.tasks[winner.ID] = winner
		env.tasks.createHook = nil
	}}
	uc := NewCreatePlanTree(runner, env.tasks, env.grants)
	res, err := uc.Execute(planTreeCtx(), basePlanTreeInput())
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if !res.AlreadyExists || res.Plan.ID != "winner" {
		t.Fatalf("want the winner's plan, got %+v", res)
	}
}

func seedActivePlan(env planTreeEnv, leafStatus domain.Status) {
	env.tasks.tasks["old-plan"] = domain.Task{ID: "old-plan", TenantID: ptTenant, Type: domain.TypePlan, RequestID: ptRequest, ProjectID: ptProject, Status: domain.StatusOpen}
	env.tasks.tasks["old-phase"] = domain.Task{ID: "old-phase", TenantID: ptTenant, Type: domain.TypePhase, ParentID: "old-plan", RequestID: ptRequest, ProjectID: ptProject, Status: domain.StatusOpen}
	env.tasks.tasks["old-done"] = domain.Task{ID: "old-done", TenantID: ptTenant, Type: domain.TypeTask, ParentID: "old-phase", RequestID: ptRequest, Status: domain.StatusDone}
	env.tasks.tasks["old-leaf"] = domain.Task{ID: "old-leaf", TenantID: ptTenant, Type: domain.TypeTask, ParentID: "old-phase", RequestID: ptRequest, Status: leafStatus}
}

func TestCreatePlanTree_Supersede_CancelsOldTree(t *testing.T) {
	env := newPlanTreeEnv()
	seedActivePlan(env, domain.StatusOpen)
	in := basePlanTreeInput()
	in.SupersedesPlanID = "old-plan"
	in.Tasks = []PlanTreeTask{{Title: "new"}}
	res, err := env.uc.Execute(planTreeCtx(), in)
	if err != nil {
		t.Fatal(err)
	}
	if res.AlreadyExists || res.Plan.ID == "old-plan" {
		t.Fatalf("expected a new plan: %+v", res)
	}
	for id, want := range map[string]domain.Status{"old-plan": domain.StatusCancelled, "old-phase": domain.StatusCancelled, "old-leaf": domain.StatusCancelled, "old-done": domain.StatusDone} {
		if got := env.tasks.tasks[id].Status; got != want {
			t.Errorf("%s: status %s, want %s", id, got, want)
		}
	}
}

func TestCreatePlanTree_Supersede_RunningTask_Rejected(t *testing.T) {
	env := newPlanTreeEnv()
	seedActivePlan(env, domain.StatusInProgress)
	in := basePlanTreeInput()
	in.SupersedesPlanID = "old-plan"
	if _, err := env.uc.Execute(planTreeCtx(), in); errCode(err) != "TASK_PLAN_HAS_RUNNING_TASKS" {
		t.Fatalf("want HAS_RUNNING_TASKS, got %v", err)
	}
	if env.tasks.tasks["old-plan"].Status != domain.StatusOpen || countByType(env.tasks, domain.TypePlan) != 1 {
		t.Errorf("a rejected supersede must change nothing")
	}
}

func TestCreatePlanTree_SupersedeMismatch(t *testing.T) {
	env := newPlanTreeEnv()
	seedActivePlan(env, domain.StatusOpen)
	in := basePlanTreeInput()
	in.SupersedesPlanID = "some-other-plan"
	if _, err := env.uc.Execute(planTreeCtx(), in); errCode(err) != "TASK_PLAN_SUPERSEDE_MISMATCH" {
		t.Fatalf("want SUPERSEDE_MISMATCH, got %v", err)
	}
}

func TestCreatePlanTree_Supersede_RetryAfterSuccess_AlreadyExists(t *testing.T) {
	env := newPlanTreeEnv()
	seedActivePlan(env, domain.StatusOpen)
	in := basePlanTreeInput()
	in.SupersedesPlanID = "old-plan"
	first, err := env.uc.Execute(planTreeCtx(), in)
	if err != nil {
		t.Fatal(err)
	}
	again, err := env.uc.Execute(planTreeCtx(), in)
	if err != nil || !again.AlreadyExists || again.Plan.ID != first.Plan.ID {
		t.Fatalf("retry should be idempotent: %+v %v", again, err)
	}
}

func TestCreatePlanTree_PhaseDependsOnPhase_NoBlocked(t *testing.T) {
	env := newPlanTreeEnv()
	in := basePlanTreeInput()
	in.Phases = []PlanTreePhase{{Title: "P1"}, {Title: "P2", DependsOnPhaseIndices: []int{0}}}
	res, err := env.uc.Execute(planTreeCtx(), in)
	if err != nil {
		t.Fatal(err)
	}
	if res.Phases[1].Status == domain.StatusBlocked {
		t.Fatalf("phase must not be blocked by a phase dependency")
	}
}

func TestCreatePlanTree_TaskDependsOnTask_Blocked(t *testing.T) {
	env := newPlanTreeEnv()
	in := basePlanTreeInput()
	in.Tasks = []PlanTreeTask{{Title: "a"}, {Title: "b", DependsOnIndices: []int{0}}}
	res, err := env.uc.Execute(planTreeCtx(), in)
	if err != nil {
		t.Fatal(err)
	}
	if res.Tasks[0].Status == domain.StatusBlocked || res.Tasks[1].Status != domain.StatusBlocked {
		t.Fatalf("statuses: a=%s b=%s", res.Tasks[0].Status, res.Tasks[1].Status)
	}
}

func TestCreatePlanTree_Cycle_Rejected(t *testing.T) {
	env := newPlanTreeEnv()
	in := basePlanTreeInput()
	in.Tasks = []PlanTreeTask{{Title: "a", DependsOnIndices: []int{1}}, {Title: "b", DependsOnIndices: []int{0}}}
	if _, err := env.uc.Execute(planTreeCtx(), in); errCode(err) != "TASK_CYCLIC_DEPENDENCY" {
		t.Fatalf("want TASK_CYCLIC_DEPENDENCY, got %v", err)
	}
	if len(env.tasks.tasks) != 0 {
		t.Errorf("cycle left %d tasks behind", len(env.tasks.tasks))
	}
}

func TestCreatePlanTree_GrantsOwnerAfterCommit(t *testing.T) {
	env := newPlanTreeEnv()
	res, err := env.uc.Execute(planTreeCtx(), basePlanTreeInput())
	if err != nil {
		t.Fatal(err)
	}
	if len(env.grants.grants) != 1 {
		t.Fatalf("want 1 grant, got %d", len(env.grants.grants))
	}
	g := env.grants.grants[0]
	if g.TaskID != res.Plan.ID || g.SubjectID != "user-9" || g.Level != domain.GrantLevelOwner || !g.ApplyTree {
		t.Errorf("bad grant: %+v", g)
	}
}

func TestCreatePlanTree_GrantFailure_DoesNotFail(t *testing.T) {
	env := newPlanTreeEnv()
	env.grants.grantErr = errors.New("grant store down")
	res, err := env.uc.Execute(planTreeCtx(), basePlanTreeInput())
	if err != nil || res.Plan.ID == "" {
		t.Fatalf("a failed grant must not fail the create: %v", err)
	}
}

func TestCreatePlanTree_TxConflict_RetriesWholeTx(t *testing.T) {
	env := newPlanTreeEnv()
	calls := 0
	env.tasks.createHook = func(task domain.Task) error {
		if task.Type == domain.TypePlan {
			calls++
			if calls == 1 {
				return fmt.Errorf("deadlock: %w", ErrTxConflict)
			}
		}
		return nil
	}
	res, err := env.uc.Execute(planTreeCtx(), basePlanTreeInput())
	if err != nil || res.AlreadyExists || calls != 2 {
		t.Fatalf("res=%+v err=%v calls=%d", res, err, calls)
	}
	if countByType(env.tasks, domain.TypePlan) != 1 {
		t.Error("retry duplicated the plan")
	}
}
