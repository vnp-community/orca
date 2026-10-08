//go:build integration

package postgres

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/task-service/internal/domain"
	"github.com/stablyai/orca-go/services/task-service/internal/usecase"
)

func tenantOutboxCount(t *testing.T, repo *Repository, tenantID string) int {
	t.Helper()
	var n int
	if err := repo.pool.QueryRow(context.Background(), `SELECT count(*) FROM task.outbox_events WHERE tenant_id = $1`, tenantID).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

func runEvent(id string) domain.OutboxEvent {
	return domain.OutboxEvent{ID: id, Subject: "orca.task.task.statuschanged", OccurredAt: time.Now().UTC(), PayloadJSON: []byte(`{"cause":"x"}`)}
}

func newRunTask(t *testing.T, repo *Repository, tenantID string, status domain.Status) domain.Task {
	t.Helper()
	task, err := domain.NewTask(uuid.NewString(), tenantID, "t", status, "", "")
	if err != nil {
		t.Fatal(err)
	}
	task.RequestID = uuid.NewString()
	if _, err := repo.Create(context.Background(), task); err != nil {
		t.Fatal(err)
	}
	return task
}

func TestClaimForExecution_WritesOutboxInSameTx(t *testing.T) {
	repo := setupRepository(t)
	tenantID := uuid.NewString()
	task := newRunTask(t, repo, tenantID, domain.StatusOpen)
	ok, err := repo.ClaimForExecution(context.Background(), tenantID, task.ID, domain.StatusOpen, []domain.OutboxEvent{runEvent(uuid.NewString())})
	if err != nil || !ok {
		t.Fatalf("claim: ok=%v err=%v", ok, err)
	}
	if n := tenantOutboxCount(t, repo, tenantID); n != 1 {
		t.Fatalf("want 1 outbox row, got %d", n)
	}
}

func TestClaimForExecution_LostRace_NoOutbox(t *testing.T) {
	repo := setupRepository(t)
	tenantID := uuid.NewString()
	task := newRunTask(t, repo, tenantID, domain.StatusOpen)
	ctx := context.Background()
	var wg sync.WaitGroup
	var mu sync.Mutex
	wins := 0
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			ok, err := repo.ClaimForExecution(ctx, tenantID, task.ID, domain.StatusOpen, []domain.OutboxEvent{runEvent(uuid.NewString())})
			if err != nil {
				t.Errorf("claim: %v", err)
			}
			if ok {
				mu.Lock()
				wins++
				mu.Unlock()
			}
		}()
	}
	wg.Wait()
	if wins != 1 || tenantOutboxCount(t, repo, tenantID) != 1 {
		t.Fatalf("wins=%d outbox=%d, want exactly 1 each", wins, tenantOutboxCount(t, repo, tenantID))
	}
}

func TestReleaseExecution_StaleLink_NoOutbox(t *testing.T) {
	repo := setupRepository(t)
	tenantID := uuid.NewString()
	taskID, linkID := newActiveRun(t, repo, tenantID, domain.EngineOrchestration)
	ctx := context.Background()
	if ok, err := repo.ReleaseExecution(ctx, tenantID, taskID, uuid.NewString(), domain.StatusOpen, []domain.OutboxEvent{runEvent(uuid.NewString())}); err != nil || ok {
		t.Fatalf("stale link: ok=%v err=%v", ok, err)
	}
	if tenantOutboxCount(t, repo, tenantID) != 0 {
		t.Fatal("stale release wrote outbox")
	}
	if ok, err := repo.ReleaseExecution(ctx, tenantID, taskID, linkID, domain.StatusOpen, []domain.OutboxEvent{runEvent(uuid.NewString())}); err != nil || !ok {
		t.Fatalf("release: ok=%v err=%v", ok, err)
	}
	if tenantOutboxCount(t, repo, tenantID) != 1 {
		t.Fatal("matching release must write one outbox row")
	}
}

