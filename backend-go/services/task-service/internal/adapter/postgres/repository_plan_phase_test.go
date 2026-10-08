//go:build integration

package postgres

import (
	"context"
	"fmt"
	"sort"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/task-service/internal/domain"
	"github.com/stablyai/orca-go/services/task-service/internal/usecase"
)

type seed struct {
	typ, parent, project, request string
	status                        domain.Status
}

func mustCreate(t *testing.T, repo *Repository, tenantID string, s seed) domain.Task {
	t.Helper()
	if s.status == "" {
		s.status = domain.StatusOpen
	}
	task := domain.Task{ID: uuid.NewString(), TenantID: tenantID, Title: "t-" + s.typ, Status: s.status, Type: s.typ,
		ParentID: s.parent, ProjectID: s.project, RequestID: s.request, Labels: []string{}}
	created, err := repo.Create(context.Background(), task)
	if err != nil {
		t.Fatalf("create %s: %v", s.typ, err)
	}
	return created
}

func TestRepository_Create_ContainerHasNoTaskNumber(t *testing.T) {
	repo := setupRepository(t)
	tenantID, project := uuid.NewString(), uuid.NewString()
	ctx := context.Background()

	plan := mustCreate(t, repo, tenantID, seed{typ: "plan", project: project})
	phase := mustCreate(t, repo, tenantID, seed{typ: "phase", parent: plan.ID, project: project})
	epic := mustCreate(t, repo, tenantID, seed{typ: "epic", project: project})
	work := mustCreate(t, repo, tenantID, seed{typ: "task", project: project})

	if plan.TaskNumber != 0 || phase.TaskNumber != 0 {
		t.Errorf("containers must not receive a number: plan=%d phase=%d", plan.TaskNumber, phase.TaskNumber)
	}
	if epic.TaskNumber == 0 {
		t.Error("epic is a work task and must keep its task_number")
	}
	if work.TaskNumber != epic.TaskNumber+1 {
		t.Errorf("containers must not burn sequence values: epic=%d task=%d", epic.TaskNumber, work.TaskNumber)
	}
	var nullCount int
	if err := repo.pool.QueryRow(ctx, `SELECT count(*) FROM task.tasks WHERE tenant_id=$1 AND task_number IS NULL`, tenantID).Scan(&nullCount); err != nil || nullCount != 2 {
		t.Errorf("task_number must be NULL for plan and phase, got %d NULL rows (%v)", nullCount, err)
	}
	if _, err := repo.FindByNumber(ctx, tenantID, project, 0); err == nil {
		t.Error("FindByNumber(0) must never match a container")
	}
}

func TestRepository_Create_PersistsRequestID(t *testing.T) {
	repo := setupRepository(t)
	tenantID, project, req := uuid.NewString(), uuid.NewString(), uuid.NewString()
	ctx := context.Background()

	plan := mustCreate(t, repo, tenantID, seed{typ: "plan", project: project, request: req})
	child := mustCreate(t, repo, tenantID, seed{typ: "phase", parent: plan.ID, project: project, request: req})
	plain := mustCreate(t, repo, tenantID, seed{typ: "task", project: project})

	for _, c := range []struct {
		id, want string
	}{{plan.ID, req}, {child.ID, req}, {plain.ID, ""}} {
		got, err := repo.Get(ctx, tenantID, c.id)
		if err != nil || got.RequestID != c.want {
			t.Errorf("Get %s: request_id=%q err=%v want %q", c.id, got.RequestID, err, c.want)
		}
	}
	chain, err := repo.GetAncestors(ctx, tenantID, child.ID, 0)
	if err != nil || len(chain) != 2 || chain[0].RequestID != req || chain[1].RequestID != req {
		t.Errorf("GetAncestors request ids: %+v %v", chain, err)
	}
	listed, _, err := repo.List(ctx, tenantID, usecase.ListFilter{RequestIDs: []string{req}})
	if err != nil || len(listed) != 2 {
		t.Errorf("List by request: %d %v", len(listed), err)
	}
	sub, _, err := repo.GetSubtree(ctx, tenantID, plan.ID, 0)
	if err != nil || len(sub) != 2 || sub[0].RequestID != req {
		t.Errorf("GetSubtree must carry request_id: %+v %v", sub, err)
	}
	nodes, err := repo.GetSubtreeWithChildPercents(ctx, tenantID, plan.ID)
	if err != nil || len(nodes) != 2 {
		t.Errorf("GetSubtreeWithChildPercents: %d %v", len(nodes), err)
	}
}

