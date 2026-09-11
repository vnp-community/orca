//go:build integration

// Integration tests run against a real MySQL via testcontainers-go, per
// specs/backend-go/standards/testing-strategy.md — gated behind the
// "integration" build tag so `go test ./...` (unit tests only) stays fast
// and Docker-free; run these explicitly with
// `go test -tags=integration ./internal/adapter/mysql/...`. Mirrors
// internal/adapter/postgres/repository_test.go's test names/shape 1:1 (no
// RLS-policy tables here to add a dedicated tenant-isolation test for
// beyond what these tests already exercise via explicit tenant_id
// scoping — see BE-DB-SOL-010 §6 for why no extra test was added).
package mysql

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	_ "github.com/go-sql-driver/mysql"

	"github.com/stablyai/orca-go/common/testutil"
	"github.com/stablyai/orca-go/services/orchestration-service/internal/domain"
	"github.com/stablyai/orca-go/services/orchestration-service/internal/usecase"
)

func setupRepository(t *testing.T) *Repository {
	t.Helper()
	// testutil.StartMySQL returns "mysql://root:orca@tcp(host:port)/db" —
	// valid as-is for golang-migrate's mysql driver CLI (used below), but
	// go-sql-driver/mysql's database/sql driver expects its OWN DSN format
	// with no "mysql://" scheme — same fixed-shape conversion
	// usage-service's own repository_test.go uses.
	rawDSN := testutil.StartMySQL(t, "orchestration")
	driverDSN := strings.TrimPrefix(rawDSN, "mysql://") + "?parseTime=true"

	migrationsPath, err := filepath.Abs("../../../migrations/mysql")
	if err != nil {
		t.Fatalf("resolving migrations path: %v", err)
	}
	cmd := exec.Command("migrate", "-path", migrationsPath, "-database", rawDSN, "up")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("running migrations: %v\n%s", err, out)
	}

	db, err := sql.Open("mysql", driverDSN)
	if err != nil {
		t.Fatalf("connecting to mysql: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := db.PingContext(context.Background()); err != nil {
		t.Fatalf("pinging mysql: %v", err)
	}

	return New(db)
}

func seedCoordinatorRun(t *testing.T, db *sql.DB, tenantID, id, handle string) {
	t.Helper()
	_, err := db.Exec(`
		INSERT INTO coordinator_runs (id, tenant_id, origin_task_id, coordinator_handle)
		VALUES (?, ?, 'origin-task-1', ?)
	`, id, tenantID, handle)
	if err != nil {
		t.Fatalf("seeding coordinator run: %v", err)
	}
}

