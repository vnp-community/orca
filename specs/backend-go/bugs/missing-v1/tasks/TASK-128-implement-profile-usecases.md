# TASK-128: Implement `tenant-service` domain/usecase/repository/grpc layers for the 5 new profile RPCs

**From Solution:** SOL-019
**Priority:** P1
**Service:** `tenant-service`
**File:** `internal/domain/company.go`, `internal/domain/department.go`, `internal/usecase/ports.go`, `internal/usecase/get_user_profile.go` (new), `internal/usecase/list_departments.go` (new), `internal/usecase/update_company.go` (new), `internal/usecase/update_department.go` (new), `internal/usecase/update_user_profile.go` (new), `internal/adapter/postgres/company_repository.go`, `internal/adapter/postgres/department_repository.go`, `internal/adapter/postgres/user_profile_repository.go`, `internal/adapter/grpc/server.go`, `cmd/server/main.go`
**Depends on:** TASK-127
**Status:** `[ ]` TODO

---

## Context

Three of the five new RPCs need a new repository method; `GetUserProfile`
needs none — `UserProfileRepository.Get(ctx, companyID, userID)` already
exists (added for `SetUserDepartment`'s internal use).

**Cache invalidation is the correctness-critical part of `UpdateCompany`/
`UpdateDepartment`/`UpdateUserProfile`**, per `tenant-service.md` §8: any
mutation must invalidate every cached `ResolvedProfile` for transitively
affected users before the write's RPC returns success. Follow
`SetUserDepartment`'s exact invalidation shape (`internal/usecase/set_user_department.go`):
`uc.cache.Invalidate` (nil-checked) then best-effort
`uc.invalidation.PublishProfileInvalidated` (nil-checked).

## Changes to make

### Step 1 — `internal/domain/company.go`: add `CompanySettingsPatch`

Append:

```go
// CompanySettingsPatch carries UpdateCompany's field-mask semantics: an
// empty string means "leave unchanged" — mirrors project-service's
// ProjectUpdatePatch convention (project-service/internal/domain/project.go).
type CompanySettingsPatch struct {
	Name         string
	SettingsJSON string // "" = no change; parsed to Settings by the usecase
}
```

### Step 2 — `internal/domain/department.go`: add `DepartmentSettingsPatch`

Append:

```go
// DepartmentSettingsPatch carries UpdateDepartment's field-mask semantics —
// same "" = no change convention as CompanySettingsPatch.
type DepartmentSettingsPatch struct {
	Name         string
	SettingsJSON string
}
```

### Step 3 — `internal/usecase/ports.go`: extend the three repository ports

Find:

```go
type CompanyRepository interface {
	Create(ctx context.Context, company domain.Company) (domain.Company, error)
	Get(ctx context.Context, id string) (domain.Company, bool, error)
	// Exists backs ValidateTenant — the logical-FK check every other
	// service calls to confirm a tenant_id it received is real
	// (tenant-service.md §3).
	Exists(ctx context.Context, id string) (bool, error)
}
```

Replace with:

```go
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
```

Find:

```go
type DepartmentRepository interface {
	Create(ctx context.Context, department domain.Department) (domain.Department, error)
	Get(ctx context.Context, companyID, id string) (domain.Department, bool, error)
}
```

Replace with:

```go
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
```

Find:

```go
type UserProfileRepository interface {
	// Upsert creates or updates a user's profile row. Used by
	// SetUserDepartment, the only mutating usecase in tenant.proto's
	// current surface that touches this table — see README "Known gaps"
	// (there's no UpdateUserProfile RPC yet to set Settings directly).
	Upsert(ctx context.Context, profile domain.UserProfile) error
	Get(ctx context.Context, companyID, userID string) (domain.UserProfile, bool, error)
}
```

Replace with:

```go
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
```

### Step 4 — `internal/usecase/get_user_profile.go` (new)

```go
package usecase

import (
	"context"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/services/tenant-service/internal/domain"
)

// GetUserProfileInput mirrors GetUserProfileRequest 1:1.
type GetUserProfileInput struct {
	UserID string
}

// GetUserProfile is thin: UserProfileRepository.Get already exists and
// already does exactly this lookup (added for SetUserDepartment's internal
// use, never exposed as its own RPC before now).
type GetUserProfile struct {
	profiles UserProfileRepository
}

func NewGetUserProfile(profiles UserProfileRepository) *GetUserProfile {
	return &GetUserProfile{profiles: profiles}
}

func (uc *GetUserProfile) Execute(ctx context.Context, in GetUserProfileInput) (domain.UserProfile, error) {
	companyID, err := tenantRequireTenantID(ctx)
	if err != nil {
		return domain.UserProfile{}, err
	}
	profile, found, err := uc.profiles.Get(ctx, companyID, in.UserID)
	if err != nil {
		return domain.UserProfile{}, apperrors.New(apperrors.KindInternal, "TENANT_PROFILE_LOOKUP_FAILED", "failed to look up user profile", err)
	}
	if !found {
		return domain.UserProfile{}, apperrors.New(apperrors.KindNotFound, "TENANT_PROFILE_NOT_FOUND", "user profile does not exist", nil)
	}
	return profile, nil
}
```

### Step 5 — `internal/usecase/list_departments.go` (new)

```go
package usecase

import (
	"context"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/services/tenant-service/internal/domain"
)

// ListDepartmentsInput mirrors ListDepartmentsRequest 1:1.
type ListDepartmentsInput struct {
	CompanyID string
}

type ListDepartments struct {
	departments DepartmentRepository
}

func NewListDepartments(departments DepartmentRepository) *ListDepartments {
	return &ListDepartments{departments: departments}
}

func (uc *ListDepartments) Execute(ctx context.Context, in ListDepartmentsInput) ([]domain.Department, error) {
	depts, err := uc.departments.List(ctx, in.CompanyID)
	if err != nil {
		return nil, apperrors.New(apperrors.KindInternal, "TENANT_LIST_DEPARTMENTS_FAILED", "failed to list departments", err)
	}
	return depts, nil
}
```

### Step 6 — `internal/usecase/update_company.go` (new)

```go
package usecase

import (
	"context"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/services/tenant-service/internal/domain"
)

// UpdateCompanyInput mirrors UpdateCompanyRequest 1:1.
type UpdateCompanyInput struct {
	ID      string
	Patch   domain.CompanySettingsPatch
}

// UpdateCompany applies patch and invalidates every affected user's cached
// ResolvedProfile — the company layer is the base of every merge, so its
// scope is EVERY user in the company (tenant-service.md §8).
type UpdateCompany struct {
	companies    CompanyRepository
	profiles     UserProfileRepository
	cache        ProfileCache
	invalidation CacheInvalidationPublisher
}

func NewUpdateCompany(companies CompanyRepository, profiles UserProfileRepository, cache ProfileCache, invalidation CacheInvalidationPublisher) *UpdateCompany {
	return &UpdateCompany{companies: companies, profiles: profiles, cache: cache, invalidation: invalidation}
}

func (uc *UpdateCompany) Execute(ctx context.Context, in UpdateCompanyInput) (domain.Company, error) {
	company, found, err := uc.companies.Update(ctx, in.ID, in.Patch)
	if err != nil {
		return domain.Company{}, apperrors.New(apperrors.KindInternal, "TENANT_UPDATE_COMPANY_FAILED", "failed to update company", err)
	}
	if !found {
		return domain.Company{}, apperrors.New(apperrors.KindNotFound, "TENANT_COMPANY_NOT_FOUND", "company does not exist", nil)
	}

	userIDs, err := uc.profiles.ListUserIDsByCompany(ctx, in.ID)
	if err != nil {
		return domain.Company{}, apperrors.New(apperrors.KindInternal, "TENANT_LIST_COMPANY_USERS_FAILED", "failed to resolve invalidation scope", err)
	}
	for _, uid := range userIDs {
		if uc.cache != nil {
			uc.cache.Invalidate(ctx, uid)
		}
		if uc.invalidation != nil {
			_ = uc.invalidation.PublishProfileInvalidated(ctx, in.ID, uid)
		}
	}
	return company, nil
}
```

### Step 7 — `internal/usecase/update_department.go` (new)

```go
package usecase

import (
	"context"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/tenant-service/internal/domain"
)

// UpdateDepartmentInput mirrors UpdateDepartmentRequest 1:1. CompanyID comes
// from request context (tenant.RequireTenantID), same convention as
// SetUserDepartment — UpdateDepartmentRequest has no company_id field.
type UpdateDepartmentInput struct {
	ID    string
	Patch domain.DepartmentSettingsPatch
}

// UpdateDepartment applies patch and invalidates every user IN THAT
// DEPARTMENT's cached ResolvedProfile — narrower scope than UpdateCompany,
// per tenant-service.md §8's per-mutation invalidation-scope table.
type UpdateDepartment struct {
	departments  DepartmentRepository
	profiles     UserProfileRepository
	cache        ProfileCache
	invalidation CacheInvalidationPublisher
}

func NewUpdateDepartment(departments DepartmentRepository, profiles UserProfileRepository, cache ProfileCache, invalidation CacheInvalidationPublisher) *UpdateDepartment {
	return &UpdateDepartment{departments: departments, profiles: profiles, cache: cache, invalidation: invalidation}
}

func (uc *UpdateDepartment) Execute(ctx context.Context, in UpdateDepartmentInput) (domain.Department, error) {
	companyID, err := tenant.RequireTenantID(ctx)
	if err != nil {
		return domain.Department{}, apperrors.New(apperrors.KindUnauthenticated, "TENANT_NO_TENANT", "no tenant in request context", err)
	}

	dept, found, err := uc.departments.Update(ctx, companyID, in.ID, in.Patch)
	if err != nil {
		return domain.Department{}, apperrors.New(apperrors.KindInternal, "TENANT_UPDATE_DEPARTMENT_FAILED", "failed to update department", err)
	}
	if !found {
		return domain.Department{}, apperrors.New(apperrors.KindNotFound, "TENANT_DEPARTMENT_NOT_FOUND", "department does not exist", nil)
	}

	userIDs, err := uc.profiles.ListUserIDsByDepartment(ctx, companyID, in.ID)
	if err != nil {
		return domain.Department{}, apperrors.New(apperrors.KindInternal, "TENANT_LIST_DEPARTMENT_USERS_FAILED", "failed to resolve invalidation scope", err)
	}
	for _, uid := range userIDs {
		if uc.cache != nil {
			uc.cache.Invalidate(ctx, uid)
		}
		if uc.invalidation != nil {
			_ = uc.invalidation.PublishProfileInvalidated(ctx, companyID, uid)
		}
	}
	return dept, nil
}
```

### Step 8 — `internal/usecase/update_user_profile.go` (new)

```go
package usecase

import (
	"context"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/tenant-service/internal/domain"
)

// UpdateUserProfileInput mirrors UpdateUserProfileRequest 1:1 — see that
// message's doc comment (TASK-127) for the clear_department/department_id
// no-change-vs-clear contract.
type UpdateUserProfileInput struct {
	UserID          string
	DepartmentID    string
	ClearDepartment bool
	Settings        domain.Settings
	SetSettings     bool // false = settings_json was empty ("" = no change)
}

// UpdateUserProfile is the "expose Upsert directly" RPC —
// UserProfileRepository.Upsert already existed for SetUserDepartment's
// internal use; this is the case the port's own former doc comment
// ("no UpdateUserProfile RPC yet") flagged as missing. Invalidation scope
// is just the one user being updated — no extra lookup needed.
type UpdateUserProfile struct {
	profiles     UserProfileRepository
	cache        ProfileCache
	invalidation CacheInvalidationPublisher
}

func NewUpdateUserProfile(profiles UserProfileRepository, cache ProfileCache, invalidation CacheInvalidationPublisher) *UpdateUserProfile {
	return &UpdateUserProfile{profiles: profiles, cache: cache, invalidation: invalidation}
}

func (uc *UpdateUserProfile) Execute(ctx context.Context, in UpdateUserProfileInput) (domain.UserProfile, error) {
	companyID, err := tenant.RequireTenantID(ctx)
	if err != nil {
		return domain.UserProfile{}, apperrors.New(apperrors.KindUnauthenticated, "TENANT_NO_TENANT", "no tenant in request context", err)
	}

	existing, _, err := uc.profiles.Get(ctx, companyID, in.UserID)
	if err != nil {
		return domain.UserProfile{}, apperrors.New(apperrors.KindInternal, "TENANT_PROFILE_LOOKUP_FAILED", "failed to look up user profile", err)
	}

	departmentID := existing.DepartmentID
	if in.ClearDepartment {
		departmentID = ""
	} else if in.DepartmentID != "" {
		departmentID = in.DepartmentID
	}
	settings := existing.Settings
	if in.SetSettings {
		settings = in.Settings
	}

	profile, err := domain.NewUserProfile(in.UserID, companyID, departmentID, settings)
	if err != nil {
		return domain.UserProfile{}, apperrors.New(apperrors.KindInvalidArgument, "TENANT_INVALID_PROFILE", err.Error(), err)
	}
	if err := uc.profiles.Upsert(ctx, profile); err != nil {
		return domain.UserProfile{}, apperrors.New(apperrors.KindInternal, "TENANT_UPDATE_PROFILE_FAILED", "failed to persist user profile", err)
	}

	if uc.cache != nil {
		uc.cache.Invalidate(ctx, in.UserID)
	}
	if uc.invalidation != nil {
		_ = uc.invalidation.PublishProfileInvalidated(ctx, companyID, in.UserID)
	}
	return profile, nil
}
```

### Step 9 — small shared helper: `internal/usecase/tenant_context.go` (new)

`GetUserProfile` above uses `tenantRequireTenantID` — add this tiny wrapper
so that file doesn't need to import `common/tenant` under a different name
than the rest of the package already uses inline (`tenant.RequireTenantID`).
Simpler: skip the wrapper and just import `"github.com/stablyai/orca-go/common/tenant"`
directly in `get_user_profile.go` and call `tenant.RequireTenantID(ctx)` —
**do this instead**, matching every other usecase file's convention. (No new
file needed — this step exists only to flag that `get_user_profile.go`
above must import `common/tenant` and call `tenant.RequireTenantID`, not a
package-local `tenantRequireTenantID` helper.)