func TestCompleteExecution_WritesOutboxInSameTx(t *testing.T) {
	repo := setupRepository(t)
	tenantID := uuid.NewString()
	task := newRunTask(t, repo, tenantID, domain.StatusInProgress)
	if err := repo.CompleteExecution(context.Background(), tenantID, task.ID, "review", 1, []domain.OutboxEvent{runEvent(uuid.NewString())}); err != nil {
		t.Fatal(err)
	}
	got, _ := repo.Get(context.Background(), tenantID, task.ID)
	if got.Status != domain.StatusReview || tenantOutboxCount(t, repo, tenantID) != 1 {
		t.Fatalf("status=%s outbox=%d", got.Status, tenantOutboxCount(t, repo, tenantID))
	}
}

func TestExecutionWrite_OutboxFailure_StatusUnchanged(t *testing.T) {
	repo := setupRepository(t)
	tenantID := uuid.NewString()
	ctx := context.Background()
	dup := uuid.NewString() // second insert violates the primary key and must roll the UPDATE back
	events := []domain.OutboxEvent{runEvent(dup), runEvent(dup)}

	open := newRunTask(t, repo, tenantID, domain.StatusOpen)
	if ok, err := repo.ClaimForExecution(ctx, tenantID, open.ID, domain.StatusOpen, events); err == nil || ok {
		t.Fatalf("claim should fail: ok=%v err=%v", ok, err)
	}
	if got, _ := repo.Get(ctx, tenantID, open.ID); got.Status != domain.StatusOpen {
		t.Errorf("claim leaked status %s", got.Status)
	}
	running := newRunTask(t, repo, tenantID, domain.StatusInProgress)
	if err := repo.CompleteExecution(ctx, tenantID, running.ID, "review", 1, events); err == nil {
		t.Fatal("complete should fail")
	}
	if got, _ := repo.Get(ctx, tenantID, running.ID); got.Status != domain.StatusInProgress {
		t.Errorf("complete leaked status %s", got.Status)
	}
	if tenantOutboxCount(t, repo, tenantID) != 0 {
		t.Error("failed writes left outbox rows")
	}
}

func TestExecution_RequestTaskLifecycle_TwoOutboxRowsInOrder(t *testing.T) {
	repo := setupRepository(t)
	tenantID := uuid.NewString()
	ctx := context.Background()
	task := newRunTask(t, repo, tenantID, domain.StatusOpen)
	first, second := runEvent(uuid.NewString()), runEvent(uuid.NewString())
	second.OccurredAt = first.OccurredAt.Add(time.Second)
	if ok, err := repo.ClaimForExecution(ctx, tenantID, task.ID, domain.StatusOpen, []domain.OutboxEvent{first}); err != nil || !ok {
		t.Fatal(ok, err)
	}
	if err := repo.CompleteExecution(ctx, tenantID, task.ID, "review", 1, []domain.OutboxEvent{second}); err != nil {
		t.Fatal(err)
	}
	rows, err := repo.pool.Query(ctx, `SELECT id::text FROM task.outbox_events WHERE tenant_id = $1 ORDER BY occurred_at`, tenantID)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var ids []string
	for rows.Next() {
		var id string
		_ = rows.Scan(&id)
		ids = append(ids, id)
	}
	if len(ids) != 2 || ids[0] != first.ID || ids[1] != second.ID {
		t.Fatalf("order: %v", ids)
	}
}

// --- CreatePlanTree ---

func planCtx(tenantID string) context.Context {
	return tenant.WithUserID(tenant.WithTenantID(context.Background(), tenantID), uuid.NewString())
}

func twelveTaskInput(requestID, projectID string) usecase.CreatePlanTreeInput {
	in := usecase.CreatePlanTreeInput{RequestID: requestID, ProjectID: projectID, Title: "Plan", CreatorID: uuid.NewString()}
	for p := 0; p < 3; p++ {
		ph := usecase.PlanTreePhase{Title: "Phase"}
		if p > 0 {
			ph.DependsOnPhaseIndices = []int{p - 1}
		}
		for k := 0; k < 4; k++ {
			task := usecase.PlanTreeTask{Title: "Task", Labels: []string{"l"}}
			if k > 0 {
				task.DependsOnIndices = []int{k - 1}
			}
			ph.Tasks = append(ph.Tasks, task)
		}
		in.Phases = append(in.Phases, ph)
	}
	return in
}

