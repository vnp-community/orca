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
}

// DepartmentRepository persists Department aggregates, always scoped by
// companyID — see tenant-service.md §9: "never inferred from a nested
// resource ID"; a department_id from another company must resolve as
// not-found, not "wrong company".
type DepartmentRepository interface {
	Create(ctx context.Context, department domain.Department) (domain.Department, error)
	Get(ctx context.Context, companyID, id string) (domain.Department, bool, error)
}

// UserProfileRepository persists the per-user profile-override row
// (tenant.user_profiles) — 1:1 with a user, logical FK to auth-service.
type UserProfileRepository interface {
	// Upsert creates or updates a user's profile row. Used by
	// SetUserDepartment, the only mutating usecase in tenant.proto's
	// current surface that touches this table — see README "Known gaps"
	// (there's no UpdateUserProfile RPC yet to set Settings directly).
	Upsert(ctx context.Context, profile domain.UserProfile) error
	Get(ctx context.Context, companyID, userID string) (domain.UserProfile, bool, error)
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