Fix `get_user_profile.go`'s import block and body to:

```go
import (
	"context"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/tenant-service/internal/domain"
)
```

and its first line to `companyID, err := tenant.RequireTenantID(ctx)`.

### Step 10 — `internal/adapter/postgres/company_repository.go`: add `Update`

Append:

```go
// Update applies patch's non-empty fields via COALESCE(NULLIF($n, ''), col)
// — mirrors project-service.Repository.UpdateProject's convention.
// Returns found=false (no error) if id doesn't match any row.
func (r *CompanyRepository) Update(ctx context.Context, id string, patch domain.CompanySettingsPatch) (domain.Company, bool, error) {
	var settingsArg any
	if patch.SettingsJSON != "" {
		settings, err := unmarshalSettings(patch.SettingsJSON)
		if err != nil {
			return domain.Company{}, false, fmt.Errorf("postgres: unmarshal company settings patch: %w", err)
		}
		marshaled, err := marshalSettings(settings)
		if err != nil {
			return domain.Company{}, false, fmt.Errorf("postgres: marshal company settings patch: %w", err)
		}
		settingsArg = marshaled
	}

	row := r.pool.QueryRow(ctx, `
		UPDATE tenant.companies
		SET name          = COALESCE(NULLIF($2, ''), name),
		    settings_json = COALESCE($3, settings_json)
		WHERE id = $1
		RETURNING id, name, settings_json
	`, id, patch.Name, settingsArg)

	var c domain.Company
	var settingsJSON string
	if err := row.Scan(&c.ID, &c.Name, &settingsJSON); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.Company{}, false, nil
		}
		return domain.Company{}, false, fmt.Errorf("postgres: update company: %w", err)
	}
	settings, err := unmarshalSettings(settingsJSON)
	if err != nil {
		return domain.Company{}, false, fmt.Errorf("postgres: unmarshal company settings: %w", err)
	}
	c.Settings = settings
	return c, true, nil
}
```