func TestCreatePlanTree_Integration_FullTree(t *testing.T) {
	repo := setupRepository(t)
	tenantID, requestID, projectID := uuid.NewString(), uuid.NewString(), uuid.NewString()
	uc := usecase.NewCreatePlanTree(repo, repo, repo)
	in := twelveTaskInput(requestID, projectID)
	res, err := uc.Execute(planCtx(tenantID), in)
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Phases) != 3 || len(res.Tasks) != 12 || res.AlreadyExists {
		t.Fatalf("phases=%d tasks=%d", len(res.Phases), len(res.Tasks))
	}
	if res.Plan.TaskNumber != 0 {
		t.Errorf("plan must have no task number")
	}
	for _, ph := range res.Phases {
		if ph.TaskNumber != 0 || ph.RequestID != requestID || ph.Status == domain.StatusBlocked {
			t.Errorf("bad phase: %+v", ph)
		}
	}
	blocked := 0
	for _, task := range res.Tasks {
		if task.RequestID != requestID || task.TaskNumber == 0 {
			t.Errorf("bad task: %+v", task)
		}
		if task.Status == domain.StatusBlocked {
			blocked++
		}
	}
	if blocked != 9 { // every task but the first of each phase depends on its predecessor
		t.Errorf("blocked=%d want 9", blocked)
	}
	edges, err := repo.ListByKind(context.Background(), tenantID, domain.EdgeKindDependsOn)
	if err != nil || len(edges) != 11 { // 9 task edges + 2 phase edges
		t.Errorf("depends_on edges=%d err=%v", len(edges), err)
	}
	again, err := uc.Execute(planCtx(tenantID), in)
	if err != nil || !again.AlreadyExists || again.Plan.ID != res.Plan.ID || len(again.Tasks) != 12 {
		t.Fatalf("second call: %+v %v", again, err)
	}
}

type failNthCreate struct {
	*Repository
	n int
}

func (f *failNthCreate) RunInTx(ctx context.Context, fn func(context.Context, usecase.TaskRepository, usecase.EdgeRepository) error) error {
	return f.Repository.RunInTx(ctx, func(ctx context.Context, tasks usecase.TaskRepository, edges usecase.EdgeRepository) error {
		return fn(ctx, &failingTasks{TaskRepository: tasks, left: f.n}, edges)
	})
}

type failingTasks struct {
	usecase.TaskRepository
	left int
}

func (f *failingTasks) Create(ctx context.Context, task domain.Task) (domain.Task, error) {
	f.left--
	if f.left == 0 {
		return domain.Task{}, errors.New("injected failure")
	}
	return f.TaskRepository.Create(ctx, task)
}

func TestCreatePlanTree_Integration_RollbackLeavesNothing(t *testing.T) {
	repo := setupRepository(t)
	tenantID, requestID := uuid.NewString(), uuid.NewString()
	uc := usecase.NewCreatePlanTree(&failNthCreate{Repository: repo, n: 8}, repo, repo)
	if _, err := uc.Execute(planCtx(tenantID), twelveTaskInput(requestID, uuid.NewString())); err == nil {
		t.Fatal("expected failure")
	}
	var n int
	if err := repo.pool.QueryRow(context.Background(), `SELECT count(*) FROM task.tasks WHERE tenant_id = $1`, tenantID).Scan(&n); err != nil || n != 0 {
		t.Fatalf("rows left: %d %v", n, err)
	}
	if err := repo.pool.QueryRow(context.Background(), `SELECT count(*) FROM task.task_edges WHERE tenant_id = $1`, tenantID).Scan(&n); err != nil || n != 0 {
		t.Fatalf("edges left: %d %v", n, err)
	}
}

