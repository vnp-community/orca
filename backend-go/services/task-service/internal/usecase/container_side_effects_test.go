package usecase

import (
	"context"
	"errors"
	"testing"

	"github.com/stablyai/orca-go/services/task-service/internal/domain"
)

func TestAddEdge_ContainerFrom_NotBlocked(t *testing.T) {
	tasks := newFakeTaskRepository()
	tasks.tasks["ph-2"] = domain.Task{ID: "ph-2", TenantID: "tenant-1", Type: domain.TypePhase, Status: domain.StatusOpen}
	tasks.tasks["ph-1"] = domain.Task{ID: "ph-1", TenantID: "tenant-1", Type: domain.TypePhase, Status: domain.StatusOpen}
	_, err := NewAddEdge(tasks, &fakeEdgeRepository{}).Execute(withIdentity(context.Background(), "tenant-1", "u"),
		AddEdgeInput{FromTaskID: "ph-2", ToTaskID: "ph-1", Kind: domain.EdgeKindDependsOn})
	if err != nil {
		t.Fatal(err)
	}
	if got := tasks.tasks["ph-2"].Status; got != domain.StatusOpen {
		t.Errorf("phase must not be blocked, got %s", got)
	}
	if len(tasks.updateStatusCalls) != 0 {
		t.Errorf("no status write expected, got %+v", tasks.updateStatusCalls)
	}
}

func TestAddEdge_WorkTaskFrom_StillBlocked(t *testing.T) {
	tasks := newFakeTaskRepository()
	tasks.tasks["a"] = domain.Task{ID: "a", TenantID: "tenant-1", Type: domain.TypeTask, Status: domain.StatusOpen}
	tasks.tasks["b"] = domain.Task{ID: "b", TenantID: "tenant-1", Type: domain.TypeTask, Status: domain.StatusOpen}
	if _, err := NewAddEdge(tasks, &fakeEdgeRepository{}).Execute(withIdentity(context.Background(), "tenant-1", "u"),
		AddEdgeInput{FromTaskID: "a", ToTaskID: "b", Kind: domain.EdgeKindDependsOn}); err != nil {
		t.Fatal(err)
	}
	if tasks.tasks["a"].Status != domain.StatusBlocked {
		t.Errorf("work task should be blocked, got %s", tasks.tasks["a"].Status)
	}
}

func TestExecuteTask_Container_NotExecutable(t *testing.T) {
	for _, typ := range []string{domain.TypePlan, domain.TypePhase} {
		tasks := newFakeTaskRepository()
		grants := &fakeGrantRepository{}
		seedExecutableTask(t, tasks, grants, "c-1", "proj-1")
		c := tasks.tasks["c-1"]
		c.Type = typ
		tasks.tasks["c-1"] = c
		simple := &fakeSimpleExecutor{}
		uc, _, _, _, links := newExecutableExecuteTask(tasks, &fakeEdgeRepository{}, simple, &fakeExecutor{}, grants)
		_, err := uc.Execute(withIdentity(context.Background(), "tenant-1", "user-1"), ExecuteTaskInput{TaskID: "c-1"})
		if !hasCode(err, "TASK_EXECUTE_CONTAINER_NOT_EXECUTABLE") {
			t.Fatalf("%s: got %v", typ, err)
		}
		if len(links.created) != 0 || len(tasks.updateStatusCalls) != 0 || simple.called {
			t.Errorf("%s: must not write links/status or dispatch", typ)
		}
	}
}

// seedLeafWithSubtaskEdge makes leaf-1 "complex" (a parent_child edge from it) so Execute takes
// the async orchestration path and leaves the leaf in_progress after dispatch.
func seedLeafWithSubtaskEdge(repo *fakeTaskRepository, grants *fakeGrantRepository) *fakeEdgeRepository {
	seedPlanPhaseLeaves(repo)
	grants.grants = append(grants.grants, domain.Grant{TaskID: "leaf-1", SubjectID: "user-1", Level: domain.GrantLevelOwner})
	return &fakeEdgeRepository{edges: []domain.TaskEdge{{FromTaskID: "leaf-1", ToTaskID: "x", Kind: domain.EdgeKindParentChild}}}
}

