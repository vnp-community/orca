//go:build integration

// Integration tests run against a real MySQL via testcontainers-go, per
// specs/backend-go/standards/testing-strategy.md — gated behind the
// "integration" build tag so `go test ./...` (unit tests only) stays fast
// and Docker-free; run these explicitly with
// `go test -tags=integration ./internal/adapter/mysql/...`. Mirrors
// internal/adapter/postgres/repository_test.go's test names/shape 1:1,
// plus tenant-isolation-without-RLS tests (TASK-BE-DB-003's pattern) —
// task.tasks/task_grants/task_comments/task_edges all had RLS policies on
// Postgres (migrations/postgres/0001_init.up.sql), none of which exist on
// MySQL (dbcapability.Capabilities.SupportsRLS is false), so tenant
// isolation here rests entirely on the explicit tenant_id filters below.
package mysql

import (
	"context"
	"database/sql"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	_ "github.com/go-sql-driver/mysql"
	"github.com/google/uuid"

	"github.com/stablyai/orca-go/common/testutil"
	"github.com/stablyai/orca-go/services/task-service/internal/domain"
	"github.com/stablyai/orca-go/services/task-service/internal/usecase"
)

func setupRepository(t *testing.T) *Repository {
	t.Helper()
	// testutil.StartMySQL returns "mysql://root:orca@tcp(host:port)/db" —
	// valid as-is for golang-migrate's mysql driver CLI, but
	// go-sql-driver/mysql's database/sql driver needs its own DSN format
	// with the scheme stripped and parseTime=true appended (TIMESTAMP
	// columns need it to scan into time.Time/sql.NullTime).
	rawDSN := testutil.StartMySQL(t, "task")
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

func TestRepository_Create_PersistsAllFields(t *testing.T) {
	repo := setupRepository(t)
	ctx := context.Background()
	tenantID := uuid.NewString()
	dueDate := time.Now().Truncate(time.Second).UTC()
	estimatedHours := 4.5

	task := domain.Task{
		ID: uuid.NewString(), TenantID: tenantID, Title: "Widened task", Status: domain.StatusOpen,
		Description: "a description", Type: "bug", Priority: "high", AssigneeID: uuid.NewString(),
		OwnerID: uuid.NewString(), DueDate: &dueDate, EstimatedHours: &estimatedHours,
		PromptTemplate: "do the thing", AIContext: "extra context", Visibility: "private",
		Labels: []string{"backend", "urgent"},
	}
	created, err := repo.Create(ctx, task)
	if err != nil {
		t.Fatalf("creating task: %v", err)
	}
	if created.ID != task.ID {
		t.Fatalf("expected Create to return the task as given, got %+v", created)
	}
	if created.TaskNumber == 0 {
		t.Error("expected a non-zero task_number to be assigned")
	}

	got, err := repo.Get(ctx, tenantID, task.ID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.Description != task.Description || got.Type != task.Type || got.Priority != task.Priority {
		t.Errorf("unexpected description/type/priority: %+v", got)
	}
	if got.AssigneeID != task.AssigneeID || got.OwnerID != task.OwnerID {
		t.Errorf("unexpected assignee/owner: %+v", got)
	}
	if got.PromptTemplate != task.PromptTemplate || got.AIContext != task.AIContext || got.Visibility != task.Visibility {
		t.Errorf("unexpected prompt_template/ai_context/visibility: %+v", got)
	}
	if got.DueDate == nil || !got.DueDate.Equal(dueDate) {
		t.Errorf("expected DueDate=%v, got %v", dueDate, got.DueDate)
	}
	if got.EstimatedHours == nil || *got.EstimatedHours != estimatedHours {
		t.Errorf("expected EstimatedHours=%v, got %v", estimatedHours, got.EstimatedHours)
	}
	if got.ProgressPercent != 0 {
		t.Errorf("expected default ProgressPercent=0, got %d", got.ProgressPercent)
	}
	// Labels round-trips through migrations/mysql/0011's JSON column
	// (Postgres's TEXT[] has no MySQL array equivalent) — this is the one
	// genuinely new translation risk this column carries.
	if len(got.Labels) != 2 || got.Labels[0] != "backend" || got.Labels[1] != "urgent" {
		t.Errorf("expected labels to round-trip through JSON, got %+v", got.Labels)
	}
}

func TestRepository_Create_DefaultLabelsIsEmptyNotNil(t *testing.T) {
	repo := setupRepository(t)
	ctx := context.Background()
	tenantID := uuid.NewString()

	task, _ := domain.NewTask(uuid.NewString(), tenantID, "no labels", domain.StatusOpen, "", "")
	created, err := repo.Create(ctx, task)
	if err != nil {
		t.Fatalf("creating task: %v", err)
	}
	if created.Labels == nil || len(created.Labels) != 0 {
		t.Errorf("expected an empty (not nil) labels slice, got %+v", created.Labels)
	}

	got, err := repo.Get(ctx, tenantID, task.ID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.Labels == nil || len(got.Labels) != 0 {
		t.Errorf("expected an empty (not nil) labels slice on read-back, got %+v", got.Labels)
	}
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

// TestRepository_List_DoesNotLeakAcrossTenants is the TASK-BE-DB-003
// tenant-isolation-without-RLS regression: task.tasks had an RLS policy on
// Postgres (migrations/postgres/0001_init.up.sql) that MySQL has no
// equivalent for — this proves List's explicit tenant_id filter alone is
// sufficient, using tasks that otherwise look identical (same project_id)
// across two tenants.
func TestRepository_List_DoesNotLeakAcrossTenants(t *testing.T) {
	repo := setupRepository(t)
	ctx := context.Background()
	tenantA := uuid.NewString()
	tenantB := uuid.NewString()
	sharedProjectID := uuid.NewString()

	taskA, _ := domain.NewTask(uuid.NewString(), tenantA, "tenant A's task", domain.StatusOpen, "", sharedProjectID)
	taskB, _ := domain.NewTask(uuid.NewString(), tenantB, "tenant B's task", domain.StatusOpen, "", sharedProjectID)
	if _, err := repo.Create(ctx, taskA); err != nil {
		t.Fatalf("creating tenant A task: %v", err)
	}
	if _, err := repo.Create(ctx, taskB); err != nil {
		t.Fatalf("creating tenant B task: %v", err)
	}

	got, _, err := repo.List(ctx, tenantA, sharedProjectID, "", 0)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(got) != 1 || got[0].ID != taskA.ID {
		t.Fatalf("expected tenant A to see only its own task, got %+v", got)
	}

	if _, err := repo.Get(ctx, tenantA, taskB.ID); err == nil {
		t.Error("expected Get to fail when tenant A requests tenant B's task id")
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
	if err := repo.Update(ctx, tenantID, task, nil); err != nil {
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

	if err := repo.Update(ctx, uuid.NewString(), task, nil); err == nil {
		t.Fatal("expected an error updating a task under the wrong tenant")
	}
}

func TestRepository_FindByNumber_ResolvesWithinProject_NotAcrossProjects(t *testing.T) {
	repo := setupRepository(t)
	ctx := context.Background()
	tenantID := uuid.NewString()
	projectA := uuid.NewString()
	projectB := uuid.NewString()

	task, _ := domain.NewTask(uuid.NewString(), tenantID, "title", domain.StatusOpen, "", projectA)
	created, err := repo.Create(ctx, task)
	if err != nil {
		t.Fatalf("creating task: %v", err)
	}

	got, err := repo.FindByNumber(ctx, tenantID, projectA, created.TaskNumber)
	if err != nil {
		t.Fatalf("FindByNumber within the correct project: %v", err)
	}
	if got.ID != created.ID {
		t.Errorf("expected to resolve task %s, got %s", created.ID, got.ID)
	}

	if _, err := repo.FindByNumber(ctx, tenantID, projectB, created.TaskNumber); err == nil {
		t.Fatal("expected NOT_FOUND resolving the same task_number under a different project")
	}
}

// TestRepository_Create_TaskNumbersAreUniqueAndMonotonic exercises
// migrations/mysql/0008's task_number_seq AUTO_INCREMENT-table emulation of
// Postgres's nextval('task.task_number_seq') across several Create calls —
// the one genuinely novel translation in this migration (MySQL/TiDB has no
// CREATE SEQUENCE).
func TestRepository_Create_TaskNumbersAreUniqueAndMonotonic(t *testing.T) {
	repo := setupRepository(t)
	ctx := context.Background()
	tenantID := uuid.NewString()

	var numbers []int64
	for i := 0; i < 3; i++ {
		task, _ := domain.NewTask(uuid.NewString(), tenantID, "title", domain.StatusOpen, "", "")
		created, err := repo.Create(ctx, task)
		if err != nil {
			t.Fatalf("creating task %d: %v", i, err)
		}
		numbers = append(numbers, created.TaskNumber)
	}
	if numbers[0] == 0 || numbers[1] == 0 || numbers[2] == 0 {
		t.Fatalf("expected every task_number to be non-zero, got %v", numbers)
	}
	if numbers[0] == numbers[1] || numbers[1] == numbers[2] || numbers[0] == numbers[2] {
		t.Fatalf("expected every task_number to be unique, got %v", numbers)
	}
	if !(numbers[0] < numbers[1] && numbers[1] < numbers[2]) {
		t.Fatalf("expected task_number to be monotonically increasing, got %v", numbers)
	}
}

func TestRepository_Update_WithEvents_WritesOutboxRowsInSameTransaction(t *testing.T) {
	repo := setupRepository(t)
	ctx := context.Background()
	tenantID := uuid.NewString()

	task, _ := domain.NewTask(uuid.NewString(), tenantID, "title", domain.StatusOpen, "", "")
	created, err := repo.Create(ctx, task)
	if err != nil {
		t.Fatalf("creating task: %v", err)
	}

	created.Status = domain.StatusCancelled
	events := []domain.OutboxEvent{
		{ID: uuid.NewString(), Subject: "orca.task.task.statuschanged", OccurredAt: time.Now().UTC(), PayloadJSON: []byte(`{}`)},
	}
	if err := repo.Update(ctx, tenantID, created, events); err != nil {
		t.Fatalf("update with events: %v", err)
	}

	unpublished, err := repo.FetchUnpublished(ctx, 10)
	if err != nil {
		t.Fatalf("FetchUnpublished: %v", err)
	}
	if len(unpublished) != 1 || unpublished[0].Subject != "orca.task.task.statuschanged" {
		t.Fatalf("expected exactly 1 unpublished outbox row, got %+v", unpublished)
	}

	if err := repo.MarkPublished(ctx, []string{unpublished[0].ID}); err != nil {
		t.Fatalf("MarkPublished: %v", err)
	}
	afterMark, err := repo.FetchUnpublished(ctx, 10)
	if err != nil {
		t.Fatalf("FetchUnpublished after mark: %v", err)
	}
	if len(afterMark) != 0 {
		t.Errorf("expected no unpublished rows after MarkPublished, got %+v", afterMark)
	}
}

func TestRepository_MarkPublished_EmptyIDsIsNoop(t *testing.T) {
	repo := setupRepository(t)
	ctx := context.Background()

	if err := repo.MarkPublished(ctx, nil); err != nil {
		t.Fatalf("expected a no-op, not an error, for empty ids: %v", err)
	}
}

func TestRepository_Update_NilEvents_WritesNoOutboxRow(t *testing.T) {
	repo := setupRepository(t)
	ctx := context.Background()
	tenantID := uuid.NewString()

	task, _ := domain.NewTask(uuid.NewString(), tenantID, "title", domain.StatusOpen, "", "")
	created, err := repo.Create(ctx, task)
	if err != nil {
		t.Fatalf("creating task: %v", err)
	}

	created.Title = "renamed"
	if err := repo.Update(ctx, tenantID, created, nil); err != nil {
		t.Fatalf("update: %v", err)
	}

	unpublished, err := repo.FetchUnpublished(ctx, 10)
	if err != nil {
		t.Fatalf("FetchUnpublished: %v", err)
	}
	if len(unpublished) != 0 {
		t.Errorf("expected no outbox rows for a nil-event update, got %+v", unpublished)
	}
}

// TestRepository_Delete_CascadesToTaskEdges confirms task_edges' FK
// ON DELETE CASCADE (migrations/mysql/0001_init.up.sql) actually fires on
// InnoDB the same way it does on Postgres.
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

// TestRepository_ListByKind_FiltersByTenantAndKind also exercises
// migrations/mysql/0001's single_parent_key generated-column emulation of
// Postgres's partial unique index — a depends_on edge sharing to_task_id
// with a parent_child edge must NOT collide (single_parent_key is NULL for
// depends_on rows).
func TestRepository_ListByKind_FiltersByTenantAndKind(t *testing.T) {
	repo := setupRepository(t)
	ctx := context.Background()
	tenantID := uuid.NewString()

	a, _ := domain.NewTask(uuid.NewString(), tenantID, "a", domain.StatusOpen, "", "")
	b, _ := domain.NewTask(uuid.NewString(), tenantID, "b", domain.StatusOpen, "", "")
	_, _ = repo.Create(ctx, a)
	_, _ = repo.Create(ctx, b)

	dependsEdge, err := domain.NewTaskEdge(a.ID, b.ID, domain.EdgeKindDependsOn)
	if err != nil {
		t.Fatalf("building depends_on edge: %v", err)
	}
	if err := repo.Add(ctx, tenantID, dependsEdge); err != nil {
		t.Fatalf("adding depends_on edge: %v", err)
	}
	// Same to_task_id (b.ID) as the depends_on edge above, but
	// parent_child — must succeed despite sharing to_task_id, since only
	// ONE parent_child edge per child is constrained, not one edge total.
	parentEdge, err := domain.NewTaskEdge(a.ID, b.ID, domain.EdgeKindParentChild)
	if err != nil {
		t.Fatalf("building parent_child edge: %v", err)
	}
	if err := repo.Add(ctx, tenantID, parentEdge); err != nil {
		t.Fatalf("adding parent_child edge sharing to_task_id with the depends_on edge: %v", err)
	}

	edges, err := repo.ListByKind(ctx, tenantID, domain.EdgeKindDependsOn)
	if err != nil {
		t.Fatalf("list by kind: %v", err)
	}
	if len(edges) != 1 || edges[0].FromTaskID != a.ID {
		t.Errorf("unexpected edges: %+v", edges)
	}
}

// TestRepository_Add_SecondParentChildEdgeForSameChild_Fails proves
// single_parent_key's UNIQUE constraint (migrations/mysql/0001.up.sql)
// actually enforces "at most one parent_child edge per child" on MySQL —
// the real behavior this generated column exists to emulate, not just that
// it doesn't break unrelated inserts (covered above).
func TestRepository_Add_SecondParentChildEdgeForSameChild_Fails(t *testing.T) {
	repo := setupRepository(t)
	ctx := context.Background()
	tenantID := uuid.NewString()

	parent1, _ := domain.NewTask(uuid.NewString(), tenantID, "parent1", domain.StatusOpen, "", "")
	parent2, _ := domain.NewTask(uuid.NewString(), tenantID, "parent2", domain.StatusOpen, "", "")
	child, _ := domain.NewTask(uuid.NewString(), tenantID, "child", domain.StatusOpen, "", "")
	for _, task := range []domain.Task{parent1, parent2, child} {
		if _, err := repo.Create(ctx, task); err != nil {
			t.Fatalf("creating task: %v", err)
		}
	}

	edge1, _ := domain.NewTaskEdge(parent1.ID, child.ID, domain.EdgeKindParentChild)
	if err := repo.Add(ctx, tenantID, edge1); err != nil {
		t.Fatalf("adding first parent_child edge: %v", err)
	}

	edge2, _ := domain.NewTaskEdge(parent2.ID, child.ID, domain.EdgeKindParentChild)
	if err := repo.Add(ctx, tenantID, edge2); err == nil {
		t.Fatal("expected a second parent_child edge for the same child to be rejected by single_parent_key's UNIQUE constraint")
	}
}

func TestRepository_Grant_And_ListGrantsForAncestors(t *testing.T) {
	repo := setupRepository(t)
	ctx := context.Background()
	tenantID := uuid.NewString()

	task, _ := domain.NewTask(uuid.NewString(), tenantID, "task", domain.StatusOpen, "", "")
	_, _ = repo.Create(ctx, task)

	grant := domain.Grant{TaskID: task.ID, SubjectID: uuid.NewString(), Level: domain.GrantLevelOwner, ApplyTree: true}
	grantID, err := repo.Grant(ctx, tenantID, grant)
	if err != nil {
		t.Fatalf("granting: %v", err)
	}
	if grantID == "" {
		t.Error("expected a non-empty grant id")
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

func TestRepository_ListGrantsForAncestors_ExcludesExpiredRows(t *testing.T) {
	repo := setupRepository(t)
	ctx := context.Background()
	tenantID := uuid.NewString()

	task, _ := domain.NewTask(uuid.NewString(), tenantID, "task", domain.StatusOpen, "", "")
	_, _ = repo.Create(ctx, task)

	past := time.Now().Add(-time.Hour)
	future := time.Now().Add(time.Hour)
	if _, err := repo.Grant(ctx, tenantID, domain.Grant{TaskID: task.ID, SubjectID: uuid.NewString(), Level: domain.GrantLevelOwner, ExpiresAt: &past}); err != nil {
		t.Fatalf("granting expired: %v", err)
	}
	if _, err := repo.Grant(ctx, tenantID, domain.Grant{TaskID: task.ID, SubjectID: uuid.NewString(), Level: domain.GrantLevelUser, ExpiresAt: &future}); err != nil {
		t.Fatalf("granting non-expired: %v", err)
	}
	if _, err := repo.Grant(ctx, tenantID, domain.Grant{TaskID: task.ID, SubjectID: uuid.NewString(), Level: domain.GrantLevelAdmin}); err != nil {
		t.Fatalf("granting never-expires: %v", err)
	}

	byTask, err := repo.ListGrantsForAncestors(ctx, tenantID, []string{task.ID})
	if err != nil {
		t.Fatalf("listing grants: %v", err)
	}
	got := byTask[task.ID]
	if len(got) != 2 {
		t.Fatalf("expected 2 non-expired grants, got %d: %+v", len(got), got)
	}
	for _, g := range got {
		if g.Level == domain.GrantLevelOwner {
			t.Errorf("expected the expired Owner grant to be excluded, got %+v", g)
		}
	}
}

func TestRepository_Revoke_And_ListGrantsForTask(t *testing.T) {
	repo := setupRepository(t)
	ctx := context.Background()
	tenantID := uuid.NewString()

	task, _ := domain.NewTask(uuid.NewString(), tenantID, "task", domain.StatusOpen, "", "")
	_, _ = repo.Create(ctx, task)

	subjectID := uuid.NewString()
	grantID, err := repo.Grant(ctx, tenantID, domain.Grant{TaskID: task.ID, SubjectID: subjectID, Level: domain.GrantLevelUser})
	if err != nil {
		t.Fatalf("granting: %v", err)
	}

	got, err := repo.ListGrantsForTask(ctx, tenantID, task.ID)
	if err != nil {
		t.Fatalf("listing grants for task: %v", err)
	}
	if len(got) != 1 || got[0].ID != grantID {
		t.Fatalf("unexpected grants: %+v", got)
	}

	if err := repo.Revoke(ctx, tenantID, task.ID, subjectID, domain.GrantLevelUser); err != nil {
		t.Fatalf("revoking: %v", err)
	}

	got2, err := repo.ListGrantsForTask(ctx, tenantID, task.ID)
	if err != nil {
		t.Fatalf("listing grants for task after revoke: %v", err)
	}
	if len(got2) != 0 {
		t.Errorf("expected no grants after revoke, got %+v", got2)
	}
}

func TestRepository_Revoke_NonexistentGrant_IsIdempotent(t *testing.T) {
	repo := setupRepository(t)
	ctx := context.Background()

	if err := repo.Revoke(ctx, uuid.NewString(), uuid.NewString(), uuid.NewString(), domain.GrantLevelUser); err != nil {
		t.Fatalf("expected a no-op, not an error, for a nonexistent grant: %v", err)
	}
}

// TestRepository_ListGrantsForTask_DoesNotLeakAcrossTenants is the
// tenant-isolation-without-RLS regression for task_grants (RLS on Postgres,
// none on MySQL) — two tenants grant against tasks that share nothing but
// coincidentally-equal subject_id/level, to make sure only an exact
// (tenant_id, task_id) match is ever returned.
func TestRepository_ListGrantsForTask_DoesNotLeakAcrossTenants(t *testing.T) {
	repo := setupRepository(t)
	ctx := context.Background()
	tenantA := uuid.NewString()
	tenantB := uuid.NewString()
	sharedSubjectID := uuid.NewString()

	taskA, _ := domain.NewTask(uuid.NewString(), tenantA, "a", domain.StatusOpen, "", "")
	taskB, _ := domain.NewTask(uuid.NewString(), tenantB, "b", domain.StatusOpen, "", "")
	_, _ = repo.Create(ctx, taskA)
	_, _ = repo.Create(ctx, taskB)

	if _, err := repo.Grant(ctx, tenantA, domain.Grant{TaskID: taskA.ID, SubjectID: sharedSubjectID, Level: domain.GrantLevelOwner}); err != nil {
		t.Fatalf("granting tenant A: %v", err)
	}
	if _, err := repo.Grant(ctx, tenantB, domain.Grant{TaskID: taskB.ID, SubjectID: sharedSubjectID, Level: domain.GrantLevelOwner}); err != nil {
		t.Fatalf("granting tenant B: %v", err)
	}

	gotA, err := repo.ListGrantsForTask(ctx, tenantA, taskA.ID)
	if err != nil {
		t.Fatalf("listing tenant A grants: %v", err)
	}
	if len(gotA) != 1 {
		t.Fatalf("expected exactly 1 grant for tenant A's task, got %+v", gotA)
	}

	// tenantB querying taskA's id (cross-tenant task id guess) must see
	// nothing, even though taskA legitimately has a grant.
	leaked, err := repo.ListGrantsForTask(ctx, tenantB, taskA.ID)
	if err != nil {
		t.Fatalf("listing tenant B against tenant A's task id: %v", err)
	}
	if len(leaked) != 0 {
		t.Errorf("expected no cross-tenant leak, got %+v", leaked)
	}
}

func TestRepository_AddComment_And_ListComments(t *testing.T) {
	repo := setupRepository(t)
	ctx := context.Background()
	tenantID := uuid.NewString()

	task, _ := domain.NewTask(uuid.NewString(), tenantID, "task", domain.StatusOpen, "", "")
	_, _ = repo.Create(ctx, task)

	c, err := domain.NewTaskComment(uuid.NewString(), task.ID, uuid.NewString(), "first comment")
	if err != nil {
		t.Fatalf("building comment: %v", err)
	}
	added, err := repo.AddComment(ctx, tenantID, c)
	if err != nil {
		t.Fatalf("adding comment: %v", err)
	}
	if added.ID == "" || added.Content != "first comment" || added.CreatedAt.IsZero() {
		t.Errorf("unexpected added comment: %+v", added)
	}

	comments, _, err := repo.ListComments(ctx, tenantID, task.ID, "", 0)
	if err != nil {
		t.Fatalf("listing comments: %v", err)
	}
	if len(comments) != 1 || comments[0].ID != added.ID {
		t.Fatalf("unexpected comments: %+v", comments)
	}
}

func TestRepository_RunInTx_CommitsAllWritesTogether(t *testing.T) {
	repo := setupRepository(t)
	ctx := context.Background()
	tenantID := uuid.NewString()

	parent, _ := domain.NewTask(uuid.NewString(), tenantID, "parent", domain.StatusOpen, "", "")
	if _, err := repo.Create(ctx, parent); err != nil {
		t.Fatalf("creating parent: %v", err)
	}

	sub1ID, sub2ID := uuid.NewString(), uuid.NewString()
	err := repo.RunInTx(ctx, func(ctx context.Context, tasks usecase.TaskRepository, edges usecase.EdgeRepository) error {
		sub1, _ := domain.NewTask(sub1ID, tenantID, "sub1", domain.StatusOpen, parent.ID, "")
		if _, err := tasks.Create(ctx, sub1); err != nil {
			return err
		}
		edge1, _ := domain.NewTaskEdge(parent.ID, sub1ID, domain.EdgeKindParentChild)
		if err := edges.Add(ctx, tenantID, edge1); err != nil {
			return err
		}
		sub2, _ := domain.NewTask(sub2ID, tenantID, "sub2", domain.StatusOpen, parent.ID, "")
		if _, err := tasks.Create(ctx, sub2); err != nil {
			return err
		}
		edge2, _ := domain.NewTaskEdge(parent.ID, sub2ID, domain.EdgeKindParentChild)
		return edges.Add(ctx, tenantID, edge2)
	})
	if err != nil {
		t.Fatalf("RunInTx: %v", err)
	}

	if _, err := repo.Get(ctx, tenantID, sub1ID); err != nil {
		t.Errorf("expected sub1 to be committed: %v", err)
	}
	if _, err := repo.Get(ctx, tenantID, sub2ID); err != nil {
		t.Errorf("expected sub2 to be committed: %v", err)
	}
	parentEdges, err := repo.ListFrom(ctx, tenantID, parent.ID, domain.EdgeKindParentChild)
	if err != nil {
		t.Fatalf("listing edges: %v", err)
	}
	if len(parentEdges) != 2 {
		t.Errorf("expected 2 committed parent_child edges, got %d: %+v", len(parentEdges), parentEdges)
	}
}

// TestRepository_RunInTx_RollsBackAllWritesOnError proves the transaction
// actually rolls back on a REAL MySQL/InnoDB database — sub1's task+edge
// insert succeeds, but sub2's edge insert violates single_parent_key's
// UNIQUE constraint (migrations/mysql/0001_init.up.sql) because it reuses
// sub1's to_task_id, a real, naturally-occurring constraint violation —
// mirrors internal/adapter/postgres's identical test against Postgres's
// task_edges_single_parent partial unique index.
func TestRepository_RunInTx_RollsBackAllWritesOnError(t *testing.T) {
	repo := setupRepository(t)
	ctx := context.Background()
	tenantID := uuid.NewString()

	parent, _ := domain.NewTask(uuid.NewString(), tenantID, "parent", domain.StatusOpen, "", "")
	if _, err := repo.Create(ctx, parent); err != nil {
		t.Fatalf("creating parent: %v", err)
	}

	sub1ID := uuid.NewString()
	err := repo.RunInTx(ctx, func(ctx context.Context, tasks usecase.TaskRepository, edges usecase.EdgeRepository) error {
		sub1, _ := domain.NewTask(sub1ID, tenantID, "sub1", domain.StatusOpen, parent.ID, "")
		if _, err := tasks.Create(ctx, sub1); err != nil {
			return err
		}
		edge1, _ := domain.NewTaskEdge(parent.ID, sub1ID, domain.EdgeKindParentChild)
		if err := edges.Add(ctx, tenantID, edge1); err != nil {
			return err
		}
		dupEdge, _ := domain.NewTaskEdge(parent.ID, sub1ID, domain.EdgeKindParentChild)
		return edges.Add(ctx, tenantID, dupEdge)
	})
	if err == nil {
		t.Fatal("expected RunInTx to surface the unique-constraint violation")
	}

	if _, getErr := repo.Get(ctx, tenantID, sub1ID); getErr == nil {
		t.Error("expected sub1 to be rolled back (not found), but it was retrievable")
	}
	parentEdges, listErr := repo.ListFrom(ctx, tenantID, parent.ID, domain.EdgeKindParentChild)
	if listErr != nil {
		t.Fatalf("listing edges: %v", listErr)
	}
	if len(parentEdges) != 0 {
		t.Errorf("expected no parent_child edges to survive the rollback, got %+v", parentEdges)
	}
}

func TestRepository_RecentCompletedTasks_FiltersByStatusAndProject(t *testing.T) {
	repo := setupRepository(t)
	ctx := context.Background()
	tenantID := uuid.NewString()
	projectID := uuid.NewString()

	done, _ := domain.NewTask(uuid.NewString(), tenantID, "done task", domain.StatusOpen, "", projectID)
	created, err := repo.Create(ctx, done)
	if err != nil {
		t.Fatalf("creating task: %v", err)
	}
	created.Status = domain.StatusDone
	if err := repo.Update(ctx, tenantID, created, nil); err != nil {
		t.Fatalf("marking done: %v", err)
	}

	openTask, _ := domain.NewTask(uuid.NewString(), tenantID, "still open", domain.StatusOpen, "", projectID)
	if _, err := repo.Create(ctx, openTask); err != nil {
		t.Fatalf("creating open task: %v", err)
	}

	recent, err := repo.RecentCompletedTasks(ctx, tenantID, projectID, 10)
	if err != nil {
		t.Fatalf("RecentCompletedTasks: %v", err)
	}
	if len(recent) != 1 || recent[0].ID != created.ID {
		t.Fatalf("expected only the done task, got %+v", recent)
	}
}
