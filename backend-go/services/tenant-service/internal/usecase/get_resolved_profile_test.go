package usecase

import (
	"context"
	"testing"

	"github.com/stablyai/orca-go/services/tenant-service/internal/domain"
)

func TestGetResolvedProfile_RequiresTenantContext(t *testing.T) {
	uc := NewGetResolvedProfile(newFakeCompanyRepository(), newFakeDepartmentRepository(), newFakeUserProfileRepository(), newFakeTeamRepository())
	_, err := uc.Execute(context.Background(), "user-1")
	if err == nil {
		t.Fatal("expected an error when no tenant is in context")
	}
}

func TestGetResolvedProfile_CompanyNotFound(t *testing.T) {
	uc := NewGetResolvedProfile(newFakeCompanyRepository(), newFakeDepartmentRepository(), newFakeUserProfileRepository(), newFakeTeamRepository())
	ctx := withTenant(context.Background(), "company-missing")
	_, err := uc.Execute(ctx, "user-1")
	if err == nil {
		t.Fatal("expected an error when the company doesn't exist")
	}
}

// TestGetResolvedProfile_MergesAllFourLayersEndToEnd exercises the full
// fetch-then-merge path through the usecase layer against in-memory fakes:
// company defaults, a department override, two team overrides (priority
// tiebreak), and a user override — verifying the same layering
// domain.ResolveProfile's own tests check, but end-to-end through
// GetResolvedProfile.
func TestGetResolvedProfile_MergesAllFourLayersEndToEnd(t *testing.T) {
	companies := newFakeCompanyRepository()
	departments := newFakeDepartmentRepository()
	profiles := newFakeUserProfileRepository()
	teams := newFakeTeamRepository()

	company := mustCompany(t, "company-1", "Acme", domain.Settings{
		"agent":  domain.Settings{"model": "sonnet"},
		"editor": domain.Settings{"theme": "dark"},
	})
	_, _ = companies.Create(context.Background(), company)

	department := mustDepartment(t, "dept-1", "company-1", "Engineering", domain.Settings{
		"agent": domain.Settings{"model": "opus"},
	})
	_, _ = departments.Create(context.Background(), department)

	lowTeam := mustTeam(t, "team-low", "company-1", "Support", domain.Settings{
		"editor": domain.Settings{"theme": "light"},
	})
	highTeam := mustTeam(t, "team-high", "company-1", "Platform", domain.Settings{
		"editor": domain.Settings{"theme": "solarized"},
	})
	_, _ = teams.Create(context.Background(), lowTeam)
	_, _ = teams.Create(context.Background(), highTeam)
	_ = teams.AddMember(context.Background(), domain.TeamMember{TeamID: lowTeam.ID, UserID: "user-1", Priority: 1})
	_ = teams.AddMember(context.Background(), domain.TeamMember{TeamID: highTeam.ID, UserID: "user-1", Priority: 10})

	profile := mustUserProfile(t, "user-1", "company-1", "dept-1", domain.Settings{
		"agent": domain.Settings{"maxTokens": float64(8000)},
	})
	_ = profiles.Upsert(context.Background(), profile)

	uc := NewGetResolvedProfile(companies, departments, profiles, teams)
	ctx := withTenant(context.Background(), "company-1")

	got, err := uc.Execute(ctx, "user-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	agent := got.Settings["agent"].(domain.Settings)
	if agent["model"] != "opus" {
		t.Errorf("expected department's agent.model to win over company (no team/user override), got %v", agent["model"])
	}
	if agent["maxTokens"] != float64(8000) {
		t.Errorf("expected user's agent.maxTokens to survive as the only layer defining it, got %v", agent["maxTokens"])
	}

	editor := got.Settings["editor"].(domain.Settings)
	if editor["theme"] != "solarized" {
		t.Errorf("expected higher-priority team (team-high) to win editor.theme, got %v", editor["theme"])
	}
	if got.Sources["editor.theme"] != domain.TeamSource("team-high") {
		t.Errorf("expected editor.theme source=team:team-high, got %q", got.Sources["editor.theme"])
	}
}

func TestGetResolvedProfile_NoUserProfile_FallsBackToCompanyOnly(t *testing.T) {
	companies := newFakeCompanyRepository()
	company := mustCompany(t, "company-1", "Acme", domain.Settings{"agent": domain.Settings{"model": "sonnet"}})
	_, _ = companies.Create(context.Background(), company)

	uc := NewGetResolvedProfile(companies, newFakeDepartmentRepository(), newFakeUserProfileRepository(), newFakeTeamRepository())
	ctx := withTenant(context.Background(), "company-1")

	got, err := uc.Execute(ctx, "user-with-no-profile")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	agent := got.Settings["agent"].(domain.Settings)
	if agent["model"] != "sonnet" {
		t.Errorf("expected company-only resolution for a user with no profile row, got %v", agent["model"])
	}
}

func TestCachedGetResolvedProfile_CacheHitSkipsBaseUsecase(t *testing.T) {
	companies := newFakeCompanyRepository()
	company := mustCompany(t, "company-1", "Acme", domain.Settings{"agent": domain.Settings{"model": "sonnet"}})
	_, _ = companies.Create(context.Background(), company)

	base := NewGetResolvedProfile(companies, newFakeDepartmentRepository(), newFakeUserProfileRepository(), newFakeTeamRepository())
	cache := newFakeProfileCache()
	cached := NewCachedGetResolvedProfile(base, cache, DefaultProfileCacheTTL)

	ctx := withTenant(context.Background(), "company-1")

	first, err := cached.Execute(ctx, "user-1")
	if err != nil {
		t.Fatalf("unexpected error on first call: %v", err)
	}
	if cache.setCalls != 1 {
		t.Fatalf("expected the first call to populate the cache once, got %d sets", cache.setCalls)
	}

	// Now make the base usecase fail if called again — a cache hit must
	// short-circuit before ever reaching the repositories.
	companies.getErr = errFakeRepository

	second, err := cached.Execute(ctx, "user-1")
	if err != nil {
		t.Fatalf("expected cache hit to avoid the now-failing repository, got error: %v", err)
	}
	if second.Sources["agent.model"] != first.Sources["agent.model"] {
		t.Errorf("expected cached result to match the first resolution")
	}
}

func TestCachedGetResolvedProfile_PropagatesBaseError(t *testing.T) {
	base := NewGetResolvedProfile(newFakeCompanyRepository(), newFakeDepartmentRepository(), newFakeUserProfileRepository(), newFakeTeamRepository())
	cache := newFakeProfileCache()
	cached := NewCachedGetResolvedProfile(base, cache, DefaultProfileCacheTTL)

	_, err := cached.Execute(context.Background(), "user-1")
	if err == nil {
		t.Fatal("expected an error to propagate when the base usecase fails (no tenant in context)")
	}
	if cache.setCalls != 0 {
		t.Errorf("expected a failed resolution to never populate the cache, got %d sets", cache.setCalls)
	}
}

func mustCompany(t *testing.T, id, name string, settings domain.Settings) domain.Company {
	t.Helper()
	c, err := domain.NewCompany(id, name, settings)
	if err != nil {
		t.Fatalf("building company: %v", err)
	}
	return c
}

func mustDepartment(t *testing.T, id, companyID, name string, settings domain.Settings) domain.Department {
	t.Helper()
	d, err := domain.NewDepartment(id, companyID, name, settings)
	if err != nil {
		t.Fatalf("building department: %v", err)
	}
	return d
}

func mustTeam(t *testing.T, id, companyID, name string, settings domain.Settings) domain.Team {
	t.Helper()
	team, err := domain.NewTeam(id, companyID, name, settings)
	if err != nil {
		t.Fatalf("building team: %v", err)
	}
	return team
}

func mustUserProfile(t *testing.T, userID, companyID, departmentID string, settings domain.Settings) domain.UserProfile {
	t.Helper()
	p, err := domain.NewUserProfile(userID, companyID, departmentID, settings)
	if err != nil {
		t.Fatalf("building user profile: %v", err)
	}
	return p
}