func TestExecuteTask_ClaimSyncsPhase_InProgress(t *testing.T) {
	repo := newFakeTaskRepository()
	grants := &fakeGrantRepository{}
	edges := seedLeafWithSubtaskEdge(repo, grants)
	uc, _, _, _, _ := newExecutableExecuteTask(repo, edges, &fakeSimpleExecutor{}, &fakeExecutor{ref: "run-1"}, grants)
	uc.WithContainerSync(NewSyncContainerStatus(repo, nil))
	if _, err := uc.Execute(withIdentity(context.Background(), "tenant-1", "user-1"), ExecuteTaskInput{TaskID: "leaf-1"}); err != nil {
		t.Fatal(err)
	}
	if repo.tasks["phase-1"].Status != domain.StatusInProgress || repo.tasks["plan-1"].Status != domain.StatusInProgress {
		t.Fatalf("phase=%s plan=%s", repo.tasks["phase-1"].Status, repo.tasks["plan-1"].Status)
	}
}

func TestExecuteTask_RevertSyncsPhase_BackToOpen(t *testing.T) {
	repo := newFakeTaskRepository()
	grants := &fakeGrantRepository{}
	edges := seedLeafWithSubtaskEdge(repo, grants)
	uc, _, _, _, _ := newExecutableExecuteTask(repo, edges, &fakeSimpleExecutor{}, &fakeExecutor{err: errors.New("down")}, grants)
	uc.WithContainerSync(NewSyncContainerStatus(repo, nil))
	if _, err := uc.Execute(withIdentity(context.Background(), "tenant-1", "user-1"), ExecuteTaskInput{TaskID: "leaf-1"}); err == nil {
		t.Fatal("dispatch failure must propagate")
	}
	if repo.tasks["leaf-1"].Status != domain.StatusOpen || repo.tasks["phase-1"].Status != domain.StatusOpen || repo.tasks["plan-1"].Status != domain.StatusOpen {
		t.Fatalf("revert must leave everything open: leaf=%s phase=%s plan=%s", repo.tasks["leaf-1"].Status, repo.tasks["phase-1"].Status, repo.tasks["plan-1"].Status)
	}
	// claim (open->in_progress) and revert (in_progress->open) each emit one event for phase and plan.
	if len(repo.containerEvents) != 4 {
		t.Errorf("expected 4 derived events, got %d", len(repo.containerEvents))
	}
}

func TestExecuteTask_DirectAgentCompletionSyncsPhase_Review(t *testing.T) {
	repo := newFakeTaskRepository()
	grants := &fakeGrantRepository{}
	seedPlanPhaseLeaves(repo)
	setStatus(repo, "leaf-2", domain.StatusDone)
	grants.grants = append(grants.grants, domain.Grant{TaskID: "leaf-1", SubjectID: "user-1", Level: domain.GrantLevelOwner})
	uc, _, _, _, _ := newExecutableExecuteTask(repo, &fakeEdgeRepository{}, &fakeSimpleExecutor{ref: "r"}, &fakeExecutor{}, grants)
	uc.WithContainerSync(NewSyncContainerStatus(repo, nil))
	if _, err := uc.Execute(withIdentity(context.Background(), "tenant-1", "user-1"), ExecuteTaskInput{TaskID: "leaf-1"}); err != nil {
		t.Fatal(err)
	}
	if repo.tasks["leaf-1"].Status != domain.StatusReview {
		t.Fatalf("leaf=%s", repo.tasks["leaf-1"].Status)
	}
	if repo.tasks["phase-1"].Status != domain.StatusReview || repo.tasks["plan-1"].Status != domain.StatusReview {
		t.Fatalf("phase=%s plan=%s", repo.tasks["phase-1"].Status, repo.tasks["plan-1"].Status)
	}
}

func TestReportExecutionResult_SuccessSyncsPhase_Review(t *testing.T) {
	repo := newFakeTaskRepository()
	seedPlanPhaseLeaves(repo)
	setStatus(repo, "leaf-1", domain.StatusInProgress)
	setStatus(repo, "leaf-2", domain.StatusDone)
	links := &fakeExecutionLinkRepository{}
	seedTaskWithActiveLink(repo, links, "leaf-1", domain.EngineOrchestration, "run-1")
	uc := NewReportTaskExecutionResult(repo, links).WithContainerSync(NewSyncContainerStatus(repo, nil))
	if err := uc.Execute(withIdentity(context.Background(), "tenant-1", "u"), ReportTaskExecutionResultInput{TaskID: "leaf-1", ExecutionRef: "run-1", Engine: "orchestration", Success: true}); err != nil {
		t.Fatal(err)
	}
	if repo.tasks["phase-1"].Status != domain.StatusReview || repo.tasks["plan-1"].Status != domain.StatusReview {
		t.Fatalf("phase=%s plan=%s", repo.tasks["phase-1"].Status, repo.tasks["plan-1"].Status)
	}
}