func TestRepository_Update_DoesNotChangeRequestID(t *testing.T) {
	repo := setupRepository(t)
	tenantID, req := uuid.NewString(), uuid.NewString()
	ctx := context.Background()
	task := mustCreate(t, repo, tenantID, seed{typ: "task", request: req})
	task.Title = "renamed"
	task.RequestID = uuid.NewString() // an attempt to re-point must be ignored
	if err := repo.Update(ctx, tenantID, task, nil); err != nil {
		t.Fatal(err)
	}
	got, _ := repo.Get(ctx, tenantID, task.ID)
	if got.Title != "renamed" || got.RequestID != req {
		t.Errorf("request_id is immutable: %+v", got)
	}
}

func TestRepository_Create_ConcurrentTaskNumbersUnique(t *testing.T) {
	repo := setupRepository(t)
	tenantID, project := uuid.NewString(), uuid.NewString()
	var wg sync.WaitGroup
	var mu sync.Mutex
	var numbers []int64
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			typ := "task"
			if i%4 == 0 {
				typ = "phase" // interleaved containers must not consume numbers
			}
			task := domain.Task{ID: uuid.NewString(), TenantID: tenantID, Title: "c", Status: domain.StatusOpen, Type: typ, ProjectID: project, Labels: []string{}}
			created, err := repo.Create(context.Background(), task)
			if err != nil {
				t.Errorf("create: %v", err)
				return
			}
			if typ == "task" {
				mu.Lock()
				numbers = append(numbers, created.TaskNumber)
				mu.Unlock()
			} else if created.TaskNumber != 0 {
				t.Errorf("container got number %d", created.TaskNumber)
			}
		}(i)
	}
	wg.Wait()
	sort.Slice(numbers, func(a, b int) bool { return numbers[a] < numbers[b] })
	if len(numbers) != 15 {
		t.Fatalf("want 15 numbered tasks, got %d", len(numbers))
	}
	for i := 1; i < len(numbers); i++ {
		if numbers[i] != numbers[i-1]+1 {
			t.Fatalf("numbers must be unique and gapless despite containers: %v", numbers)
		}
	}
}

func TestRepository_List_FilterByType(t *testing.T) {
	repo := setupRepository(t)
	tenantID, project := uuid.NewString(), uuid.NewString()
	plan := mustCreate(t, repo, tenantID, seed{typ: "plan", project: project})
	mustCreate(t, repo, tenantID, seed{typ: "phase", parent: plan.ID, project: project})
	mustCreate(t, repo, tenantID, seed{typ: "task", project: project})
	mustCreate(t, repo, tenantID, seed{typ: "bug", project: project})
	ctx := context.Background()

	got, _, err := repo.List(ctx, tenantID, usecase.ListFilter{TaskTypes: []string{"plan", "phase"}})
	if err != nil || len(got) != 2 {
		t.Fatalf("plan+phase: %d %v", len(got), err)
	}
	got, _, err = repo.List(ctx, tenantID, usecase.ListFilter{TaskTypes: []string{"task", "bug", "feature", "epic"}})
	if err != nil || len(got) != 2 {
		t.Fatalf("work types: %d %v", len(got), err)
	}
	got, _, err = repo.List(ctx, tenantID, usecase.ListFilter{})
	if err != nil || len(got) != 4 {
		t.Fatalf("an unfiltered repository call returns everything (defaults are the usecase's job): %d %v", len(got), err)
	}
}

func TestRepository_List_FilterByRequestIDs(t *testing.T) {
	repo := setupRepository(t)
	tenantID, project := uuid.NewString(), uuid.NewString()
	r1, r2, r3 := uuid.NewString(), uuid.NewString(), uuid.NewString()
	mustCreate(t, repo, tenantID, seed{typ: "plan", project: project, request: r1})
	mustCreate(t, repo, tenantID, seed{typ: "plan", project: project, request: r2})
	mustCreate(t, repo, tenantID, seed{typ: "plan", project: project, request: r3})
	got, _, err := repo.List(context.Background(), tenantID, usecase.ListFilter{TaskTypes: []string{"plan"}, RequestIDs: []string{r1, r2}})
	if err != nil || len(got) != 2 {
		t.Fatalf("got %d %v", len(got), err)
	}
	for _, g := range got {
		if g.RequestID == r3 {
			t.Error("r3 must be excluded")
		}
	}
}

