//go:build integration

// Integration tests run against a real Postgres via testcontainers-go, per
// specs/backend-go/standards/testing-strategy.md — gated behind the
// "integration" build tag so `go test ./...` (unit tests only) stays fast
// and Docker-free; run these explicitly with
// `go test -tags=integration ./internal/adapter/postgres/...`.
package postgres

import (
	"context"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/stablyai/orca-go/common/testutil"
	"github.com/stablyai/orca-go/services/task-service/internal/domain"
)

func setupRepository(t *testing.T) *Repository {
	t.Helper()
	dsn := testutil.StartPostgres(t, "task")

	migrationsPath, err := filepath.Abs("../../../migrations")
	if err != nil {
		t.Fatalf("resolving migrations path: %v", err)
	}
	cmd := exec.Command("migrate", "-path", migrationsPath, "-database", dsn, "up")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("running migrations: %v\n%s", err, out)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatalf("connecting to postgres: %v", err)
	}
	t.Cleanup(pool.Close)

	return New(pool)
}

func TestRepository_GetAncestors_WalksParentChainToRoot(t *testing.T) {
	repo := setupRepository(t)
	ctx := context.Background()
	tenantID := uuid.NewString()

	root, _ := domain.NewTask(uuid.NewString(), tenantID, "root", domain.StatusOpen, "", "")
	if _, err := repo.Create(ctx, root); err != nil {
		t.Fatalf("creating root: %v", err)
	}
	child, _ := domain.NewTask(uuid.NewString(), tenantID, "child", domain.StatusOpen, root.ID, "")
	if _, err := repo.Create(ctx, child); err != nil {
		t.Fatalf("creating child: %v", err)
	}
	grandchild, _ := domain.NewTask(uuid.NewString(), tenantID, "grandchild", domain.StatusOpen, child.ID, "")
	if _, err := repo.Create(ctx, grandchild); err != nil {
		t.Fatalf("creating grandchild: %v", err)
	}

	chain, err := repo.GetAncestors(ctx, tenantID, grandchild.ID, 0)
	if err != nil {
		t.Fatalf("get ancestors: %v", err)
	}
	if len(chain) != 3 {
		t.Fatalf("expected a 3-entry chain, got %d: %+v", len(chain), chain)
	}
	if chain[0].ID != grandchild.ID || chain[1].ID != child.ID || chain[2].ID != root.ID {
		t.Errorf("unexpected chain order: %+v", chain)
	}
}

func TestRepository_ListByKind_FiltersByTenantAndKind(t *testing.T) {
	repo := setupRepository(t)
	ctx := context.Background()
	tenantID := uuid.NewString()

	a, _ := domain.NewTask(uuid.NewString(), tenantID, "a", domain.StatusOpen, "", "")
	b, _ := domain.NewTask(uuid.NewString(), tenantID, "b", domain.StatusOpen, "", "")
	_, _ = repo.Create(ctx, a)
	_, _ = repo.Create(ctx, b)

	edge, err := domain.NewTaskEdge(a.ID, b.ID, domain.EdgeKindDependsOn)
	if err != nil {
		t.Fatalf("building edge: %v", err)
	}
	if err := repo.Add(ctx, tenantID, edge); err != nil {
		t.Fatalf("adding edge: %v", err)
	}

	edges, err := repo.ListByKind(ctx, tenantID, domain.EdgeKindDependsOn)
	if err != nil {
		t.Fatalf("list by kind: %v", err)
	}
	if len(edges) != 1 || edges[0].FromTaskID != a.ID {
		t.Errorf("unexpected edges: %+v", edges)
	}

	none, err := repo.ListByKind(ctx, tenantID, domain.EdgeKindParentChild)
	if err != nil {
		t.Fatalf("list by kind (parent_child): %v", err)
	}
	if len(none) != 0 {
		t.Errorf("expected no parent_child edges, got %+v", none)
	}
}

func TestRepository_Grant_And_ListGrantsForAncestors(t *testing.T) {
	repo := setupRepository(t)
	ctx := context.Background()
	tenantID := uuid.NewString()

	task, _ := domain.NewTask(uuid.NewString(), tenantID, "task", domain.StatusOpen, "", "")
	_, _ = repo.Create(ctx, task)

	grant := domain.Grant{TaskID: task.ID, SubjectID: uuid.NewString(), Level: domain.GrantLevelOwner, ApplyTree: true}
	if err := repo.Grant(ctx, tenantID, grant); err != nil {
		t.Fatalf("granting: %v", err)
	}

	byTask, err := repo.ListGrantsForAncestors(ctx, tenantID, []string{task.ID})
	if err != nil {
		t.Fatalf("listing grants: %v", err)
	}
	got := byTask[task.ID]
	if len(got) != 1 || got[0].Level != domain.GrantLevelOwner || !got[0].ApplyTree {
		t.Errorf("unexpected grants: %+v", got)
	}
}

