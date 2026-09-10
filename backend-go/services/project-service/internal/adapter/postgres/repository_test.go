//go:build integration

// Integration tests run against a real Postgres via testcontainers-go, per
// specs/backend-go/standards/testing-strategy.md — gated behind the
// "integration" build tag so `go test ./...` (unit tests only) stays fast
// and Docker-free; run these explicitly with
// `go test -tags=integration ./internal/adapter/postgres/...`.
package postgres

import (
	"context"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/stablyai/orca-go/common/testutil"
	"github.com/stablyai/orca-go/services/project-service/internal/domain"
)

// setupPool starts a disposable Postgres container, runs every migration
// against it, and returns a connected pool — shared by every *_test.go file
// in this package (repo_repository_test.go, worktree_repository_test.go,
// project_group_repository_test.go) so each doesn't spin up its own
// container per entity.
func setupPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	dsn := testutil.StartPostgres(t, "project")

	migrationsPath, err := filepath.Abs("../../../migrations")
	if err != nil {
		t.Fatalf("resolving migrations path: %v", err)
	}
	// Uses the golang-migrate CLI directly rather than importing the
	// library, keeping this test's dependency footprint minimal — swap for
	// the library-based runner once the shared migration-runner helper
	// (referenced in architecture/05-data-architecture.md) exists in common/.
	//
	// Retried a few times: testutil.StartPostgres's wait strategy only waits
	// for the port to accept TCP connections, but the official postgres
	// image's entrypoint briefly opens that port during its own internal
	// "temporary server for initdb" phase before the real server is up,
	// which can race a migrate CLI invocation right after container-ready
	// into "the database system is starting up". Retrying is the pragmatic
	// fix here (in this service's own test file, not touching the shared
	// testutil helper) rather than every caller needing a smarter wait
	// strategy.
	var out []byte
	for attempt := 0; attempt < 5; attempt++ {
		cmd := exec.Command("migrate", "-path", migrationsPath, "-database", dsn, "up")
		out, err = cmd.CombinedOutput()
		if err == nil {
			break
		}
		if !strings.Contains(string(out), "the database system is starting up") {
			break
		}
		time.Sleep(time.Second)
	}
	if err != nil {
		t.Fatalf("running migrations: %v\n%s", err, out)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatalf("connecting to postgres: %v", err)
	}
	t.Cleanup(pool.Close)

	return pool
}

func setupRepository(t *testing.T) *Repository {
	t.Helper()
	return New(setupPool(t))
}

// newTestProject builds a domain.Project the same way usecase.CreateProject
// does — NewProject alone leaves Visibility empty, which the visibility
// CHECK constraint (migrations/0002) now rejects; every integration test
// that inserts a project needs a valid value, matching the usecase's
// default-application step.
func newTestProject(id, tenantID, name string) domain.Project {
	p, err := domain.NewProject(id, tenantID, name, "")
	if err != nil {
		panic(err)
	}
	p.DefaultBranch = domain.DefaultBranch
	p.Visibility = domain.DefaultVisibility
	return p
}