func TestRepository_List_FilterByParent(t *testing.T) {
	repo := setupRepository(t)
	tenantID, project := uuid.NewString(), uuid.NewString()
	plan := mustCreate(t, repo, tenantID, seed{typ: "plan", project: project})
	other := mustCreate(t, repo, tenantID, seed{typ: "plan", project: project})
	mustCreate(t, repo, tenantID, seed{typ: "phase", parent: plan.ID, project: project})
	mustCreate(t, repo, tenantID, seed{typ: "phase", parent: plan.ID, project: project})
	mustCreate(t, repo, tenantID, seed{typ: "phase", parent: other.ID, project: project})
	got, _, err := repo.List(context.Background(), tenantID, usecase.ListFilter{ParentID: plan.ID, ProjectID: project})
	if err != nil || len(got) != 2 {
		t.Fatalf("got %d %v", len(got), err)
	}
}

func TestRepository_List_PaginationStable(t *testing.T) {
	repo := setupRepository(t)
	tenantID, project := uuid.NewString(), uuid.NewString()
	want := map[string]bool{}
	for i := 0; i < 30; i++ {
		want[mustCreate(t, repo, tenantID, seed{typ: "task", project: project}).ID] = true
	}
	ctx := context.Background()
	seen := map[string]bool{}
	token := ""
	for page := 0; page < 10; page++ {
		got, next, err := repo.List(ctx, tenantID, usecase.ListFilter{ProjectID: project, PageSize: 7, PageToken: token})
		if err != nil {
			t.Fatal(err)
		}
		for _, g := range got {
			if seen[g.ID] {
				t.Fatalf("duplicate %s across pages", g.ID)
			}
			seen[g.ID] = true
		}
		if next == "" {
			break
		}
		token = next
	}
	if len(seen) != 30 {
		t.Errorf("pagination lost rows: %d of 30", len(seen))
	}
	for id := range want {
		if !seen[id] {
			t.Errorf("missing %s", id)
		}
	}
}

func TestRepository_List_TenantIsolation(t *testing.T) {
	repo := setupRepository(t)
	a, b, req := uuid.NewString(), uuid.NewString(), uuid.NewString()
	mustCreate(t, repo, a, seed{typ: "plan", project: uuid.NewString(), request: req})
	got, _, err := repo.List(context.Background(), b, usecase.ListFilter{RequestIDs: []string{req}})
	if err != nil || len(got) != 0 {
		t.Fatalf("tenant b must not see tenant a rows: %d %v", len(got), err)
	}
}

func TestRepository_UpdateContainerStatus_CAS_8Goroutines(t *testing.T) {
	repo := setupRepository(t)
	tenantID := uuid.NewString()
	plan := mustCreate(t, repo, tenantID, seed{typ: "plan", project: uuid.NewString()})
	var wins int32
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			ev := domain.OutboxEvent{ID: uuid.NewString(), Subject: "orca.task.task.statuschanged", OccurredAt: time.Now().UTC(), PayloadJSON: []byte(`{"n":` + fmt.Sprint(i) + `}`)}
			ok, err := repo.UpdateContainerStatus(context.Background(), tenantID, plan.ID, domain.StatusOpen, domain.StatusInProgress, []domain.OutboxEvent{ev})
			if err != nil {
				t.Errorf("cas: %v", err)
			}
			if ok {
				atomic.AddInt32(&wins, 1)
			}
		}(i)
	}
	wg.Wait()
	if wins != 1 {
		t.Fatalf("exactly one writer must win, got %d", wins)
	}
	var events int
	_ = repo.pool.QueryRow(context.Background(), `SELECT count(*) FROM task.outbox_events WHERE tenant_id=$1`, tenantID).Scan(&events)
	if events != 1 {
		t.Errorf("exactly one outbox row for the single winner, got %d", events)
	}
}

