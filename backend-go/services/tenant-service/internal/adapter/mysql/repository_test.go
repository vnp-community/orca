//go:build integration

// Integration tests run against a real MySQL via testcontainers-go, per
// specs/backend-go/standards/testing-strategy.md — gated behind the
// "integration" build tag so `go test ./...` (unit tests only) stays fast
// and Docker-free; run these explicitly with
// `go test -tags=integration ./internal/adapter/mysql/...`. Mirrors
// internal/adapter/postgres's test names/shape where a Postgres equivalent
// exists, plus explicit "DoesNotLeakAcrossTenants" tests (TASK-BE-DB-003's
// pattern) for every table that carries an RLS policy in
// migrations/postgres — migrations/mysql has NO RLS equivalent at all (not
// just an inert one), so this package's own company_id/companyID filtering
// is the ONLY tenant-isolation enforcement on this dialect and must be
// proven directly, not assumed from the Postgres adapter's behavior. See
// BE-DB-SOL-011 §"Kết quả thực tế" for the full list of tables audited.
package mysql

import (
	"context"
	"database/sql"
	"encoding/json"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	_ "github.com/go-sql-driver/mysql"

	"github.com/stablyai/orca-go/common/testutil"
	"github.com/stablyai/orca-go/services/tenant-service/internal/domain"
)