### Step 11 — `internal/adapter/postgres/department_repository.go`: add `List`/`Update`

Append:

```go
func (r *DepartmentRepository) List(ctx context.Context, companyID string) ([]domain.Department, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, company_id, name, settings_json
		FROM tenant.departments
		WHERE company_id = $1
		ORDER BY id
	`, companyID)
	if err != nil {
		return nil, fmt.Errorf("postgres: query departments: %w", err)
	}
	defer rows.Close()

	var out []domain.Department
	for rows.Next() {
		var d domain.Department
		var settingsJSON string
		if err := rows.Scan(&d.ID, &d.CompanyID, &d.Name, &settingsJSON); err != nil {
			return nil, fmt.Errorf("postgres: scan department row: %w", err)
		}
		settings, err := unmarshalSettings(settingsJSON)
		if err != nil {
			return nil, fmt.Errorf("postgres: unmarshal department settings: %w", err)
		}
		d.Settings = settings
		out = append(out, d)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("postgres: iterate department rows: %w", err)
	}
	return out, nil
}

// Update applies patch's non-empty fields, scoped by (companyID, id) — a
// department belonging to a different company resolves as not-found, same
// isolation rule as Get.
func (r *DepartmentRepository) Update(ctx context.Context, companyID, id string, patch domain.DepartmentSettingsPatch) (domain.Department, bool, error) {
	var settingsArg any
	if patch.SettingsJSON != "" {
		settings, err := unmarshalSettings(patch.SettingsJSON)
		if err != nil {
			return domain.Department{}, false, fmt.Errorf("postgres: unmarshal department settings patch: %w", err)
		}
		marshaled, err := marshalSettings(settings)
		if err != nil {
			return domain.Department{}, false, fmt.Errorf("postgres: marshal department settings patch: %w", err)
		}
		settingsArg = marshaled
	}

	row := r.pool.QueryRow(ctx, `
		UPDATE tenant.departments
		SET name          = COALESCE(NULLIF($3, ''), name),
		    settings_json = COALESCE($4, settings_json)
		WHERE company_id = $1 AND id = $2
		RETURNING id, company_id, name, settings_json
	`, companyID, id, patch.Name, settingsArg)

	var d domain.Department
	var settingsJSON string
	if err := row.Scan(&d.ID, &d.CompanyID, &d.Name, &settingsJSON); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.Department{}, false, nil
		}
		return domain.Department{}, false, fmt.Errorf("postgres: update department: %w", err)
	}
	settings, err := unmarshalSettings(settingsJSON)
	if err != nil {
		return domain.Department{}, false, fmt.Errorf("postgres: unmarshal department settings: %w", err)
	}
	d.Settings = settings
	return d, true, nil
}
```

### Step 12 — `internal/adapter/postgres/user_profile_repository.go`: add the two `ListUserIDsBy*` methods

Append:

```go
func (r *UserProfileRepository) ListUserIDsByDepartment(ctx context.Context, companyID, departmentID string) ([]string, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT user_id FROM tenant.user_profiles
		WHERE company_id = $1 AND department_id = $2
	`, companyID, departmentID)
	if err != nil {
		return nil, fmt.Errorf("postgres: query user ids by department: %w", err)
	}
	defer rows.Close()

	var out []string
	for rows.Next() {
		var uid string
		if err := rows.Scan(&uid); err != nil {
			return nil, fmt.Errorf("postgres: scan user id row: %w", err)
		}
		out = append(out, uid)
	}
	return out, rows.Err()
}

func (r *UserProfileRepository) ListUserIDsByCompany(ctx context.Context, companyID string) ([]string, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT user_id FROM tenant.user_profiles WHERE company_id = $1
	`, companyID)
	if err != nil {
		return nil, fmt.Errorf("postgres: query user ids by company: %w", err)
	}
	defer rows.Close()

	var out []string
	for rows.Next() {
		var uid string
		if err := rows.Scan(&uid); err != nil {
			return nil, fmt.Errorf("postgres: scan user id row: %w", err)
		}
		out = append(out, uid)
	}
	return out, rows.Err()
}
```

### Step 13 — `internal/adapter/grpc/server.go`: register the 5 new RPC handlers

Add 5 fields to `Server`/`Deps`/`New` following `createTeam`'s exact
pattern, and 5 handler methods:

```go
// Server struct: add
	getUserProfile    *usecase.GetUserProfile
	listDepartments   *usecase.ListDepartments
	updateCompany     *usecase.UpdateCompany
	updateDepartment  *usecase.UpdateDepartment
	updateUserProfile *usecase.UpdateUserProfile