func TestRepository_UpdateContainerStatus_WritesOutboxSameTx(t *testing.T) {
	repo := setupRepository(t)
	ctx := context.Background()
	tenantID := uuid.NewString()
	plan := mustCreate(t, repo, tenantID, seed{typ: "plan", project: uuid.NewString()})
	dup := uuid.NewString()
	ev := func() domain.OutboxEvent {
		return domain.OutboxEvent{ID: dup, Subject: "orca.task.task.statuschanged", OccurredAt: time.Now().UTC(), PayloadJSON: []byte(`{}`)}
	}
	if err := repo.WriteOutboxEvent(ctx, tenantID, ev()); err != nil {
		t.Fatal(err)
	}
	// Reusing the id makes the outbox insert fail: the status change must roll back with it.
	ok, err := repo.UpdateContainerStatus(ctx, tenantID, plan.ID, domain.StatusOpen, domain.StatusInProgress, []domain.OutboxEvent{ev()})
	if err == nil || ok {
		t.Fatalf("expected the outbox failure to surface, ok=%v err=%v", ok, err)
	}
	got, _ := repo.Get(ctx, tenantID, plan.ID)
	if got.Status != domain.StatusOpen {
		t.Errorf("status must be unchanged after the failed outbox insert, got %s", got.Status)
	}
}

func TestRepository_UpdateContainerStatus_RejectsNonContainerRow(t *testing.T) {
	repo := setupRepository(t)
	ctx := context.Background()
	tenantID := uuid.NewString()
	work := mustCreate(t, repo, tenantID, seed{typ: "task"})
	ok, err := repo.UpdateContainerStatus(ctx, tenantID, work.ID, domain.StatusOpen, domain.StatusDone, nil)
	if err != nil || ok {
		t.Fatalf("a work task must never be written by the container path: ok=%v err=%v", ok, err)
	}
	if got, _ := repo.Get(ctx, tenantID, work.ID); got.Status != domain.StatusOpen {
		t.Errorf("status changed: %s", got.Status)
	}
}

func TestRepository_ListChildStatuses(t *testing.T) {
	repo := setupRepository(t)
	tenantID, project := uuid.NewString(), uuid.NewString()
	plan := mustCreate(t, repo, tenantID, seed{typ: "plan", project: project})
	mustCreate(t, repo, tenantID, seed{typ: "phase", parent: plan.ID, project: project, status: domain.StatusDone})
	mustCreate(t, repo, tenantID, seed{typ: "phase", parent: plan.ID, project: project})
	got, err := repo.ListChildStatuses(context.Background(), tenantID, plan.ID)
	if err != nil || len(got) != 2 {
		t.Fatalf("got %v %v", got, err)
	}
}

func TestExecutionLeases_ReleaseUnlinked_SkipsContainers(t *testing.T) {
	repo := setupRepository(t)
	ctx := context.Background()
	tenantID, project := uuid.NewString(), uuid.NewString()
	plan := mustCreate(t, repo, tenantID, seed{typ: "plan", project: project, status: domain.StatusInProgress})
	work := mustCreate(t, repo, tenantID, seed{typ: "task", project: project, status: domain.StatusInProgress})
	backdateTask(t, repo, plan.ID, time.Hour)
	backdateTask(t, repo, work.ID, time.Hour)

	n, err := repo.ReleaseUnlinkedInProgress(ctx, 30*time.Minute, 10)
	if err != nil || n != 1 {
		t.Fatalf("only the work task is released: n=%d err=%v", n, err)
	}
	if got, _ := repo.Get(ctx, tenantID, plan.ID); got.Status != domain.StatusInProgress {
		t.Errorf("container in_progress is derived and must survive the sweep, got %s", got.Status)
	}
	if got, _ := repo.Get(ctx, tenantID, work.ID); got.Status != domain.StatusOpen {
		t.Errorf("work task should be released, got %s", got.Status)
	}
}