func TestRepository_CreateAndGet_RoundTrips(t *testing.T) {
	repo := setupRepository(t)
	ctx := context.Background()

	project := newTestProject("00000000-0000-0000-0000-000000000001", "11111111-1111-1111-1111-111111111111", "my-project")

	if _, err := repo.Create(ctx, project); err != nil {
		t.Fatalf("create: %v", err)
	}

	got, err := repo.Get(ctx, project.TenantID, project.ID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.Name != "my-project" || got.DevServerID != "" {
		t.Errorf("unexpected project: %+v", got)
	}
}

func TestRepository_Get_FiltersByTenant(t *testing.T) {
	repo := setupRepository(t)
	ctx := context.Background()

	p := newTestProject("00000000-0000-0000-0000-000000000002", "11111111-1111-1111-1111-111111111111", "proj")
	if _, err := repo.Create(ctx, p); err != nil {
		t.Fatalf("create: %v", err)
	}

	if _, err := repo.Get(ctx, "22222222-2222-2222-2222-222222222222", p.ID); err != domain.ErrProjectNotFound {
		t.Errorf("expected ErrProjectNotFound for a mismatched tenant, got %v", err)
	}
}

func TestRepository_UpdateDevServerID(t *testing.T) {
	repo := setupRepository(t)
	ctx := context.Background()

	p := newTestProject("00000000-0000-0000-0000-000000000003", "11111111-1111-1111-1111-111111111111", "proj")
	if _, err := repo.Create(ctx, p); err != nil {
		t.Fatalf("create: %v", err)
	}

	updated, err := repo.UpdateDevServerID(ctx, p.TenantID, p.ID, "33333333-3333-3333-3333-333333333333")
	if err != nil {
		t.Fatalf("update dev server id: %v", err)
	}
	if updated.DevServerID != "33333333-3333-3333-3333-333333333333" {
		t.Errorf("expected dev_server_id to be updated, got %q", updated.DevServerID)
	}
}

// TestRepository_List_EmptyPageToken_ReturnsFirstPage is the regression test
// for BUG-004: List previously bound pageToken="" straight into `id > $2`
// (id is UUID), which Postgres rejected as "invalid input syntax for type
// uuid" on every first-page call.
func TestRepository_List_EmptyPageToken_ReturnsFirstPage(t *testing.T) {
	repo := setupRepository(t)
	ctx := context.Background()
	tenantID := "44444444-4444-4444-4444-444444444444"
	userID := uuid.NewString()

	for i := 0; i < 3; i++ {
		p := newTestProject(uuid.NewString(), tenantID, fmt.Sprintf("project-%d", i))
		if _, err := repo.Create(ctx, p); err != nil {
			t.Fatalf("seeding project %d: %v", i, err)
		}
		if err := repo.AddMember(ctx, domain.ProjectMember{ProjectID: p.ID, UserID: userID, Role: domain.ProjectRoleOwner}); err != nil {
			t.Fatalf("seeding membership for project %d: %v", i, err)
		}
	}

	got, _, err := repo.List(ctx, tenantID, userID, "", 10)
	if err != nil {
		t.Fatalf("List with empty pageToken: %v", err)
	}
	if len(got) != 3 {
		t.Errorf("expected 3 projects, got %d", len(got))
	}
}

// TestRepository_List_ScopesToMembership is the direct regression test for
// the "one private default project per user" pass: a bare tenant_id filter
// previously leaked every tenant member's projects to every other member.
func TestRepository_List_ScopesToMembership(t *testing.T) {
	repo := setupRepository(t)
	ctx := context.Background()
	tenantID := "66666666-6666-6666-6666-666666666666"
	mine := uuid.NewString()
	theirs := uuid.NewString()

	mineProject := newTestProject(uuid.NewString(), tenantID, "mine")
	if _, err := repo.Create(ctx, mineProject); err != nil {
		t.Fatalf("create mine: %v", err)
	}
	if err := repo.AddMember(ctx, domain.ProjectMember{ProjectID: mineProject.ID, UserID: mine, Role: domain.ProjectRoleOwner}); err != nil {
		t.Fatalf("add member to mine: %v", err)
	}

	theirsProject := newTestProject(uuid.NewString(), tenantID, "theirs")
	if _, err := repo.Create(ctx, theirsProject); err != nil {
		t.Fatalf("create theirs: %v", err)
	}
	if err := repo.AddMember(ctx, domain.ProjectMember{ProjectID: theirsProject.ID, UserID: theirs, Role: domain.ProjectRoleOwner}); err != nil {
		t.Fatalf("add member to theirs: %v", err)
	}

	got, _, err := repo.List(ctx, tenantID, mine, "", 10)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(got) != 1 || got[0].ID != mineProject.ID {
		t.Fatalf("want only [mine], got %+v", got)
	}
}

func TestRepository_List_ValidCursor_ReturnsNextPage(t *testing.T) {
	repo := setupRepository(t)
	ctx := context.Background()
	tenantID := "55555555-5555-5555-5555-555555555555"
	userID := uuid.NewString()

	for i := 0; i < 3; i++ {
		p := newTestProject(uuid.NewString(), tenantID, fmt.Sprintf("project-%d", i))
		if _, err := repo.Create(ctx, p); err != nil {
			t.Fatalf("seeding project %d: %v", i, err)
		}
		if err := repo.AddMember(ctx, domain.ProjectMember{ProjectID: p.ID, UserID: userID, Role: domain.ProjectRoleOwner}); err != nil {
			t.Fatalf("seeding membership for project %d: %v", i, err)
		}
	}

	firstPage, next, err := repo.List(ctx, tenantID, userID, "", 2)
	if err != nil {
		t.Fatalf("List (first page): %v", err)
	}
	if len(firstPage) != 2 || next == "" {
		t.Fatalf("expected 2 results and a non-empty cursor, got %d results, next=%q", len(firstPage), next)
	}

	// Guards the fix didn't break the already-working cursor path.
	secondPage, _, err := repo.List(ctx, tenantID, userID, next, 2)
	if err != nil {
		t.Fatalf("List (second page, real cursor): %v", err)
	}
	if len(secondPage) != 1 {
		t.Errorf("expected 1 remaining project on the second page, got %d", len(secondPage))
	}
}