```

Add matching params to `New(...)` and its return struct literal (same
mechanical pattern as the 8 existing params).

```go
func (s *Server) GetUserProfile(ctx context.Context, req *tenantv1.GetUserProfileRequest) (*tenantv1.GetUserProfileResponse, error) {
	profile, err := s.getUserProfile.Execute(ctx, usecase.GetUserProfileInput{UserID: req.GetUserId()})
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	proto, err := toProtoUserProfile(profile)
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	return &tenantv1.GetUserProfileResponse{Profile: proto}, nil
}

func (s *Server) ListDepartments(ctx context.Context, req *tenantv1.ListDepartmentsRequest) (*tenantv1.ListDepartmentsResponse, error) {
	depts, err := s.listDepartments.Execute(ctx, usecase.ListDepartmentsInput{CompanyID: req.GetCompanyId()})
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	out := make([]*tenantv1.Department, 0, len(depts))
	for _, d := range depts {
		proto, err := toProtoDepartment(d)
		if err != nil {
			return nil, apperrors.ToGRPCStatus(err)
		}
		out = append(out, proto)
	}
	return &tenantv1.ListDepartmentsResponse{Departments: out}, nil
}

func (s *Server) UpdateCompany(ctx context.Context, req *tenantv1.UpdateCompanyRequest) (*tenantv1.UpdateCompanyResponse, error) {
	company, err := s.updateCompany.Execute(ctx, usecase.UpdateCompanyInput{
		ID: req.GetId(),
		Patch: domain.CompanySettingsPatch{
			Name:         req.GetName(),
			SettingsJSON: req.GetSettingsJson(),
		},
	})
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	proto, err := toProtoCompany(company)
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	return &tenantv1.UpdateCompanyResponse{Company: proto}, nil
}