func TestHasActiveExecutions_IgnoresContainers(t *testing.T) {
	repo := setupRepository(t)
	ctx := context.Background()
	tenantID, project := uuid.NewString(), uuid.NewString()
	plan := mustCreate(t, repo, tenantID, seed{typ: "plan", project: project, status: domain.StatusInProgress})
	mustCreate(t, repo, tenantID, seed{typ: "phase", parent: plan.ID, project: project, status: domain.StatusInProgress})
	if has, err := repo.HasActiveExecutions(ctx, tenantID, project); err != nil || has {
		t.Fatalf("containers are not runs: has=%v err=%v", has, err)
	}
	mustCreate(t, repo, tenantID, seed{typ: "task", project: project, status: domain.StatusInProgress})
	if has, err := repo.HasActiveExecutions(ctx, tenantID, project); err != nil || !has {
		t.Fatalf("a work task in progress counts: has=%v err=%v", has, err)
	}
}

func TestRecentCompletedTasks_IgnoresContainers(t *testing.T) {
	repo := setupRepository(t)
	ctx := context.Background()
	tenantID, project := uuid.NewString(), uuid.NewString()
	mustCreate(t, repo, tenantID, seed{typ: "plan", project: project, status: domain.StatusDone})
	work := mustCreate(t, repo, tenantID, seed{typ: "task", project: project, status: domain.StatusDone})
	got, err := repo.RecentCompletedTasks(ctx, tenantID, project, 10)
	if err != nil || len(got) != 1 || got[0].ID != work.ID {
		t.Fatalf("got %+v %v", got, err)
	}
}

func outboxCount(t *testing.T, repo *Repository, tenantID, taskID string) int {
	t.Helper()
	var n int
	err := repo.pool.QueryRow(context.Background(), `
		SELECT count(*) FROM task.outbox_events
		WHERE tenant_id = $1 AND subject = 'orca.task.task.statuschanged' AND payload->>'task_id' = $2
	`, tenantID, taskID).Scan(&n)
	if err != nil {
		t.Fatal(err)
	}
	return n
}

// TestContainerLifecycle_LeafDrivesPhaseAndPlan walks a leaf open -> in_progress -> review -> done through
// the real repository and checks phase and plan follow, one derived outbox row per change.
func TestContainerLifecycle_LeafDrivesPhaseAndPlan(t *testing.T) {
	repo := setupRepository(t)
	tenantID, project := uuid.NewString(), uuid.NewString()
	ctx := tenantCtx(context.Background(), tenantID)
	plan := mustCreate(t, repo, tenantID, seed{typ: "plan", project: project, request: uuid.NewString()})
	phase := mustCreate(t, repo, tenantID, seed{typ: "phase", parent: plan.ID, project: project})
	leaf := mustCreate(t, repo, tenantID, seed{typ: "task", parent: phase.ID, project: project})
	sync := usecase.NewSyncContainerStatus(repo, usecase.NewRecalculateProgress(repo))

	step := func(leafStatus domain.Status, wantContainer domain.Status) {
		t.Helper()
		if leafStatus == domain.StatusReview || leafStatus == domain.StatusDone {
			if err := repo.CompleteExecution(ctx, tenantID, leaf.ID, string(leafStatus), 0, nil); err != nil {
				t.Fatal(err)
			}
		} else if err := repo.UpdateStatus(ctx, tenantID, leaf.ID, leafStatus); err != nil {
			t.Fatal(err)
		}
		if err := sync.Execute(ctx, leaf.ID); err != nil {
			t.Fatal(err)
		}
		for _, id := range []string{phase.ID, plan.ID} {
			got, _ := repo.Get(ctx, tenantID, id)
			if got.Status != wantContainer {
				t.Fatalf("leaf=%s: container %s = %s, want %s", leafStatus, id, got.Status, wantContainer)
			}
		}
	}
	step(domain.StatusInProgress, domain.StatusInProgress)
	step(domain.StatusReview, domain.StatusReview)
	step(domain.StatusDone, domain.StatusDone)

	for _, id := range []string{phase.ID, plan.ID} {
		if n := outboxCount(t, repo, tenantID, id); n != 3 {
			t.Errorf("container %s: want exactly one statuschanged row per change (3), got %d", id, n)
		}
	}
	var derived int
	_ = repo.pool.QueryRow(ctx, `SELECT count(*) FROM task.outbox_events WHERE tenant_id=$1 AND payload->>'cause'='derived' AND payload->>'task_type' IN ('plan','phase')`, tenantID).Scan(&derived)
	if derived != 6 {
		t.Errorf("all container events carry cause=derived and task_type: got %d of 6", derived)
	}
	if got, _ := repo.Get(ctx, tenantID, plan.ID); got.ProgressPercent != 100 {
		t.Errorf("plan progress after done: %d", got.ProgressPercent)
	}
}

