// Package usecase holds tenant-service's application services and the
// ports they need — defined here, implemented in internal/adapter/*, per
// the Dependency Inversion convention in
// specs/backend-go/architecture/03-clean-architecture-guidelines.md.
package usecase

import (
	"context"
	"time"

	"github.com/stablyai/orca-go/services/tenant-service/internal/domain"
)

// CompanyRepository persists Company aggregates — tenant-service's own
// database, per specs/backend-go/architecture/05-data-architecture.md.
// tenant.companies has no tenant_id column to filter by: this table IS the
// tenant root (tenant-service.md §5), so its methods take only the
// company's own id.
type CompanyRepository interface {
	Create(ctx context.Context, company domain.Company) (domain.Company, error)
	Get(ctx context.Context, id string) (domain.Company, bool, error)
	// Exists backs ValidateTenant — the logical-FK check every other
	// service calls to confirm a tenant_id it received is real
	// (tenant-service.md §3).
	Exists(ctx context.Context, id string) (bool, error)
	// Update applies patch's non-empty fields only. Returns found=false if
	// no company matches id.
	Update(ctx context.Context, id string, patch domain.CompanySettingsPatch) (domain.Company, bool, error)
}

// DepartmentRepository persists Department aggregates, always scoped by
// companyID — see tenant-service.md §9: "never inferred from a nested
// resource ID"; a department_id from another company must resolve as
// not-found, not "wrong company".
type DepartmentRepository interface {
	Create(ctx context.Context, department domain.Department) (domain.Department, error)
	Get(ctx context.Context, companyID, id string) (domain.Department, bool, error)
	// List returns every department scoped to companyID — flat, no
	// hierarchy (tenant-service.md's departments.parent_department_id
	// column is not surfaced by any RPC yet, see domain.Department's doc
	// comment).
	List(ctx context.Context, companyID string) ([]domain.Department, error)
	// Update applies patch's non-empty fields only, scoped by (companyID,
	// id) — a department_id from another company resolves as not-found,
	// same isolation rule as Get. Returns found=false if no match.
	Update(ctx context.Context, companyID, id string, patch domain.DepartmentSettingsPatch) (domain.Department, bool, error)
}

// UserProfileRepository persists the per-user profile-override row
// (tenant.user_profiles) — 1:1 with a user, logical FK to auth-service.
type UserProfileRepository interface {
	// Upsert creates or updates a user's profile row — used by
	// SetUserDepartment and (after this task) UpdateUserProfile.
	Upsert(ctx context.Context, profile domain.UserProfile) error
	Get(ctx context.Context, companyID, userID string) (domain.UserProfile, bool, error)
	// ListUserIDsByDepartment returns every user_id whose profile currently
	// has department_id = departmentID — UpdateDepartment's cache-
	// invalidation scope (tenant-service.md §8's per-mutation invalidation
	// table). Cheap indexed read against idx_user_profiles_department.
	ListUserIDsByDepartment(ctx context.Context, companyID, departmentID string) ([]string, error)
	// ListUserIDsByCompany returns every user_id in companyID —
	// UpdateCompany's (wider) cache-invalidation scope. Cheap indexed read
	// against idx_user_profiles_company.
	ListUserIDsByCompany(ctx context.Context, companyID string) ([]string, error)
}

// TeamRepository persists Team aggregates and TeamMember rows, always
// scoped by companyID.
type TeamRepository interface {
	Create(ctx context.Context, team domain.Team) (domain.Team, error)
	Get(ctx context.Context, companyID, id string) (domain.Team, bool, error)
	AddMember(ctx context.Context, member domain.TeamMember) error
	ListMembers(ctx context.Context, teamID string) ([]domain.TeamMember, error)
	// ListUserTeamLayers returns, for one user within one company, every
	// team they belong to with that team's Settings and the membership's
	// Priority — exactly the pre-fetched input domain.ResolveProfile's team
	// layer needs (tenant-service.md §4/§6).
	ListUserTeamLayers(ctx context.Context, companyID, userID string) ([]domain.TeamSettingsLayer, error)
}

// ProfileCache is the in-process LRU-with-TTL cache port for
// GetResolvedProfile — a usecase-layer concern per tenant-service.md §6,
// implemented by internal/adapter/cache, deliberately NOT baked into
// internal/adapter/postgres. Every mutating usecase that touches a Settings
// layer (SetUserDepartment, AddTeamMember) calls Invalidate for the exact
// user(s) it affects before returning success (tenant-service.md §8).
type ProfileCache interface {
	Get(ctx context.Context, userID string) (domain.ResolvedProfile, bool)
	Set(ctx context.Context, userID string, profile domain.ResolvedProfile, ttl time.Duration)
	Invalidate(ctx context.Context, userID string)
}

// CacheInvalidationPublisher broadcasts a profile-cache invalidation to
// every tenant-service replica over NATS — closes the horizontal-scaling
// gap ProfileCache's own doc comment used to flag as an accepted, TTL-bound
// staleness window (docs/execution-plan.md Epic F). Every mutating usecase
// that calls ProfileCache.Invalidate locally also calls this, best-effort,
// right after; internal/adapter/eventbus.Consumer is what makes every OTHER
// replica invalidate the same entry. A nil CacheInvalidationPublisher
// (wired in cmd/server/main.go when NATS is unreachable at startup) is
// valid — callers must nil-check before use, same convention as an absent
// optional dependency elsewhere in this codebase; the cache simply falls
// back to today's TTL-bounded staleness.
type CacheInvalidationPublisher interface {
	PublishProfileInvalidated(ctx context.Context, tenantID, userID string) error
}