func (s *Server) UpdateDepartment(ctx context.Context, req *tenantv1.UpdateDepartmentRequest) (*tenantv1.UpdateDepartmentResponse, error) {
	dept, err := s.updateDepartment.Execute(ctx, usecase.UpdateDepartmentInput{
		ID: req.GetId(),
		Patch: domain.DepartmentSettingsPatch{
			Name:         req.GetName(),
			SettingsJSON: req.GetSettingsJson(),
		},
	})
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	proto, err := toProtoDepartment(dept)
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	return &tenantv1.UpdateDepartmentResponse{Department: proto}, nil
}

func (s *Server) UpdateUserProfile(ctx context.Context, req *tenantv1.UpdateUserProfileRequest) (*tenantv1.UpdateUserProfileResponse, error) {
	in := usecase.UpdateUserProfileInput{
		UserID:          req.GetUserId(),
		DepartmentID:    req.GetDepartmentId(),
		ClearDepartment: req.GetClearDepartment(),
	}
	if req.GetSettingsJson() != "" {
		settings, err := unmarshalSettings(req.GetSettingsJson())
		if err != nil {
			return nil, apperrors.ToGRPCStatus(apperrors.New(apperrors.KindInvalidArgument, "TENANT_INVALID_PROFILE_SETTINGS", "settings_json is not valid JSON", err))
		}
		in.Settings = settings
		in.SetSettings = true
	}
	profile, err := s.updateUserProfile.Execute(ctx, in)
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	proto, err := toProtoUserProfile(profile)
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	return &tenantv1.UpdateUserProfileResponse{Profile: proto}, nil
}