func TestRepository_List_FiltersByTenantAndProject(t *testing.T) {
	repo := setupRepository(t)
	ctx := context.Background()
	tenantID := uuid.NewString()
	otherTenantID := uuid.NewString()
	projectID := uuid.NewString()

	a, _ := domain.NewTask(uuid.NewString(), tenantID, "a", domain.StatusOpen, "", projectID)
	b, _ := domain.NewTask(uuid.NewString(), tenantID, "b", domain.StatusOpen, "", "")
	c, _ := domain.NewTask(uuid.NewString(), otherTenantID, "c", domain.StatusOpen, "", projectID)
	for _, task := range []domain.Task{a, b, c} {
		if _, err := repo.Create(ctx, task); err != nil {
			t.Fatalf("creating task: %v", err)
		}
	}

	got, _, err := repo.List(ctx, tenantID, projectID, "", 0)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(got) != 1 || got[0].ID != a.ID {
		t.Fatalf("expected only task a (tenant+project match), got %+v", got)
	}

	all, _, err := repo.List(ctx, tenantID, "", "", 0)
	if err != nil {
		t.Fatalf("list (no project filter): %v", err)
	}
	if len(all) != 2 {
		t.Fatalf("expected both tenant-1 tasks, got %+v", all)
	}
}

func TestRepository_Update_PersistsTitleAndStatus(t *testing.T) {
	repo := setupRepository(t)
	ctx := context.Background()
	tenantID := uuid.NewString()

	task, _ := domain.NewTask(uuid.NewString(), tenantID, "old title", domain.StatusOpen, "", "")
	if _, err := repo.Create(ctx, task); err != nil {
		t.Fatalf("creating task: %v", err)
	}

	task.Title = "new title"
	task.Status = domain.StatusDone
	if err := repo.Update(ctx, tenantID, task); err != nil {
		t.Fatalf("update: %v", err)
	}

	got, err := repo.Get(ctx, tenantID, task.ID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.Title != "new title" || got.Status != domain.StatusDone {
		t.Errorf("unexpected task after update: %+v", got)
	}
}

func TestRepository_Update_WrongTenant_Fails(t *testing.T) {
	repo := setupRepository(t)
	ctx := context.Background()
	tenantID := uuid.NewString()

	task, _ := domain.NewTask(uuid.NewString(), tenantID, "title", domain.StatusOpen, "", "")
	if _, err := repo.Create(ctx, task); err != nil {
		t.Fatalf("creating task: %v", err)
	}

	if err := repo.Update(ctx, uuid.NewString(), task); err == nil {
		t.Fatal("expected an error updating a task under the wrong tenant")
	}
}

// TestRepository_Delete_CascadesToTaskEdges confirms task_edges' ON DELETE
// CASCADE (migrations/0001_init.up.sql) actually fires: deleting a task
// that has an edge referencing it leaves no orphaned edge row.
func TestRepository_Delete_CascadesToTaskEdges(t *testing.T) {
	repo := setupRepository(t)
	ctx := context.Background()
	tenantID := uuid.NewString()

	parent, _ := domain.NewTask(uuid.NewString(), tenantID, "parent", domain.StatusOpen, "", "")
	child, _ := domain.NewTask(uuid.NewString(), tenantID, "child", domain.StatusOpen, "", "")
	if _, err := repo.Create(ctx, parent); err != nil {
		t.Fatalf("creating parent: %v", err)
	}
	if _, err := repo.Create(ctx, child); err != nil {
		t.Fatalf("creating child: %v", err)
	}
	edge, _ := domain.NewTaskEdge(parent.ID, child.ID, domain.EdgeKindParentChild)
	if err := repo.Add(ctx, tenantID, edge); err != nil {
		t.Fatalf("adding edge: %v", err)
	}

	if err := repo.Delete(ctx, tenantID, parent.ID); err != nil {
		t.Fatalf("delete: %v", err)
	}

	edges, err := repo.ListByKind(ctx, tenantID, domain.EdgeKindParentChild)
	if err != nil {
		t.Fatalf("list by kind: %v", err)
	}
	for _, e := range edges {
		if e.FromTaskID == parent.ID {
			t.Errorf("expected the edge referencing the deleted parent to be gone via cascade, found %+v", e)
		}
	}
}

func TestRepository_Delete_NotFound_Fails(t *testing.T) {
	repo := setupRepository(t)
	ctx := context.Background()

	if err := repo.Delete(ctx, uuid.NewString(), uuid.NewString()); err == nil {
		t.Fatal("expected an error deleting a nonexistent task")
	}
}
