//go:build integration

// Integration tests run against a real MySQL via testcontainers-go, per
// specs/backend-go/standards/testing-strategy.md — gated behind the
// "integration" build tag so `go test ./...` (unit tests only) stays fast
// and Docker-free; run these explicitly with
// `go test -tags=integration ./internal/adapter/mysql/...`. Mirrors
// internal/adapter/postgres's test names/shape where a Postgres precedent
// exists, plus TASK-BE-DB-003-pattern tenant-isolation-without-RLS tests
// for tables with a direct tenant_id column (see BE-DB-SOL-016 §6).
package mysql

import (
	"context"
	"database/sql"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	_ "github.com/go-sql-driver/mysql"
	"github.com/google/uuid"

	"github.com/stablyai/orca-go/common/testutil"
	"github.com/stablyai/orca-go/services/project-service/internal/domain"
)

// setupDB starts a disposable MySQL container, runs every mysql migration
// against it, and returns a connected *sql.DB — shared by every *_test.go
// file in this package, mirroring postgres/repository_test.go's setupPool.
func setupDB(t *testing.T) *sql.DB {
	t.Helper()
	rawDSN := testutil.StartMySQL(t, "project")
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
	return db
}

func setupRepository(t *testing.T) *Repository {
	t.Helper()
	return New(setupDB(t))
}

// newTestProject mirrors postgres/repository_test.go's helper of the same
// name exactly — a valid Visibility is required by the CHECK constraint.
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

	project := newTestProject(uuid.NewString(), uuid.NewString(), "my-project")
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

	p := newTestProject(uuid.NewString(), uuid.NewString(), "proj")
	if _, err := repo.Create(ctx, p); err != nil {
		t.Fatalf("create: %v", err)
	}

	if _, err := repo.Get(ctx, uuid.NewString(), p.ID); err != domain.ErrProjectNotFound {
		t.Errorf("expected ErrProjectNotFound for a mismatched tenant, got %v", err)
	}
}

// TestRepository_ListForMember_DoesNotLeakAcrossTenants is the TASK-BE-DB-003
// pattern: two tenants, each with their own project+membership, proving
// application-layer tenant_id scoping alone is sufficient on MySQL, which
// has NO RLS equivalent at all (unlike Postgres, where migrations/postgres/
// 0001_init.up.sql declares a tenant_isolation policy on this exact table —
// see BE-DB-SOL-001 §4 for why that policy was never actually the real
// enforcement even on Postgres).
func TestRepository_ListForMember_DoesNotLeakAcrossTenants(t *testing.T) {
	repo := setupRepository(t)
	ctx := context.Background()

	tenantA, tenantB := uuid.NewString(), uuid.NewString()
	userA, userB := uuid.NewString(), uuid.NewString()

	pa := newTestProject(uuid.NewString(), tenantA, "tenant-a-project")
	if _, err := repo.Create(ctx, pa); err != nil {
		t.Fatalf("create tenant A project: %v", err)
	}
	if err := repo.AddMember(ctx, domain.ProjectMember{ProjectID: pa.ID, UserID: userA, Role: domain.ProjectRoleOwner}); err != nil {
		t.Fatalf("add tenant A member: %v", err)
	}

	pb := newTestProject(uuid.NewString(), tenantB, "tenant-b-project")
	if _, err := repo.Create(ctx, pb); err != nil {
		t.Fatalf("create tenant B project: %v", err)
	}
	if err := repo.AddMember(ctx, domain.ProjectMember{ProjectID: pb.ID, UserID: userB, Role: domain.ProjectRoleOwner}); err != nil {
		t.Fatalf("add tenant B member: %v", err)
	}

	gotA, _, err := repo.ListForMember(ctx, tenantA, userA, "", 50)
	if err != nil {
		t.Fatalf("list for tenant A: %v", err)
	}
	if len(gotA) != 1 || gotA[0].ID != pa.ID {
		t.Fatalf("tenant A should see only its own project, got %+v", gotA)
	}

	// Cross-tenant lookup with tenant A's own user id but tenant B's tenant
	// id must see nothing — proves the WHERE clause, not just the join,
	// enforces isolation.
	gotCross, _, err := repo.ListForMember(ctx, tenantB, userA, "", 50)
	if err != nil {
		t.Fatalf("list cross-tenant: %v", err)
	}
	if len(gotCross) != 0 {
		t.Fatalf("expected no projects leaking across tenants, got %+v", gotCross)
	}
}