func TestRepository_UpdateStatusAndPromote_PromotesReadySiblingsAtomically(t *testing.T) {
	repo := setupRepository(t)
	ctx := context.Background()
	tenantID := "11111111-1111-1111-1111-111111111111"
	runID := "22222222-2222-2222-2222-222222222222"

	seedCoordinatorRun(t, repo.db, tenantID, runID, "coord-1")

	root, err := domain.NewOrchestrationTask("", tenantID, runID, "", "", "root", nil, nil)
	if err != nil {
		t.Fatalf("building root: %v", err)
	}
	root, err = repo.Create(ctx, root)
	if err != nil {
		t.Fatalf("creating root task: %v", err)
	}

	dependent, err := domain.NewOrchestrationTask("", tenantID, runID, "", "", "dependent", nil, []string{root.ID})
	if err != nil {
		t.Fatalf("building dependent: %v", err)
	}
	dependent, err = repo.Create(ctx, dependent)
	if err != nil {
		t.Fatalf("creating dependent task: %v", err)
	}

	result, err := repo.UpdateStatusAndPromote(ctx, tenantID, root.ID, domain.TaskStatusCompleted, domain.OutboxEvent{})
	if err != nil {
		t.Fatalf("UpdateStatusAndPromote: %v", err)
	}
	if result.Task.Status != domain.TaskStatusCompleted {
		t.Errorf("expected root completed, got %s", result.Task.Status)
	}
	if len(result.PromotedIDs) != 1 || result.PromotedIDs[0] != dependent.ID {
		t.Fatalf("expected dependent promoted, got %v", result.PromotedIDs)
	}
	if result.RunFinalized != nil {
		t.Errorf("expected run NOT finalized while a sibling remains non-terminal, got %+v", result.RunFinalized)
	}

	got, err := repo.Get(ctx, tenantID, dependent.ID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.Status != domain.TaskStatusReady {
		t.Errorf("expected dependent status ready, got %s", got.Status)
	}
}

func TestRepository_ResolveGate_CannotBeResolvedTwice(t *testing.T) {
	repo := setupRepository(t)
	ctx := context.Background()
	tenantID := "33333333-3333-3333-3333-333333333333"
	runID := "44444444-4444-4444-4444-444444444444"

	seedCoordinatorRun(t, repo.db, tenantID, runID, "coord-2")

	task, err := domain.NewOrchestrationTask("", tenantID, runID, "", "", "gated task", nil, nil)
	if err != nil {
		t.Fatalf("building task: %v", err)
	}
	task, err = repo.Create(ctx, task)
	if err != nil {
		t.Fatalf("creating task: %v", err)
	}

	dc, err := repo.CreateDispatchContext(ctx, tenantID, "user-1", "", "handle-1", runID, task.ID, domain.OutboxEvent{})
	if err != nil {
		t.Fatalf("creating dispatch context: %v", err)
	}

	gate, err := repo.CreateGate(ctx, tenantID, dc.ID, "proceed?", []string{"yes", "no"}, domain.OutboxEvent{})
	if err != nil {
		t.Fatalf("CreateGate: %v", err)
	}

	if _, _, err := repo.ResolveGate(ctx, tenantID, gate.ID, "yes"); err != nil {
		t.Fatalf("first ResolveGate: %v", err)
	}

	if _, _, err := repo.ResolveGate(ctx, tenantID, gate.ID, "no"); err == nil {
		t.Fatal("expected an error resolving an already-resolved gate")
	}
}

// TestRepository_CreateGate_SucceedsWhenDispatchContextHasTask mirrors the
// postgres adapter's identically-named test — see that file's doc comment
// for the Epic C background this proves for the MySQL adapter too.
func TestRepository_CreateGate_SucceedsWhenDispatchContextHasTask(t *testing.T) {
	repo := setupRepository(t)
	ctx := context.Background()
	tenantID := "55555555-5555-5555-5555-555555555555"
	runID := "66666666-6666-6666-6666-666666666666"

	seedCoordinatorRun(t, repo.db, tenantID, runID, "coord-3")

	task, err := domain.NewOrchestrationTask("", tenantID, runID, "", "", "gated task", nil, nil)
	if err != nil {
		t.Fatalf("building task: %v", err)
	}
	task, err = repo.Create(ctx, task)
	if err != nil {
		t.Fatalf("creating task: %v", err)
	}

	dc, err := repo.CreateDispatchContext(ctx, tenantID, "user-1", "", "handle-1", runID, task.ID, domain.OutboxEvent{})
	if err != nil {
		t.Fatalf("creating dispatch context: %v", err)
	}
	if dc.OrchestrationTaskID != task.ID {
		t.Fatalf("expected dispatch context orchestration_task_id %q, got %q", task.ID, dc.OrchestrationTaskID)
	}

	gate, err := repo.CreateGate(ctx, tenantID, dc.ID, "proceed?", []string{"yes", "no"}, domain.OutboxEvent{})
	if err != nil {
		t.Fatalf("expected CreateGate to succeed for a dispatch context with a task, got: %v", err)
	}
	if gate.OrchestrationTaskID != task.ID {
		t.Errorf("expected gate.OrchestrationTaskID %q, got %q", task.ID, gate.OrchestrationTaskID)
	}
	if gate.Question != "proceed?" {
		t.Errorf("expected question to round-trip, got %q", gate.Question)
	}
	if len(gate.Options) != 2 || gate.Options[0] != "yes" || gate.Options[1] != "no" {
		t.Errorf("expected options to round-trip, got %v", gate.Options)
	}

	got, err := repo.Get(ctx, tenantID, task.ID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.Status != domain.TaskStatusBlocked {
		t.Errorf("expected task blocked by the gate, got %s", got.Status)
	}
}

func TestRepository_CreateGate_FailsWhenDispatchContextHasNoTask(t *testing.T) {
	repo := setupRepository(t)
	ctx := context.Background()
	tenantID := "77777777-7777-7777-7777-777777777777"
	runID := "88888888-8888-8888-8888-888888888888"

	seedCoordinatorRun(t, repo.db, tenantID, runID, "coord-4")

	dc, err := repo.CreateDispatchContext(ctx, tenantID, "user-1", "", "handle-1", runID, "", domain.OutboxEvent{})
	if err != nil {
		t.Fatalf("creating dispatch context: %v", err)
	}

	if _, err := repo.CreateGate(ctx, tenantID, dc.ID, "proceed?", []string{"yes", "no"}, domain.OutboxEvent{}); !errors.Is(err, usecase.ErrDispatchContextHasNoTask) {
		t.Fatalf("expected usecase.ErrDispatchContextHasNoTask, got: %v", err)
	}
}

func TestRepository_GetLatestForTask_ReturnsMostRecentAfterRetry(t *testing.T) {
	repo := setupRepository(t)
	ctx := context.Background()
	tenantID := "99999999-9999-9999-9999-999999999999"
	runID := "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"

	seedCoordinatorRun(t, repo.db, tenantID, runID, "coord-5")

	task, err := domain.NewOrchestrationTask("", tenantID, runID, "", "", "retried task", nil, nil)
	if err != nil {
		t.Fatalf("building task: %v", err)
	}
	task, err = repo.Create(ctx, task)
	if err != nil {
		t.Fatalf("creating task: %v", err)
	}

	first, err := repo.CreateDispatchContext(ctx, tenantID, "user-1", "", "handle-a", runID, task.ID, domain.OutboxEvent{})
	if err != nil {
		t.Fatalf("create first dispatch context: %v", err)
	}
	_ = first
	time.Sleep(10 * time.Millisecond) // ensure created_at strictly orders the second row after the first
	second, err := repo.CreateDispatchContext(ctx, tenantID, "user-1", "", "handle-b", runID, task.ID, domain.OutboxEvent{})
	if err != nil {
		t.Fatalf("create second (retry) dispatch context: %v", err)
	}

	got, err := repo.GetLatestForTask(ctx, tenantID, task.ID)
	if err != nil {
		t.Fatalf("get latest for task: %v", err)
	}
	if got.ID != second.ID {
		t.Errorf("want the later dispatch context (id=%s), got id=%s", second.ID, got.ID)
	}
}

func TestRepository_GetLatestForTask_NoRows_ReturnsErrDispatchContextNotFound(t *testing.T) {
	repo := setupRepository(t)
	ctx := context.Background()

	_, err := repo.GetLatestForTask(ctx, "bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb", "cccccccc-cccc-cccc-cccc-cccccccccccc")
	if !errors.Is(err, usecase.ErrDispatchContextNotFound) {
		t.Fatalf("want ErrDispatchContextNotFound, got %v", err)
	}
}

// TestRepository_ListActiveDispatchContextsForUser_ReturnsOnlyCallerNonTerminal
// mirrors the postgres adapter's identically-named test, including the
// cross-tenant leak check — this table carries no RLS policy on either
// dialect's schema variant beyond explicit tenant_id scoping in every
// query, so this test is the only tenant-isolation proof either adapter
// has for dispatch_contexts, on both dialects equally.
func TestRepository_ListActiveDispatchContextsForUser_ReturnsOnlyCallerNonTerminal(t *testing.T) {
	repo := setupRepository(t)
	ctx := context.Background()
	tenantID := "11111111-1111-1111-1111-111111111111"
	otherTenantID := "22222222-2222-2222-2222-222222222222"
	runID := "33333333-3333-3333-3333-333333333333"
	otherTenantRunID := "44444444-4444-4444-4444-444444444444"

	seedCoordinatorRun(t, repo.db, tenantID, runID, "coord-list-1")
	seedCoordinatorRun(t, repo.db, otherTenantID, otherTenantRunID, "coord-list-2")

	// Active, this user, this tenant — should be returned.
	active, err := repo.CreateDispatchContext(ctx, tenantID, "user-a", "", "handle-active", runID, "", domain.OutboxEvent{})
	if err != nil {
		t.Fatalf("create active dispatch context: %v", err)
	}

	// Completed, this user, this tenant — terminal, should be excluded.
	completed, err := repo.CreateDispatchContext(ctx, tenantID, "user-a", "", "handle-completed", runID, "", domain.OutboxEvent{})
	if err != nil {
		t.Fatalf("create completed dispatch context: %v", err)
	}
	if _, err := repo.db.Exec(`UPDATE dispatch_contexts SET status = 'completed' WHERE id = ?`, completed.ID); err != nil {
		t.Fatalf("marking dispatch context completed: %v", err)
	}

	// Active, DIFFERENT user, same tenant — should be excluded.
	if _, err := repo.CreateDispatchContext(ctx, tenantID, "user-b", "", "handle-other-user", runID, "", domain.OutboxEvent{}); err != nil {
		t.Fatalf("create other-user dispatch context: %v", err)
	}

	// Active, same user id string, DIFFERENT tenant — should be excluded
	// (tenant isolation must not leak across the user_id filter alone).
	if _, err := repo.CreateDispatchContext(ctx, otherTenantID, "user-a", "", "handle-other-tenant", otherTenantRunID, "", domain.OutboxEvent{}); err != nil {
		t.Fatalf("create other-tenant dispatch context: %v", err)
	}

	got, err := repo.ListActiveDispatchContextsForUser(ctx, tenantID, "user-a")
	if err != nil {
		t.Fatalf("list active dispatch contexts for user: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("want exactly 1 active dispatch context, got %d: %+v", len(got), got)
	}
	if got[0].ID != active.ID {
		t.Errorf("want the active dispatch context (id=%s), got id=%s", active.ID, got[0].ID)
	}
	if got[0].UserID != "user-a" {
		t.Errorf("want UserID=user-a, got %q", got[0].UserID)
	}
}

func TestRepository_CreateWithTasks_ResolvesTempIDDepsToRealIDs(t *testing.T) {
	repo := setupRepository(t)
	ctx := context.Background()
	tenantID := "d0000000-0000-0000-0000-000000000001"

	specJSON := []byte(`[
		{"tempId":"a","title":"Root","spec":{},"deps":[]},
		{"tempId":"b","title":"Child","spec":{},"deps":["a"]}
	]`)
	run, err := domain.NewCoordinatorRun("", tenantID, "origin-task-1", "coord-create-with-tasks", specJSON, 0)
	if err != nil {
		t.Fatalf("building run: %v", err)
	}
	run.Status = domain.RunStatusRunning

	created, err := repo.CreateWithTasks(ctx, tenantID, run)
	if err != nil {
		t.Fatalf("CreateWithTasks: %v", err)
	}
	if created.ID == "" {
		t.Fatal("expected a minted run id")
	}

	rows, err := repo.db.Query(`
		SELECT id, task_title, status, deps FROM orchestration_tasks
		WHERE coordinator_run_id = ? ORDER BY task_title
	`, created.ID)
	if err != nil {
		t.Fatalf("querying materialized tasks: %v", err)
	}
	defer rows.Close()

	type row struct {
		id, title, status string
		deps              []string
	}
	var got []row
	for rows.Next() {
		var r row
		var depsJSON []byte
		if err := rows.Scan(&r.id, &r.title, &r.status, &depsJSON); err != nil {
			t.Fatalf("scanning task row: %v", err)
		}
		if len(depsJSON) > 0 {
			if err := json.Unmarshal(depsJSON, &r.deps); err != nil {
				t.Fatalf("unmarshal deps: %v", err)
			}
		}
		got = append(got, r)
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("iterating task rows: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("expected 2 materialized tasks, got %d", len(got))
	}
	// got[0] = Child (alphabetically first), got[1] = Root
	child, root := got[0], got[1]
	if child.title != "Child" || root.title != "Root" {
		t.Fatalf("unexpected task ordering: %+v", got)
	}
	if root.status != string(domain.TaskStatusReady) {
		t.Errorf("expected root (no deps) status ready, got %s", root.status)
	}
	if child.status != string(domain.TaskStatusPending) {
		t.Errorf("expected child (has dep) status pending, got %s", child.status)
	}
	if len(child.deps) != 1 || child.deps[0] != root.id {
		t.Errorf("expected child.deps to resolve to root's real id %q, got %v", root.id, child.deps)
	}
}

func TestRepository_ClaimReady_ExactlyOneWinsUnderConcurrency(t *testing.T) {
	repo := setupRepository(t)
	ctx := context.Background()
	tenantID := "d0000000-0000-0000-0000-000000000002"
	runID := "d0000000-0000-0000-0000-000000000003"

	seedCoordinatorRun(t, repo.db, tenantID, runID, "coord-claim")

	task, err := domain.NewOrchestrationTask("", tenantID, runID, "", "", "claimable", nil, nil)
	if err != nil {
		t.Fatalf("building task: %v", err)
	}
	task, err = repo.Create(ctx, task)
	if err != nil {
		t.Fatalf("creating task: %v", err)
	}
	if _, err := repo.db.Exec(`UPDATE orchestration_tasks SET status = 'ready' WHERE id = ?`, task.ID); err != nil {
		t.Fatalf("marking task ready: %v", err)
	}

	const attempts = 5
	results := make(chan bool, attempts)
	var wg sync.WaitGroup
	for i := 0; i < attempts; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, ok, err := repo.ClaimReady(ctx, tenantID, task.ID)
			if err != nil {
				t.Errorf("ClaimReady: %v", err)
				return
			}
			results <- ok
		}()
	}
	wg.Wait()
	close(results)

	successes := 0
	for ok := range results {
		if ok {
			successes++
		}
	}
	if successes != 1 {
		t.Fatalf("expected exactly 1 successful claim, got %d", successes)
	}
}

func TestRepository_ListPending_ReturnsOnlyPendingRows(t *testing.T) {
	repo := setupRepository(t)
	ctx := context.Background()
	tenantID := "d0000000-0000-0000-0000-000000000004"
	runID := "d0000000-0000-0000-0000-000000000005"

	seedCoordinatorRun(t, repo.db, tenantID, runID, "coord-pending")

	pendingTask, err := domain.NewOrchestrationTask("", tenantID, runID, "", "", "pending gate task", nil, nil)
	if err != nil {
		t.Fatalf("building task: %v", err)
	}
	pendingTask, err = repo.Create(ctx, pendingTask)
	if err != nil {
		t.Fatalf("creating task: %v", err)
	}
	dc1, err := repo.CreateDispatchContext(ctx, tenantID, "user-1", "", "handle-pending", runID, pendingTask.ID, domain.OutboxEvent{})
	if err != nil {
		t.Fatalf("creating dispatch context: %v", err)
	}
	pendingGate, err := repo.CreateGate(ctx, tenantID, dc1.ID, "proceed?", []string{"yes", "no"}, domain.OutboxEvent{})
	if err != nil {
		t.Fatalf("CreateGate: %v", err)
	}

	resolvedTask, err := domain.NewOrchestrationTask("", tenantID, runID, "", "", "resolved gate task", nil, nil)
	if err != nil {
		t.Fatalf("building task: %v", err)
	}
	resolvedTask, err = repo.Create(ctx, resolvedTask)
	if err != nil {
		t.Fatalf("creating task: %v", err)
	}
	dc2, err := repo.CreateDispatchContext(ctx, tenantID, "user-1", "", "handle-resolved", runID, resolvedTask.ID, domain.OutboxEvent{})
	if err != nil {
		t.Fatalf("creating dispatch context: %v", err)
	}
	resolvedGate, err := repo.CreateGate(ctx, tenantID, dc2.ID, "proceed?", []string{"yes", "no"}, domain.OutboxEvent{})
	if err != nil {
		t.Fatalf("CreateGate: %v", err)
	}
	if _, _, err := repo.ResolveGate(ctx, tenantID, resolvedGate.ID, "yes"); err != nil {
		t.Fatalf("ResolveGate: %v", err)
	}

	got, err := repo.ListPending(ctx, tenantID)
	if err != nil {
		t.Fatalf("ListPending: %v", err)
	}
	if len(got) != 1 || got[0].ID != pendingGate.ID {
		t.Fatalf("expected exactly the pending gate %q, got %+v", pendingGate.ID, got)
	}
}

func TestRepository_CountNonTerminalByRun(t *testing.T) {
	repo := setupRepository(t)
	ctx := context.Background()
	tenantID := "d0000000-0000-0000-0000-000000000006"
	runID := "d0000000-0000-0000-0000-000000000007"

	seedCoordinatorRun(t, repo.db, tenantID, runID, "coord-count")

	statuses := []domain.TaskStatus{
		domain.TaskStatusPending, domain.TaskStatusReady, domain.TaskStatusDispatched,
		domain.TaskStatusBlocked, domain.TaskStatusCompleted, domain.TaskStatusFailed,
	}
	for i, s := range statuses {
		task, err := domain.NewOrchestrationTask("", tenantID, runID, "", "", fmt.Sprintf("task-%d", i), nil, nil)
		if err != nil {
			t.Fatalf("building task: %v", err)
		}
		task, err = repo.Create(ctx, task)
		if err != nil {
			t.Fatalf("creating task: %v", err)
		}
		if _, err := repo.db.Exec(`UPDATE orchestration_tasks SET status = ? WHERE id = ?`, string(s), task.ID); err != nil {
			t.Fatalf("setting status: %v", err)
		}
	}

	count, err := repo.CountNonTerminalByRun(ctx, tenantID, runID)
	if err != nil {
		t.Fatalf("CountNonTerminalByRun: %v", err)
	}
	if count != 4 { // pending, ready, dispatched, blocked — completed/failed excluded
		t.Errorf("expected 4 non-terminal tasks, got %d", count)
	}
}

func TestRepository_RecordHeartbeat(t *testing.T) {
	repo := setupRepository(t)
	ctx := context.Background()
	tenantID := "d0000000-0000-0000-0000-000000000008"
	runID := "d0000000-0000-0000-0000-000000000009"

	seedCoordinatorRun(t, repo.db, tenantID, runID, "coord-heartbeat")

	dc, err := repo.CreateDispatchContext(ctx, tenantID, "user-1", "", "handle-hb", runID, "", domain.OutboxEvent{})
	if err != nil {
		t.Fatalf("creating dispatch context: %v", err)
	}

	updated, err := repo.RecordHeartbeat(ctx, tenantID, dc.ID)
	if err != nil {
		t.Fatalf("RecordHeartbeat: %v", err)
	}
	if updated.LastHeartbeatAt.IsZero() {
		t.Error("expected LastHeartbeatAt to be set")
	}

	if _, err := repo.RecordHeartbeat(ctx, tenantID, "does-not-exist"); !errors.Is(err, usecase.ErrDispatchContextNotFound) {
		t.Fatalf("expected ErrDispatchContextNotFound, got %v", err)
	}
}

func TestRepository_ListUnreportedTerminal_AndMarkReported(t *testing.T) {
	repo := setupRepository(t)
	ctx := context.Background()
	tenantID := "d0000000-0000-0000-0000-00000000000a"
	runID := "d0000000-0000-0000-0000-00000000000b"

	seedCoordinatorRun(t, repo.db, tenantID, runID, "coord-unreported")
	if _, err := repo.db.Exec(`UPDATE coordinator_runs SET status = 'completed' WHERE id = ?`, runID); err != nil {
		t.Fatalf("marking run completed: %v", err)
	}

	unreported, err := repo.ListUnreportedTerminal(ctx)
	if err != nil {
		t.Fatalf("ListUnreportedTerminal: %v", err)
	}
	found := false
	for _, r := range unreported {
		if r.ID == runID {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected run %q in unreported terminal list, got %+v", runID, unreported)
	}

	if err := repo.MarkReported(ctx, tenantID, runID); err != nil {
		t.Fatalf("MarkReported: %v", err)
	}

	after, err := repo.ListUnreportedTerminal(ctx)
	if err != nil {
		t.Fatalf("ListUnreportedTerminal (after): %v", err)
	}
	for _, r := range after {
		if r.ID == runID {
			t.Fatalf("expected run %q to be removed from unreported list after MarkReported", runID)
		}
	}
}

// TestRepository_UpdateStatusAndPromote_FinalizesRunOnLastTask is the
// concrete MySQL-level proof mirroring the postgres adapter's identically-
// named test: completing the last non-terminal task in a run transitions
// coordinator_runs to completed in the SAME transaction and returns
// RunFinalized with the run's origin_task_id — proving the no-RETURNING
// origin_task_id-fetched-before-CAS-UPDATE translation (see
// UpdateStatusAndPromote's doc comment) preserves the Postgres adapter's
// behavior exactly.
func TestRepository_UpdateStatusAndPromote_FinalizesRunOnLastTask(t *testing.T) {
	repo := setupRepository(t)
	ctx := context.Background()
	tenantID := "d0000000-0000-0000-0000-00000000000c"
	runID := "d0000000-0000-0000-0000-00000000000d"

	seedCoordinatorRun(t, repo.db, tenantID, runID, "coord-finalize")
	if _, err := repo.db.Exec(`UPDATE coordinator_runs SET status = 'running' WHERE id = ?`, runID); err != nil {
		t.Fatalf("marking run running: %v", err)
	}

	task, err := domain.NewOrchestrationTask("", tenantID, runID, "", "", "only task", nil, nil)
	if err != nil {
		t.Fatalf("building task: %v", err)
	}
	task, err = repo.Create(ctx, task)
	if err != nil {
		t.Fatalf("creating task: %v", err)
	}

	result, err := repo.UpdateStatusAndPromote(ctx, tenantID, task.ID, domain.TaskStatusCompleted, domain.OutboxEvent{})
	if err != nil {
		t.Fatalf("UpdateStatusAndPromote: %v", err)
	}
	if result.RunFinalized == nil {
		t.Fatal("expected RunFinalized to be set when the last task completes")
	}
	if result.RunFinalized.CoordinatorRunID != runID {
		t.Errorf("expected CoordinatorRunID %q, got %q", runID, result.RunFinalized.CoordinatorRunID)
	}
	if result.RunFinalized.OriginTaskID != "origin-task-1" {
		t.Errorf("expected OriginTaskID origin-task-1 (seeded), got %q", result.RunFinalized.OriginTaskID)
	}
	if !result.RunFinalized.Success {
		t.Error("expected Success=true for an all-completed run")
	}

	var status string
	if err := repo.db.QueryRow(`SELECT status FROM coordinator_runs WHERE id = ?`, runID).Scan(&status); err != nil {
		t.Fatalf("querying run status: %v", err)
	}
	if status != "completed" {
		t.Errorf("expected coordinator_runs.status = completed, got %q", status)
	}
}

// TestRepository_UpdateStatusAndPromote_DoesNotDoubleFinalizeRacingRun
// proves the `WHERE ... AND status = 'running'` CAS guard translated
// without RETURNING (see UpdateStatusAndPromote's doc comment): a run
// already completed/failed by a racing concurrent call must not error the
// whole transaction, only skip setting RunFinalized.
func TestRepository_UpdateStatusAndPromote_DoesNotDoubleFinalizeRacingRun(t *testing.T) {
	repo := setupRepository(t)
	ctx := context.Background()
	tenantID := "d0000000-0000-0000-0000-00000000000e"
	runID := "d0000000-0000-0000-0000-00000000000f"

	seedCoordinatorRun(t, repo.db, tenantID, runID, "coord-race")
	// Simulate the run already finalized by a racing concurrent call.
	if _, err := repo.db.Exec(`UPDATE coordinator_runs SET status = 'completed', completed_at = ? WHERE id = ?`, time.Now().UTC(), runID); err != nil {
		t.Fatalf("marking run already completed: %v", err)
	}

	task, err := domain.NewOrchestrationTask("", tenantID, runID, "", "", "task", nil, nil)
	if err != nil {
		t.Fatalf("building task: %v", err)
	}
	task, err = repo.Create(ctx, task)
	if err != nil {
		t.Fatalf("creating task: %v", err)
	}

	result, err := repo.UpdateStatusAndPromote(ctx, tenantID, task.ID, domain.TaskStatusCompleted, domain.OutboxEvent{})
	if err != nil {
		t.Fatalf("expected no error for a racing already-finalized run, got: %v", err)
	}
	if result.RunFinalized != nil {
		t.Errorf("expected RunFinalized to stay nil for an already-finalized run, got %+v", result.RunFinalized)
	}
	if result.Task.Status != domain.TaskStatusCompleted {
		t.Errorf("expected the task-status write itself to still succeed, got %s", result.Task.Status)
	}
}

// TestRepository_Outbox_EnqueueFetchMarkPublished proves the
// common/outbox.Store implementation end to end — no equivalent test
// exists in the postgres adapter's own test file (that adapter's outbox
// methods are only exercised indirectly, via UpdateStatusAndPromote/
// CreateDispatchContext/CreateGate's own event.ID != "" branches above)
// so this is additive coverage specific to this rollout, following
// issue-tracking-service's TASK-BE-DB-009 precedent of adding a direct
// outbox round-trip test.
func TestRepository_Outbox_EnqueueFetchMarkPublished(t *testing.T) {
	repo := setupRepository(t)
	ctx := context.Background()
	tenantID := "d0000000-0000-0000-0000-000000000010"
	runID := "d0000000-0000-0000-0000-000000000011"

	seedCoordinatorRun(t, repo.db, tenantID, runID, "coord-outbox")

	task, err := domain.NewOrchestrationTask("", tenantID, runID, "", "", "task", nil, nil)
	if err != nil {
		t.Fatalf("building task: %v", err)
	}
	task, err = repo.Create(ctx, task)
	if err != nil {
		t.Fatalf("creating task: %v", err)
	}

	event := domain.OutboxEvent{ID: "event-1", Subject: "orchestration.task.completed", OccurredAt: time.Now().UTC(), PayloadJSON: []byte(`{"taskId":"x"}`)}
	if _, err := repo.UpdateStatusAndPromote(ctx, tenantID, task.ID, domain.TaskStatusCompleted, event); err != nil {
		t.Fatalf("UpdateStatusAndPromote (with outbox event): %v", err)
	}

	records, err := repo.FetchUnpublished(ctx, 10)
	if err != nil {
		t.Fatalf("FetchUnpublished: %v", err)
	}
	found := false
	for _, rec := range records {
		if rec.ID == event.ID {
			found = true
			if rec.Subject != event.Subject {
				t.Errorf("expected subject %q, got %q", event.Subject, rec.Subject)
			}
		}
	}
	if !found {
		t.Fatalf("expected event %q in unpublished records, got %+v", event.ID, records)
	}

	if err := repo.MarkPublished(ctx, []string{event.ID}); err != nil {
		t.Fatalf("MarkPublished: %v", err)
	}

	after, err := repo.FetchUnpublished(ctx, 10)
	if err != nil {
		t.Fatalf("FetchUnpublished (after mark): %v", err)
	}
	for _, rec := range after {
		if rec.ID == event.ID {
			t.Fatalf("expected event %q to no longer be unpublished", event.ID)
		}
	}
}

// TestRepository_MarkPublished_EmptyIDsIsNoop guards the len(ids)==0 early
// return — same guard usage-service's/issue-tracking-service's own
// MarkPublished carry, since fmt.Sprintf with zero placeholders would
// otherwise build "IN ()", invalid SQL.
func TestRepository_MarkPublished_EmptyIDsIsNoop(t *testing.T) {
	repo := setupRepository(t)
	if err := repo.MarkPublished(context.Background(), nil); err != nil {
		t.Fatalf("expected no error for empty ids, got: %v", err)
	}
}