func TestReportExecutionResult_FailureSyncsPhase_BackToOpen(t *testing.T) {
	repo := newFakeTaskRepository()
	seedPlanPhaseLeaves(repo)
	setStatus(repo, "leaf-1", domain.StatusInProgress)
	setStatus(repo, "phase-1", domain.StatusInProgress)
	setStatus(repo, "plan-1", domain.StatusInProgress)
	links := &fakeExecutionLinkRepository{}
	seedTaskWithActiveLink(repo, links, "leaf-1", domain.EngineOrchestration, "run-1")
	leases := &fakeLeaseRepository{tasks: repo}
	uc := NewReportTaskExecutionResult(repo, links).WithExecutionRelease(leases).WithContainerSync(NewSyncContainerStatus(repo, nil))
	if err := uc.Execute(withIdentity(context.Background(), "tenant-1", "u"), ReportTaskExecutionResultInput{TaskID: "leaf-1", ExecutionRef: "run-1", Engine: "orchestration", Success: false}); err != nil {
		t.Fatal(err)
	}
	if repo.tasks["leaf-1"].Status != domain.StatusOpen || repo.tasks["phase-1"].Status != domain.StatusOpen {
		t.Fatalf("leaf=%s phase=%s", repo.tasks["leaf-1"].Status, repo.tasks["phase-1"].Status)
	}
}

func TestUpdateTask_SyncsParentContainer(t *testing.T) {
	repo := newFakeTaskRepository()
	seedPlanPhaseLeaves(repo)
	uc := NewUpdateTask(repo, &fakeEdgeRepository{}).WithContainerSync(NewSyncContainerStatus(repo, nil))
	ctx := withIdentity(context.Background(), "tenant-1", "u")
	for _, id := range []string{"leaf-1", "leaf-2"} {
		done := domain.StatusDone
		if _, err := uc.Execute(ctx, UpdateTaskInput{ID: id, Status: &done}); err != nil {
			t.Fatal(err)
		}
	}
	if repo.tasks["phase-1"].Status != domain.StatusDone || repo.tasks["plan-1"].Status != domain.StatusDone {
		t.Fatalf("phase=%s plan=%s", repo.tasks["phase-1"].Status, repo.tasks["plan-1"].Status)
	}
}

func TestSyncFailure_DoesNotFailUpdateTask(t *testing.T) {
	repo := newFakeTaskRepository()
	seedPlanPhaseLeaves(repo)
	repo.listChildStatusesErr = errors.New("db down")
	uc := NewUpdateTask(repo, &fakeEdgeRepository{}).WithContainerSync(NewSyncContainerStatus(repo, nil))
	done := domain.StatusDone
	got, err := uc.Execute(withIdentity(context.Background(), "tenant-1", "u"), UpdateTaskInput{ID: "leaf-1", Status: &done})
	if err != nil || got.Status != domain.StatusDone {
		t.Fatalf("sync failure must not fail the command: %v", err)
	}
}

func TestUpdateTask_ContainerDone_Rejected(t *testing.T) {
	assertContainerStatusRejected(t, domain.StatusDone)
}

func TestUpdateTask_ContainerInProgress_Rejected(t *testing.T) {
	assertContainerStatusRejected(t, domain.StatusInProgress)
}

func TestUpdateTask_ContainerOpen_Rejected(t *testing.T) {
	assertContainerStatusRejected(t, domain.StatusOpen)
}

func assertContainerStatusRejected(t *testing.T, s domain.Status) {
	t.Helper()
	repo := newFakeTaskRepository()
	seedPlanPhaseLeaves(repo)
	_, err := NewUpdateTask(repo, &fakeEdgeRepository{}).Execute(withIdentity(context.Background(), "tenant-1", "u"), UpdateTaskInput{ID: "phase-1", Status: &s})
	if !hasCode(err, "TASK_CONTAINER_STATUS_DERIVED") {
		t.Fatalf("status %s: got %v", s, err)
	}
	if repo.tasks["phase-1"].Status != domain.StatusOpen {
		t.Errorf("status must be unchanged, got %s", repo.tasks["phase-1"].Status)
	}
}

func TestUpdateTask_ContainerCancelled_OK(t *testing.T) {
	repo := newFakeTaskRepository()
	seedPlanPhaseLeaves(repo)
	c := domain.StatusCancelled
	got, err := NewUpdateTask(repo, &fakeEdgeRepository{}).Execute(withIdentity(context.Background(), "tenant-1", "u"), UpdateTaskInput{ID: "phase-1", Status: &c})
	if err != nil || got.Status != domain.StatusCancelled {
		t.Fatalf("got %+v, %v", got, err)
	}
}