// TestRepository_UpdateProject_NoopPatchStillSucceeds is this rollout's
// version of BE-DB-SOL-005 §3.1's RowsAffected() pitfall regression test:
// re-submitting a patch with the SAME values MySQL's default driver would
// report as RowsAffected()==0 for (a "changed rows", not "matched rows"
// count) — UpdateProject must not mistake that for ErrProjectNotFound. See
// repository.go's UpdateProject doc comment for the fix (always re-SELECT,
// never branch on RowsAffected for this method).
func TestRepository_UpdateProject_NoopPatchStillSucceeds(t *testing.T) {
	repo := setupRepository(t)
	ctx := context.Background()

	p := newTestProject(uuid.NewString(), uuid.NewString(), "stable-name")
	if _, err := repo.Create(ctx, p); err != nil {
		t.Fatalf("create: %v", err)
	}

	patch := domain.ProjectUpdatePatch{Name: "stable-name", DefaultBranch: domain.DefaultBranch, Visibility: domain.DefaultVisibility}
	updated, err := repo.UpdateProject(ctx, p.TenantID, p.ID, patch)
	if err != nil {
		t.Fatalf("no-op update project should succeed, got: %v", err)
	}
	if updated.Name != "stable-name" {
		t.Errorf("expected name unchanged, got %q", updated.Name)
	}

	// A second no-op call (now genuinely re-setting identical values on an
	// unchanged row) must ALSO succeed — this is the case that would trip
	// the naive RowsAffected()==0-means-not-found translation.
	if _, err := repo.UpdateProject(ctx, p.TenantID, p.ID, patch); err != nil {
		t.Fatalf("second no-op update should also succeed, got: %v", err)
	}
}

func TestRepository_UpdateDevServerID_NoopStillSucceeds(t *testing.T) {
	repo := setupRepository(t)
	ctx := context.Background()

	p := newTestProject(uuid.NewString(), uuid.NewString(), "proj")
	if _, err := repo.Create(ctx, p); err != nil {
		t.Fatalf("create: %v", err)
	}
	devServer := uuid.NewString()
	if _, err := repo.UpdateDevServerID(ctx, p.TenantID, p.ID, devServer); err != nil {
		t.Fatalf("first rebind: %v", err)
	}
	// Same value again — RowsAffected()==0 on MySQL, must not be mistaken
	// for not-found.
	got, err := repo.UpdateDevServerID(ctx, p.TenantID, p.ID, devServer)
	if err != nil {
		t.Fatalf("no-op rebind should still succeed, got: %v", err)
	}
	if got.DevServerID != devServer {
		t.Errorf("expected dev_server_id %q, got %q", devServer, got.DevServerID)
	}
}

func TestRepository_DeleteProject_NotFoundReturnsSentinel(t *testing.T) {
	repo := setupRepository(t)
	ctx := context.Background()

	if err := repo.DeleteProject(ctx, uuid.NewString(), uuid.NewString()); err != domain.ErrProjectNotFound {
		t.Errorf("expected ErrProjectNotFound, got %v", err)
	}
}

// TestRepository_DeleteProject_CascadesMembers proves ON DELETE CASCADE on
// project_members.project_id works identically on MySQL/InnoDB — the FK is
// declared with explicit `CONSTRAINT ... FOREIGN KEY ... ON DELETE CASCADE`
// (not the ambiguous inline column-REFERENCES form), see
// migrations/mysql/0001_init.up.sql.
func TestRepository_DeleteProject_CascadesMembers(t *testing.T) {
	repo := setupRepository(t)
	ctx := context.Background()

	p := newTestProject(uuid.NewString(), uuid.NewString(), "proj")
	if _, err := repo.Create(ctx, p); err != nil {
		t.Fatalf("create: %v", err)
	}
	userID := uuid.NewString()
	if err := repo.AddMember(ctx, domain.ProjectMember{ProjectID: p.ID, UserID: userID, Role: domain.ProjectRoleOwner}); err != nil {
		t.Fatalf("add member: %v", err)
	}

	if err := repo.DeleteProject(ctx, p.TenantID, p.ID); err != nil {
		t.Fatalf("delete: %v", err)
	}

	if _, err := repo.GetMembership(ctx, p.ID, userID); err != domain.ErrMembershipNotFound {
		t.Errorf("expected membership to cascade-delete, got %v", err)
	}
}
