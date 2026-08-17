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

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/stablyai/orca-go/common/testutil"
	"github.com/stablyai/orca-go/services/tenant-service/internal/domain"
)

func setupPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	dsn := testutil.StartPostgres(t, "tenant")

	migrationsPath, err := filepath.Abs("../../../migrations")
	if err != nil {
		t.Fatalf("resolving migrations path: %v", err)
	}
	// Uses the golang-migrate CLI directly rather than importing the
	// library, keeping this test's dependency footprint minimal — same
	// approach as usage-service's integration test.
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

	return pool
}

func TestCompanyRepository_CreateAndGetRoundTrip(t *testing.T) {
	pool := setupPool(t)
	repo := NewCompanyRepository(pool)
	ctx := context.Background()

	company, err := domain.NewCompany("11111111-1111-1111-1111-111111111111", "Acme", domain.Settings{"agent": domain.Settings{"model": "sonnet"}})
	if err != nil {
		t.Fatalf("building company: %v", err)
	}
	if _, err := repo.Create(ctx, company); err != nil {
		t.Fatalf("create: %v", err)
	}

	got, found, err := repo.Get(ctx, company.ID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if !found {
		t.Fatal("expected company to be found")
	}
	if got.Name != "Acme" {
		t.Errorf("expected Name=Acme, got %q", got.Name)
	}
}

func TestDepartmentRepository_GetIsScopedByCompanyID(t *testing.T) {
	pool := setupPool(t)
	companies := NewCompanyRepository(pool)
	departments := NewDepartmentRepository(pool)
	ctx := context.Background()

	companyA, _ := domain.NewCompany("11111111-1111-1111-1111-111111111111", "Company A", nil)
	companyB, _ := domain.NewCompany("22222222-2222-2222-2222-222222222222", "Company B", nil)
	_, _ = companies.Create(ctx, companyA)
	_, _ = companies.Create(ctx, companyB)

	dept, _ := domain.NewDepartment("33333333-3333-3333-3333-333333333333", companyA.ID, "Engineering", nil)
	if _, err := departments.Create(ctx, dept); err != nil {
		t.Fatalf("create department: %v", err)
	}

	if _, found, err := departments.Get(ctx, companyB.ID, dept.ID); err != nil {
		t.Fatalf("get: %v", err)
	} else if found {
		t.Error("expected a department from another company to resolve as not-found")
	}

	if _, found, err := departments.Get(ctx, companyA.ID, dept.ID); err != nil {
		t.Fatalf("get: %v", err)
	} else if !found {
		t.Error("expected the department to be found when scoped to its own company")
	}
}

func TestTeamRepository_ListUserTeamLayers(t *testing.T) {
	pool := setupPool(t)
	companies := NewCompanyRepository(pool)
	teams := NewTeamRepository(pool)
	ctx := context.Background()

	company, _ := domain.NewCompany("11111111-1111-1111-1111-111111111111", "Acme", nil)
	_, _ = companies.Create(ctx, company)

	team, _ := domain.NewTeam("33333333-3333-3333-3333-333333333333", company.ID, "Platform", domain.Settings{"editor": domain.Settings{"theme": "dark"}})
	if _, err := teams.Create(ctx, team); err != nil {
		t.Fatalf("create team: %v", err)
	}
	member, _ := domain.NewTeamMember(team.ID, "44444444-4444-4444-4444-444444444444", 5)
	if err := teams.AddMember(ctx, member); err != nil {
		t.Fatalf("add member: %v", err)
	}

	layers, err := teams.ListUserTeamLayers(ctx, company.ID, member.UserID)
	if err != nil {
		t.Fatalf("list user team layers: %v", err)
	}
	if len(layers) != 1 || layers[0].TeamID != team.ID || layers[0].Priority != 5 {
		t.Fatalf("unexpected layers: %+v", layers)
	}
}