func TestCreatePlanTree_Integration_ConcurrentSameRequest_OnePlan(t *testing.T) {
	repo := setupRepository(t)
	tenantID, requestID, projectID := uuid.NewString(), uuid.NewString(), uuid.NewString()
	uc := usecase.NewCreatePlanTree(repo, repo, repo)
	in := twelveTaskInput(requestID, projectID)
	var wg sync.WaitGroup
	var mu sync.Mutex
	fresh, existing := 0, 0
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			res, err := uc.Execute(planCtx(tenantID), in)
			if err != nil {
				t.Errorf("Execute: %v", err)
				return
			}
			mu.Lock()
			defer mu.Unlock()
			if res.AlreadyExists {
				existing++
			} else {
				fresh++
			}
		}()
	}
	wg.Wait()
	if fresh != 1 || existing != 7 {
		t.Fatalf("fresh=%d existing=%d", fresh, existing)
	}
	var n int
	_ = repo.pool.QueryRow(context.Background(), `SELECT count(*) FROM task.tasks WHERE tenant_id = $1 AND task_type = 'plan' AND status <> 'cancelled'`, tenantID).Scan(&n)
	if n != 1 {
		t.Fatalf("active plans=%d", n)
	}
	_ = repo.pool.QueryRow(context.Background(), `SELECT count(*) FROM task.tasks WHERE tenant_id = $1`, tenantID).Scan(&n)
	if n != 16 {
		t.Fatalf("total tasks=%d, a losing tx leaked rows", n)
	}
}

func TestCreatePlanTree_Integration_SupersedeThenRecreate(t *testing.T) {
	repo := setupRepository(t)
	tenantID, requestID, projectID := uuid.NewString(), uuid.NewString(), uuid.NewString()
	uc := usecase.NewCreatePlanTree(repo, repo, repo)
	first, err := uc.Execute(planCtx(tenantID), twelveTaskInput(requestID, projectID))
	if err != nil {
		t.Fatal(err)
	}
	in := twelveTaskInput(requestID, projectID)
	in.SupersedesPlanID = first.Plan.ID
	second, err := uc.Execute(planCtx(tenantID), in)
	if err != nil || second.AlreadyExists || second.Plan.ID == first.Plan.ID {
		t.Fatalf("supersede: %+v %v", second, err)
	}
	old, _ := repo.Get(context.Background(), tenantID, first.Plan.ID)
	oldLeaf, _ := repo.Get(context.Background(), tenantID, first.Tasks[0].ID)
	if old.Status != domain.StatusCancelled || oldLeaf.Status != domain.StatusCancelled {
		t.Fatalf("old tree not cancelled: plan=%s leaf=%s", old.Status, oldLeaf.Status)
	}
	// A running task blocks a further replan.
	if err := repo.UpdateStatus(context.Background(), tenantID, second.Tasks[0].ID, domain.StatusInProgress); err != nil {
		t.Fatal(err)
	}
	in.SupersedesPlanID = second.Plan.ID
	_, err = uc.Execute(planCtx(tenantID), in)
	var ae *apperrors.AppError
	if !errors.As(err, &ae) || ae.Code != "TASK_PLAN_HAS_RUNNING_TASKS" {
		t.Fatalf("want HAS_RUNNING_TASKS, got %v", err)
	}
}

type allowAllOPA struct{}

func (allowAllOPA) Decision(context.Context, domain.GrantLevel, string, string) (bool, error) {
	return true, nil
}

type noTeams struct{}

func (noTeams) ResolveTeams(context.Context, string, string) ([]string, error) { return nil, nil }

func TestCreatePlanTree_Integration_CreatorInheritsAccessThreeLevels(t *testing.T) {
	repo := setupRepository(t)
	tenantID := uuid.NewString()
	creator := uuid.NewString()
	in := twelveTaskInput(uuid.NewString(), uuid.NewString())
	in.CreatorID = creator
	res, err := usecase.NewCreatePlanTree(repo, repo, repo).Execute(planCtx(tenantID), in)
	if err != nil {
		t.Fatal(err)
	}
	rp := usecase.NewResolvePermission(repo, repo, noTeams{}, allowAllOPA{}, nil)
	ctx := tenant.WithTenantID(context.Background(), tenantID)
	leaf := res.Tasks[11] // plan -> phase -> task
	for _, action := range []string{"read", "execute"} {
		if _, err := rp.Execute(ctx, usecase.ResolvePermissionInput{TaskID: leaf.ID, UserID: creator, Action: action}); err != nil {
			t.Errorf("creator %s on leaf: %v", action, err)
		}
	}
	if _, err := rp.Execute(ctx, usecase.ResolvePermissionInput{TaskID: leaf.ID, UserID: uuid.NewString(), Action: "read"}); err == nil {
		t.Error("a stranger must not get access")
	}
}