func setupDB(t *testing.T) *sql.DB {
	t.Helper()
	// testutil.StartMySQL returns "mysql://root:orca@tcp(host:port)/db" —
	// valid as-is for golang-migrate's mysql driver CLI (used below), but
	// go-sql-driver/mysql's database/sql driver expects its OWN DSN format
	// with no "mysql://" scheme. parseTime=true is required for TIMESTAMP
	// columns to scan into time.Time instead of []byte.
	rawDSN := testutil.StartMySQL(t, "tenant")
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

// ---------------------------------------------------------------------
// CompanyRepository
// ---------------------------------------------------------------------

func TestCompanyRepository_CreateAndGetRoundTrip(t *testing.T) {
	db := setupDB(t)
	repo := NewCompanyRepository(db)
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

func TestCompanyRepository_Update_NotFoundReturnsFalse(t *testing.T) {
	db := setupDB(t)
	repo := NewCompanyRepository(db)
	ctx := context.Background()

	_, found, err := repo.Update(ctx, "99999999-9999-9999-9999-999999999999", domain.CompanySettingsPatch{Name: "New Name"})
	if err != nil {
		t.Fatalf("update: %v", err)
	}
	if found {
		t.Error("expected found=false for an id that doesn't exist — must not be derived from RowsAffected()")
	}
}

func TestCompanyRepository_Update_AppliesNonEmptyFieldsOnly(t *testing.T) {
	db := setupDB(t)
	repo := NewCompanyRepository(db)
	ctx := context.Background()

	company, _ := domain.NewCompany("22222222-2222-2222-2222-222222222222", "Original", domain.Settings{"agent": domain.Settings{"model": "sonnet"}})
	if _, err := repo.Create(ctx, company); err != nil {
		t.Fatalf("create: %v", err)
	}

	// A no-op-value retry (same name) must still report found=true, never a
	// false not-found derived from RowsAffected()==0 (MySQL's UPDATE counts
	// CHANGED rows, not MATCHED rows — see this repository's Update doc
	// comment).
	got, found, err := repo.Update(ctx, company.ID, domain.CompanySettingsPatch{Name: "Original"})
	if err != nil {
		t.Fatalf("no-op update: %v", err)
	}
	if !found {
		t.Fatal("expected a no-op-value update to still report found=true")
	}
	if got.Name != "Original" {
		t.Errorf("expected Name unchanged, got %q", got.Name)
	}

	got, found, err = repo.Update(ctx, company.ID, domain.CompanySettingsPatch{Name: "Renamed"})
	if err != nil {
		t.Fatalf("update: %v", err)
	}
	if !found || got.Name != "Renamed" {
		t.Fatalf("expected Name=Renamed found=true, got %+v found=%v", got, found)
	}
}

// TestCompanyRepository_List seeds 2 companies on top of whatever the
// migrations themselves already inserted — migrations/mysql/0002_backfill_legacy_bootstrap_company.up.sql
// always seeds a "Legacy Bootstrap Company" row (mirrors the Postgres
// migration 1:1), so this asserts List surfaces the 2 seeded companies
// (ordered by name) rather than asserting an exact total row count.
func TestCompanyRepository_List(t *testing.T) {
	db := setupDB(t)
	repo := NewCompanyRepository(db)
	ctx := context.Background()

	a, _ := domain.NewCompany("33333333-3333-3333-3333-333333333333", "B Company", nil)
	b, _ := domain.NewCompany("44444444-4444-4444-4444-444444444444", "A Company", nil)
	if _, err := repo.Create(ctx, a); err != nil {
		t.Fatalf("create a: %v", err)
	}
	if _, err := repo.Create(ctx, b); err != nil {
		t.Fatalf("create b: %v", err)
	}

	got, err := repo.List(ctx)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	byName := make(map[string]bool, len(got))
	for i, c := range got {
		byName[c.Name] = true
		if i > 0 && got[i-1].Name > c.Name {
			t.Fatalf("expected companies ordered by name, got %+v", got)
		}
	}
	if !byName["A Company"] || !byName["B Company"] {
		t.Fatalf("expected both seeded companies present, got %+v", got)
	}
}

// ---------------------------------------------------------------------
// DepartmentRepository
// ---------------------------------------------------------------------

func TestDepartmentRepository_GetIsScopedByCompanyID(t *testing.T) {
	db := setupDB(t)
	companies := NewCompanyRepository(db)
	departments := NewDepartmentRepository(db)
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

func TestDepartmentRepository_ExistsByNameIsScopedByCompanyID(t *testing.T) {
	db := setupDB(t)
	companies := NewCompanyRepository(db)
	departments := NewDepartmentRepository(db)
	ctx := context.Background()

	companyA, _ := domain.NewCompany("44444444-4444-4444-4444-444444444444", "Company A", nil)
	companyB, _ := domain.NewCompany("55555555-5555-5555-5555-555555555555", "Company B", nil)
	_, _ = companies.Create(ctx, companyA)
	_, _ = companies.Create(ctx, companyB)

	dept, _ := domain.NewDepartment("66666666-6666-6666-6666-666666666666", companyA.ID, "Engineering", nil)
	if _, err := departments.Create(ctx, dept); err != nil {
		t.Fatalf("create department: %v", err)
	}

	if exists, err := departments.ExistsByName(ctx, companyB.ID, "Engineering"); err != nil {
		t.Fatalf("exists by name: %v", err)
	} else if exists {
		t.Error("expected ExistsByName to be scoped by company_id, not find a same-named department in a different company")
	}
}

// TestDepartmentRepository_List_DoesNotLeakAcrossTenants mirrors
// TASK-BE-DB-003's pattern directly: two companies each with a department
// of the SAME name/id-shape, List scoped to company A must never surface
// company B's row. migrations/mysql/0001_init.up.sql has NO RLS on
// departments — this test IS the enforcement proof for this dialect.
func TestDepartmentRepository_List_DoesNotLeakAcrossTenants(t *testing.T) {
	db := setupDB(t)
	companies := NewCompanyRepository(db)
	departments := NewDepartmentRepository(db)
	ctx := context.Background()

	companyA, _ := domain.NewCompany("77777777-7777-7777-7777-777777777777", "Company A", nil)
	companyB, _ := domain.NewCompany("88888888-8888-8888-8888-888888888888", "Company B", nil)
	_, _ = companies.Create(ctx, companyA)
	_, _ = companies.Create(ctx, companyB)

	deptA, _ := domain.NewDepartment("99999999-9999-9999-9999-999999999991", companyA.ID, "Engineering", nil)
	deptB, _ := domain.NewDepartment("99999999-9999-9999-9999-999999999992", companyB.ID, "Engineering", nil)
	if _, err := departments.Create(ctx, deptA); err != nil {
		t.Fatalf("create deptA: %v", err)
	}
	if _, err := departments.Create(ctx, deptB); err != nil {
		t.Fatalf("create deptB: %v", err)
	}

	got, err := departments.List(ctx, companyA.ID)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(got) != 1 || got[0].ID != deptA.ID {
		t.Fatalf("expected exactly company A's own department, got %+v", got)
	}
}

// ---------------------------------------------------------------------
// TeamRepository
// ---------------------------------------------------------------------

func TestTeamRepository_ListUserTeamLayers(t *testing.T) {
	db := setupDB(t)
	companies := NewCompanyRepository(db)
	teams := NewTeamRepository(db)
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

func TestTeamRepository_RemoveMember(t *testing.T) {
	db := setupDB(t)
	companies := NewCompanyRepository(db)
	teams := NewTeamRepository(db)
	ctx := context.Background()

	company, _ := domain.NewCompany("11111111-1111-1111-1111-111111111111", "Acme", nil)
	_, _ = companies.Create(ctx, company)

	team, _ := domain.NewTeam("33333333-3333-3333-3333-333333333333", company.ID, "Platform", nil)
	if _, err := teams.Create(ctx, team); err != nil {
		t.Fatalf("create team: %v", err)
	}
	member, _ := domain.NewTeamMember(team.ID, "44444444-4444-4444-4444-444444444444", 5)
	if err := teams.AddMember(ctx, member); err != nil {
		t.Fatalf("add member: %v", err)
	}

	removed, err := teams.RemoveMember(ctx, team.ID, member.UserID)
	if err != nil {
		t.Fatalf("remove member: %v", err)
	}
	if !removed {
		t.Fatal("expected RemoveMember to report the row was found and removed")
	}

	remaining, err := teams.ListMembers(ctx, team.ID)
	if err != nil {
		t.Fatalf("list members: %v", err)
	}
	if len(remaining) != 0 {
		t.Fatalf("expected member to be gone, got %+v", remaining)
	}

	// Removing again is an idempotent no-op, not an error — DELETE's
	// RowsAffected() counts WHERE-matched rows on every dialect (no
	// UPDATE-style "changed value" ambiguity), safe to read directly.
	removedAgain, err := teams.RemoveMember(ctx, team.ID, member.UserID)
	if err != nil {
		t.Fatalf("remove member again: %v", err)
	}
	if removedAgain {
		t.Fatal("expected second RemoveMember call to report nothing was found")
	}
}

// TestTeamRepository_ListByCompany_DoesNotLeakAcrossTenants mirrors
// TASK-BE-DB-003's pattern for teams — no RLS equivalent on this dialect.
func TestTeamRepository_ListByCompany_DoesNotLeakAcrossTenants(t *testing.T) {
	db := setupDB(t)
	companies := NewCompanyRepository(db)
	teams := NewTeamRepository(db)
	ctx := context.Background()

	companyA, _ := domain.NewCompany("11111111-1111-1111-1111-111111111111", "Company A", nil)
	companyB, _ := domain.NewCompany("22222222-2222-2222-2222-222222222222", "Company B", nil)
	_, _ = companies.Create(ctx, companyA)
	_, _ = companies.Create(ctx, companyB)

	teamA, _ := domain.NewTeam("33333333-3333-3333-3333-333333333333", companyA.ID, "Platform", nil)
	teamB, _ := domain.NewTeam("44444444-4444-4444-4444-444444444444", companyB.ID, "Platform", nil)
	if _, err := teams.Create(ctx, teamA); err != nil {
		t.Fatalf("create teamA: %v", err)
	}
	if _, err := teams.Create(ctx, teamB); err != nil {
		t.Fatalf("create teamB: %v", err)
	}

	got, err := teams.ListByCompany(ctx, companyA.ID)
	if err != nil {
		t.Fatalf("list by company: %v", err)
	}
	if len(got) != 1 || got[0].ID != teamA.ID {
		t.Fatalf("expected exactly company A's own team, got %+v", got)
	}
}

// ---------------------------------------------------------------------
// UserProfileRepository / ClientStateRepository
// ---------------------------------------------------------------------

func TestUserProfileRepository_GetClientStateColumn_NotFoundWhenNoRow(t *testing.T) {
	db := setupDB(t)
	companies := NewCompanyRepository(db)
	repo := NewUserProfileRepository(db)
	ctx := context.Background()

	company, _ := domain.NewCompany("11111111-1111-1111-1111-111111111111", "Acme", nil)
	if _, err := companies.Create(ctx, company); err != nil {
		t.Fatalf("create company: %v", err)
	}

	_, found, err := repo.GetClientStateColumn(ctx, company.ID, "22222222-2222-2222-2222-222222222222", "keybindings_json")
	if err != nil {
		t.Fatalf("get client state column: %v", err)
	}
	if found {
		t.Error("expected found=false when no profile row exists at all")
	}
}

func TestUserProfileRepository_SetClientStateColumn_ThenGet_RoundTrips(t *testing.T) {
	db := setupDB(t)
	companies := NewCompanyRepository(db)
	repo := NewUserProfileRepository(db)
	ctx := context.Background()

	company, _ := domain.NewCompany("11111111-1111-1111-1111-111111111111", "Acme", nil)
	if _, err := companies.Create(ctx, company); err != nil {
		t.Fatalf("create company: %v", err)
	}
	userID := "44444444-4444-4444-4444-444444444444"

	for column, value := range map[string]string{
		"keybindings_json":                `{"save":"cmd+s"}`,
		"ui_local_state_json":             `{"sidebarWidth":240}`,
		"saved_runtime_environments_json": `{"envs":[]}`,
		"client_settings_json":            `{"theme":"dark"}`,
		"accounts_dev_server_json":        `{"acct1":"dev1"}`,
	} {
		if err := repo.SetClientStateColumn(ctx, company.ID, userID, column, value); err != nil {
			t.Fatalf("set %s: %v", column, err)
		}
		got, found, err := repo.GetClientStateColumn(ctx, company.ID, userID, column)
		if err != nil {
			t.Fatalf("get %s: %v", column, err)
		}
		if !found {
			t.Fatalf("expected found=true for %s after saving", column)
		}
		if got != value {
			t.Errorf("column %s: want %q, got %q", column, value, got)
		}
	}
}

func TestUserProfileRepository_ClientStateColumn_UnknownColumnRejected(t *testing.T) {
	db := setupDB(t)
	repo := NewUserProfileRepository(db)
	ctx := context.Background()

	if _, _, err := repo.GetClientStateColumn(ctx, "11111111-1111-1111-1111-111111111111", "22222222-2222-2222-2222-222222222222", "settings_json; DROP TABLE user_profiles;--"); err == nil {
		t.Fatal("expected an error for an unwhitelisted column name, not a query built from it")
	}
	if err := repo.SetClientStateColumn(ctx, "11111111-1111-1111-1111-111111111111", "22222222-2222-2222-2222-222222222222", "not_a_real_column", "{}"); err == nil {
		t.Fatal("expected an error for an unwhitelisted column name on Set too")
	}
}

func TestUserProfileRepository_OnboardingState_RoundTrips(t *testing.T) {
	db := setupDB(t)
	companies := NewCompanyRepository(db)
	repo := NewUserProfileRepository(db)
	ctx := context.Background()

	company, _ := domain.NewCompany("11111111-1111-1111-1111-111111111111", "Acme", nil)
	if _, err := companies.Create(ctx, company); err != nil {
		t.Fatalf("create company: %v", err)
	}
	userID := "55555555-5555-5555-5555-555555555555"

	if _, found, err := repo.GetOnboardingState(ctx, company.ID, userID); err != nil || found {
		t.Fatalf("expected not-found before any save: found=%v err=%v", found, err)
	}

	if err := repo.SetOnboardingState(ctx, company.ID, userID, `{"step":2}`); err != nil {
		t.Fatalf("set onboarding state: %v", err)
	}
	got, found, err := repo.GetOnboardingState(ctx, company.ID, userID)
	if err != nil || !found {
		t.Fatalf("get onboarding state: found=%v err=%v", found, err)
	}
	// Semantic JSON comparison, not byte-for-byte: onboarding_state_json is a
	// native JSON column (migrations/mysql/0003_add_onboarding_state.up.sql)
	// — MySQL re-serializes on write (observed: `{"step":2}` round-trips as
	// `{"step": 2}`, a space after the colon), the same canonicalization
	// Postgres's JSONB does for its own `{"step": 2}` text output. Not a
	// production bug — GetOnboardingState/SetOnboardingState never assume
	// byte-identical round-trip, only valid JSON.
	var gotVal, wantVal map[string]any
	if err := json.Unmarshal([]byte(got), &gotVal); err != nil {
		t.Fatalf("unmarshal got onboarding state %q: %v", got, err)
	}
	if err := json.Unmarshal([]byte(`{"step":2}`), &wantVal); err != nil {
		t.Fatalf("unmarshal want onboarding state: %v", err)
	}
	if gotVal["step"] != wantVal["step"] {
		t.Errorf("want step=%v, got %v (raw got=%q)", wantVal["step"], gotVal["step"], got)
	}
}

// TestUserProfileRepository_ClientStateColumn_DoesNotLeakAcrossTenants
// mirrors TASK-BE-DB-003's pattern for user_profiles — no RLS equivalent on
// this dialect (migrations/mysql/0001_init.up.sql).
func TestUserProfileRepository_ClientStateColumn_DoesNotLeakAcrossTenants(t *testing.T) {
	db := setupDB(t)
	companies := NewCompanyRepository(db)
	repo := NewUserProfileRepository(db)
	ctx := context.Background()

	companyA, _ := domain.NewCompany("11111111-1111-1111-1111-111111111111", "Company A", nil)
	companyB, _ := domain.NewCompany("22222222-2222-2222-2222-222222222222", "Company B", nil)
	_, _ = companies.Create(ctx, companyA)
	_, _ = companies.Create(ctx, companyB)

	userID := "66666666-6666-6666-6666-666666666666"
	if err := repo.SetClientStateColumn(ctx, companyA.ID, userID, "keybindings_json", `{"save":"cmd+s"}`); err != nil {
		t.Fatalf("set under company A: %v", err)
	}

	_, found, err := repo.GetClientStateColumn(ctx, companyB.ID, userID, "keybindings_json")
	if err != nil {
		t.Fatalf("get scoped to company B: %v", err)
	}
	if found {
		t.Error("expected a row belonging to a different company to resolve as not-found")
	}

	ids, err := repo.ListUserIDsByCompany(ctx, companyB.ID)
	if err != nil {
		t.Fatalf("list user ids by company B: %v", err)
	}
	if len(ids) != 0 {
		t.Errorf("expected company B to see none of company A's users, got %v", ids)
	}
}

// ---------------------------------------------------------------------
// UserWorkspaceSessionRepository
// ---------------------------------------------------------------------

func TestUserWorkspaceSessionRepository_SetThenGet_RoundTrips(t *testing.T) {
	db := setupDB(t)
	companies := NewCompanyRepository(db)
	repo := NewUserWorkspaceSessionRepository(db)
	ctx := context.Background()

	company, _ := domain.NewCompany("11111111-1111-1111-1111-111111111111", "Acme", nil)
	if _, err := companies.Create(ctx, company); err != nil {
		t.Fatalf("create company: %v", err)
	}
	userID := "33333333-3333-3333-3333-333333333333"

	if err := repo.Set(ctx, company.ID, userID, "local", `{"tabs":["a","b"]}`); err != nil {
		t.Fatalf("set: %v", err)
	}
	got, found, err := repo.Get(ctx, company.ID, userID, "local")
	if err != nil || !found {
		t.Fatalf("get: found=%v err=%v", found, err)
	}
	if got != `{"tabs":["a","b"]}` {
		t.Errorf("want the saved session round-tripped, got %q", got)
	}
}

func TestUserWorkspaceSessionRepository_Patch_MergesIntoExisting(t *testing.T) {
	db := setupDB(t)
	companies := NewCompanyRepository(db)
	repo := NewUserWorkspaceSessionRepository(db)
	ctx := context.Background()

	company, _ := domain.NewCompany("11111111-1111-1111-1111-111111111111", "Acme", nil)
	if _, err := companies.Create(ctx, company); err != nil {
		t.Fatalf("create company: %v", err)
	}
	userID := "77777777-7777-7777-7777-777777777777"

	if err := repo.Set(ctx, company.ID, userID, "local", `{"a":1,"b":2}`); err != nil {
		t.Fatalf("set: %v", err)
	}
	if err := repo.Patch(ctx, company.ID, userID, "local", `{"b":3,"c":4}`); err != nil {
		t.Fatalf("patch: %v", err)
	}

	got, found, err := repo.Get(ctx, company.ID, userID, "local")
	if err != nil || !found {
		t.Fatalf("get: found=%v err=%v", found, err)
	}
	want := `{"a":1,"b":3,"c":4}`
	if got != want {
		t.Errorf("want merged session %s, got %s", want, got)
	}
}

func TestUserWorkspaceSessionRepository_Patch_CreatesRowWhenNoneExists(t *testing.T) {
	db := setupDB(t)
	companies := NewCompanyRepository(db)
	repo := NewUserWorkspaceSessionRepository(db)
	ctx := context.Background()

	company, _ := domain.NewCompany("11111111-1111-1111-1111-111111111111", "Acme", nil)
	if _, err := companies.Create(ctx, company); err != nil {
		t.Fatalf("create company: %v", err)
	}
	userID := "88888888-8888-8888-8888-888888888888"

	if err := repo.Patch(ctx, company.ID, userID, "local", `{"first":true}`); err != nil {
		t.Fatalf("patch: %v", err)
	}
	got, found, err := repo.Get(ctx, company.ID, userID, "local")
	if err != nil || !found {
		t.Fatalf("get: found=%v err=%v", found, err)
	}
	if got != `{"first":true}` {
		t.Errorf("want %s, got %s", `{"first":true}`, got)
	}
}

// TestUserWorkspaceSessionRepository_Patch_ConcurrentPatchesDoNotLoseFields
// is the real-MySQL regression guard mirroring BE-SOL-STORAGE-001 §7's
// Postgres test — two goroutines patch DIFFERENT fields of the same session
// at (approximately) the same time; InnoDB's SELECT ... FOR UPDATE must
// serialize them so both fields survive.
func TestUserWorkspaceSessionRepository_Patch_ConcurrentPatchesDoNotLoseFields(t *testing.T) {
	db := setupDB(t)
	companies := NewCompanyRepository(db)
	repo := NewUserWorkspaceSessionRepository(db)
	ctx := context.Background()

	company, _ := domain.NewCompany("11111111-1111-1111-1111-111111111111", "Acme", nil)
	if _, err := companies.Create(ctx, company); err != nil {
		t.Fatalf("create company: %v", err)
	}
	userID := "99999999-9999-9999-9999-999999999999"
	if err := repo.Set(ctx, company.ID, userID, "local", `{}`); err != nil {
		t.Fatalf("seed initial row: %v", err)
	}

	var wg sync.WaitGroup
	errs := make(chan error, 2)
	wg.Add(2)
	go func() {
		defer wg.Done()
		if err := repo.Patch(ctx, company.ID, userID, "local", `{"fieldA":1}`); err != nil {
			errs <- err
		}
	}()
	go func() {
		defer wg.Done()
		if err := repo.Patch(ctx, company.ID, userID, "local", `{"fieldB":2}`); err != nil {
			errs <- err
		}
	}()
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Fatalf("concurrent patch failed: %v", err)
	}

	got, found, err := repo.Get(ctx, company.ID, userID, "local")
	if err != nil || !found {
		t.Fatalf("get: found=%v err=%v", found, err)
	}
	if got != `{"fieldA":1,"fieldB":2}` && got != `{"fieldB":2,"fieldA":1}` {
		t.Errorf("expected both concurrent patches' fields to survive, got %s", got)
	}
}

// TestUserWorkspaceSessionRepository_DoesNotLeakAcrossTenants mirrors
// TASK-BE-DB-003's pattern for user_workspace_sessions — no RLS equivalent
// on this dialect.
func TestUserWorkspaceSessionRepository_DoesNotLeakAcrossTenants(t *testing.T) {
	db := setupDB(t)
	companies := NewCompanyRepository(db)
	repo := NewUserWorkspaceSessionRepository(db)
	ctx := context.Background()

	companyA, _ := domain.NewCompany("11111111-1111-1111-1111-111111111111", "Company A", nil)
	companyB, _ := domain.NewCompany("22222222-2222-2222-2222-222222222222", "Company B", nil)
	_, _ = companies.Create(ctx, companyA)
	_, _ = companies.Create(ctx, companyB)

	// Same user_id/host_id pair intentionally re-used under a different
	// company: user_workspace_sessions' PRIMARY KEY is (user_id, host_id)
	// with no company_id component, so this is the exact shape where a
	// missing company_id filter would silently return the wrong tenant's
	// row instead of not-found.
	userID, hostID := "66666666-6666-6666-6666-666666666666", "local"
	if err := repo.Set(ctx, companyA.ID, userID, hostID, `{"tenant":"A"}`); err != nil {
		t.Fatalf("set under company A: %v", err)
	}

	_, found, err := repo.Get(ctx, companyB.ID, userID, hostID)
	if err != nil {
		t.Fatalf("get scoped to company B: %v", err)
	}
	if found {
		t.Error("expected a session belonging to a different company to resolve as not-found, not company A's row")
	}
}

// ---------------------------------------------------------------------
// StarNagStateRepository
// ---------------------------------------------------------------------

func TestStarNagStateRepository_GetOrCreate_LazilyInsertsDefault(t *testing.T) {
	db := setupDB(t)
	companies := NewCompanyRepository(db)
	repo := NewStarNagStateRepository(db)
	ctx := context.Background()

	company, _ := domain.NewCompany("11111111-1111-1111-1111-111111111111", "Acme", nil)
	if _, err := companies.Create(ctx, company); err != nil {
		t.Fatalf("create company: %v", err)
	}

	userID := "22222222-2222-2222-2222-222222222222"
	state, err := repo.GetOrCreate(ctx, company.ID, userID)
	if err != nil {
		t.Fatalf("get or create: %v", err)
	}
	if state.NextThreshold != domain.StarNagInitialThreshold {
		t.Errorf("expected NextThreshold=%d, got %d", domain.StarNagInitialThreshold, state.NextThreshold)
	}
	if state.Completed {
		t.Error("expected Completed=false for a brand-new row")
	}
}

func TestStarNagStateRepository_Save_RoundTripsActivePrompt(t *testing.T) {
	db := setupDB(t)
	companies := NewCompanyRepository(db)
	repo := NewStarNagStateRepository(db)
	ctx := context.Background()

	company, _ := domain.NewCompany("11111111-1111-1111-1111-111111111111", "Acme", nil)
	if _, err := companies.Create(ctx, company); err != nil {
		t.Fatalf("create company: %v", err)
	}

	userID := "44444444-4444-4444-4444-444444444444"
	state, err := repo.GetOrCreate(ctx, company.ID, userID)
	if err != nil {
		t.Fatalf("get or create: %v", err)
	}

	baseline := int64(12)
	appVersion := "1.2.3"
	deferredUntil := time.Now().Add(domain.StarNagCooldown).UTC().Truncate(time.Second)
	state.BaselineAgents = &baseline
	state.AppVersion = &appVersion
	state.DeferredUntil = &deferredUntil
	state.ActivePrompt = &domain.ActiveStarNagPrompt{
		Source:            "force_show",
		Mode:              "gh",
		Surface:           "card",
		OpenedRepoTracked: true,
		ShownAt:           time.Now().UTC().Truncate(time.Second),
	}
	if err := repo.Save(ctx, state); err != nil {
		t.Fatalf("save: %v", err)
	}

	got, err := repo.GetOrCreate(ctx, company.ID, userID)
	if err != nil {
		t.Fatalf("re-fetch: %v", err)
	}
	if got.ActivePrompt == nil {
		t.Fatal("expected ActivePrompt to round-trip non-nil")
	}
	if got.ActivePrompt.Source != "force_show" || got.ActivePrompt.Mode != "gh" ||
		got.ActivePrompt.Surface != "card" || !got.ActivePrompt.OpenedRepoTracked {
		t.Errorf("unexpected ActivePrompt round-trip: %+v", got.ActivePrompt)
	}
	if got.BaselineAgents == nil || *got.BaselineAgents != baseline {
		t.Errorf("expected BaselineAgents=%d to round-trip, got %+v", baseline, got.BaselineAgents)
	}

	// Clearing ActivePrompt back to nil must persist as SQL NULL, not a
	// stale JSON value.
	got.ActivePrompt = nil
	if err := repo.Save(ctx, got); err != nil {
		t.Fatalf("save clearing active prompt: %v", err)
	}
	cleared, err := repo.GetOrCreate(ctx, company.ID, userID)
	if err != nil {
		t.Fatalf("re-fetch after clearing: %v", err)
	}
	if cleared.ActivePrompt != nil {
		t.Errorf("expected ActivePrompt to be cleared to nil, got %+v", cleared.ActivePrompt)
	}
}

func TestStarNagStateRepository_ScopedByCompanyID(t *testing.T) {
	db := setupDB(t)
	companies := NewCompanyRepository(db)
	repo := NewStarNagStateRepository(db)
	ctx := context.Background()

	companyA, _ := domain.NewCompany("11111111-1111-1111-1111-111111111111", "Company A", nil)
	companyB, _ := domain.NewCompany("55555555-5555-5555-5555-555555555555", "Company B", nil)
	if _, err := companies.Create(ctx, companyA); err != nil {
		t.Fatalf("create company A: %v", err)
	}
	if _, err := companies.Create(ctx, companyB); err != nil {
		t.Fatalf("create company B: %v", err)
	}

	userID := "66666666-6666-6666-6666-666666666666"
	if _, err := repo.GetOrCreate(ctx, companyA.ID, userID); err != nil {
		t.Fatalf("get or create under company A: %v", err)
	}

	_, found, err := repo.get(ctx, companyB.ID, userID)
	if err != nil {
		t.Fatalf("get scoped to company B: %v", err)
	}
	if found {
		t.Error("expected a row belonging to a different company to resolve as not-found")
	}
}

// ---------------------------------------------------------------------
// CompanyEmailDomainRepository
// ---------------------------------------------------------------------

func TestCompanyEmailDomainRepository_AddListResolveRemove_RoundTrip(t *testing.T) {
	db := setupDB(t)
	companies := NewCompanyRepository(db)
	repo := NewCompanyEmailDomainRepository(db)
	ctx := context.Background()

	company, _ := domain.NewCompany("11111111-1111-1111-1111-111111111111", "Acme", nil)
	if _, err := companies.Create(ctx, company); err != nil {
		t.Fatalf("create company: %v", err)
	}

	if err := repo.Add(ctx, company.ID, "acme.com"); err != nil {
		t.Fatalf("add: %v", err)
	}
	// Re-adding the same pair is a no-op, not an error.
	if err := repo.Add(ctx, company.ID, "acme.com"); err != nil {
		t.Fatalf("re-add: %v", err)
	}

	companyID, found, err := repo.ResolveCompanyID(ctx, "acme.com")
	if err != nil || !found || companyID != company.ID {
		t.Fatalf("resolve: companyID=%q found=%v err=%v", companyID, found, err)
	}

	domains, err := repo.ListForCompany(ctx, company.ID)
	if err != nil {
		t.Fatalf("list for company: %v", err)
	}
	if len(domains) != 1 || domains[0] != "acme.com" {
		t.Fatalf("unexpected domains: %+v", domains)
	}

	if err := repo.Remove(ctx, "acme.com"); err != nil {
		t.Fatalf("remove: %v", err)
	}
	_, found, err = repo.ResolveCompanyID(ctx, "acme.com")
	if err != nil {
		t.Fatalf("resolve after remove: %v", err)
	}
	if found {
		t.Error("expected the domain to be gone after Remove")
	}
}

func TestCompanyEmailDomainRepository_ResolveCompanyID_NotFound(t *testing.T) {
	db := setupDB(t)
	repo := NewCompanyEmailDomainRepository(db)
	ctx := context.Background()

	_, found, err := repo.ResolveCompanyID(ctx, "unregistered.example.com")
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if found {
		t.Error("expected found=false for a domain that was never registered")
	}
}