// TestReconcile_ReleaseUnlinked_FixesPhase: the sweep resets a leaf without telling anyone; reconcile repairs the phase.
func TestReconcile_ReleaseUnlinked_FixesPhase(t *testing.T) {
	repo := setupRepository(t)
	tenantID, project := uuid.NewString(), uuid.NewString()
	ctx := tenantCtx(context.Background(), tenantID)
	plan := mustCreate(t, repo, tenantID, seed{typ: "plan", project: project})
	phase := mustCreate(t, repo, tenantID, seed{typ: "phase", parent: plan.ID, project: project})
	leaf := mustCreate(t, repo, tenantID, seed{typ: "task", parent: phase.ID, project: project})
	sync := usecase.NewSyncContainerStatus(repo, nil)

	if err := repo.UpdateStatus(ctx, tenantID, leaf.ID, domain.StatusInProgress); err != nil {
		t.Fatal(err)
	}
	if err := sync.Execute(ctx, leaf.ID); err != nil {
		t.Fatal(err)
	}
	if got, _ := repo.Get(ctx, tenantID, phase.ID); got.Status != domain.StatusInProgress {
		t.Fatalf("phase should follow the leaf: %s", got.Status)
	}
	// Leaf has no execution link and has been in_progress for an hour: the sweep resets it.
	backdateTask(t, repo, leaf.ID, time.Hour)
	if n, err := repo.ReleaseUnlinkedInProgress(ctx, 30*time.Minute, 10); err != nil || n != 1 {
		t.Fatalf("sweep: n=%d err=%v", n, err)
	}
	// The phase is not yet repaired; the reconcile step must do it within one pass.
	reconcile := usecase.NewReconcileContainerStatuses(repo, sync)
	fixed, err := reconcile.Execute(ctx)
	if err != nil || fixed != 1 {
		t.Fatalf("reconcile fixed=%d err=%v", fixed, err)
	}
	for _, id := range []string{phase.ID, plan.ID} {
		if got, _ := repo.Get(ctx, tenantID, id); got.Status != domain.StatusOpen {
			t.Errorf("container %s = %s, want open", id, got.Status)
		}
	}
	// A second pass finds nothing stale: the repaired containers are newer than their children.
	if again, err := reconcile.Execute(ctx); err != nil || again != 0 {
		t.Errorf("second reconcile: %d %v", again, err)
	}
}

func TestReconcile_TouchesUnchangedContainer(t *testing.T) {
	repo := setupRepository(t)
	tenantID, project := uuid.NewString(), uuid.NewString()
	ctx := tenantCtx(context.Background(), tenantID)
	plan := mustCreate(t, repo, tenantID, seed{typ: "plan", project: project})
	leaf := mustCreate(t, repo, tenantID, seed{typ: "phase", parent: plan.ID, project: project})
	// Child touched after the container, but the derived status is unchanged (open).
	backdateTask(t, repo, plan.ID, time.Hour)
	if err := repo.UpdateStatus(ctx, tenantID, leaf.ID, domain.StatusOpen); err != nil {
		t.Fatal(err)
	}
	stale, err := repo.ListStaleContainers(ctx, 10)
	if err != nil || len(stale) != 1 || stale[0].ID != plan.ID {
		t.Fatalf("stale: %+v %v", stale, err)
	}
	reconcile := usecase.NewReconcileContainerStatuses(repo, usecase.NewSyncContainerStatus(repo, nil))
	if n, err := reconcile.Execute(ctx); err != nil || n != 0 {
		t.Fatalf("n=%d err=%v", n, err)
	}
	if stale, _ := repo.ListStaleContainers(ctx, 10); len(stale) != 0 {
		t.Errorf("a checked container must be touched out of the stale set: %+v", stale)
	}
}

func tenantCtx(ctx context.Context, tenantID string) context.Context {
	return tenant.WithTenantID(ctx, tenantID)
}
