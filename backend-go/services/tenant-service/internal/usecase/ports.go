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
	// List returns every company row — genuinely cross-tenant (this table
	// IS the tenant root, see this interface's own doc comment), so callers
	// MUST admin-gate before exposing it. Added because a created company
	// was otherwise unreachable after the creating session ended (nothing
	// else lists tenant.companies) — see wscompat's profile.listCompanies.
	List(ctx context.Context) ([]domain.Company, error)
}

// CompanyEmailDomainRepository persists the email-domain -> company mapping
// (tenant.company_email_domains) — the multi-tenant SSO follow-up to
// CR-LOGIN-001. Every method takes/returns an already-normalized domain
// (domain.NormalizeEmailDomain) — normalization is the usecase layer's job,
// not this port's.
type CompanyEmailDomainRepository interface {
	// Add registers emailDomain as belonging to companyID. Re-adding the
	// same (companyID, emailDomain) pair is a no-op, not an error — the
	// usecase layer is responsible for rejecting an attempt to register a
	// domain already claimed by a DIFFERENT company (see
	// AddCompanyEmailDomain's doc comment); this method itself doesn't
	// enforce that, it just persists.
	Add(ctx context.Context, companyID, emailDomain string) error
	// Remove deletes one domain's mapping. Removing a domain that isn't
	// registered is a no-op, not an error.
	Remove(ctx context.Context, emailDomain string) error
	// ListForCompany returns every domain currently registered to
	// companyID, for the admin-facing "what domains does this company
	// own" view.
	ListForCompany(ctx context.Context, companyID string) ([]string, error)
	// ResolveCompanyID returns found=false (not an error) when no company
	// has registered emailDomain — the "this domain isn't set up for SSO
	// yet" case, which the caller (auth-service, via gRPC) surfaces as a
	// clear error rather than guessing a tenant.
	ResolveCompanyID(ctx context.Context, emailDomain string) (companyID string, found bool, err error)
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
	// GetOnboardingState/SetOnboardingState persist the onboarding wizard's
	// per-user progress (frontend/src/shared/types.ts's OnboardingState,
	// stored as an opaque JSON blob — see onboarding_state_json's migration
	// comment). found=false means "no state ever saved" (row missing OR
	// column NULL), the same "wizard not started" default the caller
	// already renders for a brand-new user. A dedicated partial-update
	// method rather than routing through Upsert: Upsert fully replaces
	// department_id/settings_json each call, which would silently clobber
	// them for a caller that only wants to touch onboarding state.
	GetOnboardingState(ctx context.Context, companyID, userID string) (stateJSON string, found bool, err error)
	SetOnboardingState(ctx context.Context, companyID, userID, stateJSON string) error
}

// TeamRepository persists Team aggregates and TeamMember rows, always
// scoped by companyID.
type TeamRepository interface {
	Create(ctx context.Context, team domain.Team) (domain.Team, error)
	Get(ctx context.Context, companyID, id string) (domain.Team, bool, error)
	// ListByCompany backs ListTeams — every team row scoped to companyID,
	// same not-found-not-wrong-company posture as Get (tenant-service.md §9).
	ListByCompany(ctx context.Context, companyID string) ([]domain.Team, error)
	AddMember(ctx context.Context, member domain.TeamMember) error
	// RemoveMember deletes one (team_id, user_id) row — backs
	// RemoveTeamMember. Returns found=false (not an error) when no such row
	// existed, so the usecase can treat "already removed" as an idempotent
	// no-op, matching DELETE semantics elsewhere in this codebase.
	RemoveMember(ctx context.Context, teamID, userID string) (bool, error)
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