func toProtoUserProfile(p domain.UserProfile) (*tenantv1.UserProfile, error) {
	settingsJSON, err := marshalSettings(p.Settings)
	if err != nil {
		return nil, apperrors.New(apperrors.KindInternal, "TENANT_MARSHAL_PROFILE_FAILED", "failed to marshal user profile settings", err)
	}
	return &tenantv1.UserProfile{
		UserId: p.UserID, CompanyId: p.CompanyID, DepartmentId: p.DepartmentID, SettingsJson: settingsJSON,
	}, nil
}
```

### Step 14 — `cmd/server/main.go`: wire the 5 new usecases

Find where existing usecases are constructed (near
`setUserDepartmentUC := usecase.NewSetUserDepartment(...)`) and add:

```go
getUserProfileUC := usecase.NewGetUserProfile(profiles)
listDepartmentsUC := usecase.NewListDepartments(departments)
updateCompanyUC := usecase.NewUpdateCompany(companies, profiles, profileCache, invalidationPublisher)
updateDepartmentUC := usecase.NewUpdateDepartment(departments, profiles, profileCache, invalidationPublisher)
updateUserProfileUC := usecase.NewUpdateUserProfile(profiles, profileCache, invalidationPublisher)
```

(`companies`, `departments`, `profiles` are the existing repository
variables already in scope; `profileCache`/`invalidationPublisher` are the
existing cache/eventbus variables `setUserDepartmentUC` already uses.)

Add the 5 new params to the `tenantgrpc.New(...)` call site, in the same
order as `Server`'s new fields (Step 13).

## Verify

```bash
cd /opt/repos/orca/backend-go/services/tenant-service
go build ./... && go vet ./...
```

Expected: clean build. `cmd/server/main.go` build failure until Step 14 is
applied is expected mid-task, same caveat as TASK-005 in `api-v1`'s task set.