func TestUpdateTask_ContainerTitle_OK(t *testing.T) {
	repo := newFakeTaskRepository()
	seedPlanPhaseLeaves(repo)
	title := "renamed"
	got, err := NewUpdateTask(repo, &fakeEdgeRepository{}).Execute(withIdentity(context.Background(), "tenant-1", "u"), UpdateTaskInput{ID: "phase-1", Title: &title})
	if err != nil || got.Title != "renamed" {
		t.Fatalf("got %+v, %v", got, err)
	}
}

type fakeContainerReconcileRepo struct {
	stale   []domain.Task
	touched []string
}

func (f *fakeContainerReconcileRepo) ListStaleContainers(ctx context.Context, limit int) ([]domain.Task, error) {
	return f.stale, nil
}
func (f *fakeContainerReconcileRepo) TouchContainer(ctx context.Context, tenantID, id string) error {
	f.touched = append(f.touched, id)
	return nil
}

func TestReconcile_FixesStalePhase(t *testing.T) {
	repo := newFakeTaskRepository()
	seedPlanPhaseLeaves(repo)
	setStatus(repo, "phase-1", domain.StatusInProgress) // leaf was released to open by a bulk sweep
	setStatus(repo, "plan-1", domain.StatusInProgress)
	rr := &fakeContainerReconcileRepo{stale: []domain.Task{repo.tasks["phase-1"]}}
	n, err := NewReconcileContainerStatuses(rr, NewSyncContainerStatus(repo, nil)).Execute(context.Background())
	if err != nil || n != 1 {
		t.Fatalf("n=%d err=%v", n, err)
	}
	if repo.tasks["phase-1"].Status != domain.StatusOpen || repo.tasks["plan-1"].Status != domain.StatusOpen {
		t.Fatalf("phase=%s plan=%s", repo.tasks["phase-1"].Status, repo.tasks["plan-1"].Status)
	}
	if len(rr.touched) != 0 {
		t.Errorf("a repaired container needs no touch: %v", rr.touched)
	}
}

func TestReconcile_TouchesWhenUnchanged(t *testing.T) {
	repo := newFakeTaskRepository()
	seedPlanPhaseLeaves(repo)
	rr := &fakeContainerReconcileRepo{stale: []domain.Task{repo.tasks["phase-1"]}}
	n, err := NewReconcileContainerStatuses(rr, NewSyncContainerStatus(repo, nil)).Execute(context.Background())
	if err != nil || n != 0 {
		t.Fatalf("n=%d err=%v", n, err)
	}
	if len(rr.touched) != 1 || rr.touched[0] != "phase-1" {
		t.Errorf("unchanged container must be touched, got %v", rr.touched)
	}
}

func TestReconcile_ReleaseUnlinked_FixesPhase(t *testing.T) {
	repo := newFakeTaskRepository()
	seedPlanPhaseLeaves(repo)
	setStatus(repo, "leaf-1", domain.StatusInProgress) // no link: the bulk sweep puts it back to open
	setStatus(repo, "phase-1", domain.StatusInProgress)
	setStatus(repo, "plan-1", domain.StatusInProgress)
	leases := &fakeLeaseRepository{tasks: repo}
	rr := &fakeContainerReconcileRepo{stale: []domain.Task{repo.tasks["phase-1"]}}
	uc := NewRecoverInterruptedExecutions(leases).WithContainerReconcile(NewReconcileContainerStatuses(rr, NewSyncContainerStatus(repo, nil)))
	if err := uc.Sweep(context.Background()); err != nil {
		t.Fatal(err)
	}
	if repo.tasks["leaf-1"].Status != domain.StatusOpen {
		t.Fatalf("sweep should release leaf, got %s", repo.tasks["leaf-1"].Status)
	}
	if repo.tasks["phase-1"].Status != domain.StatusOpen || repo.tasks["plan-1"].Status != domain.StatusOpen {
		t.Errorf("reconcile must repair containers: phase=%s plan=%s", repo.tasks["phase-1"].Status, repo.tasks["plan-1"].Status)
	}
}

func TestRecoverInterruptedExecutions_ReleaseUnlinkedSkipsContainers(t *testing.T) {
	repo := newFakeTaskRepository()
	seedPlanPhaseLeaves(repo)
	setStatus(repo, "phase-1", domain.StatusInProgress)
	leases := &fakeLeaseRepository{tasks: repo}
	if _, err := leases.ReleaseUnlinkedInProgress(context.Background(), 0, 10); err != nil {
		t.Fatal(err)
	}
	if repo.tasks["phase-1"].Status != domain.StatusInProgress {
		t.Errorf("container must not be released, got %s", repo.tasks["phase-1"].Status)
	}
}
