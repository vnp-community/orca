# TASK-PRF-01-06: Wire RBAC gate, validation, name-uniqueness, and audit into `UpdateDepartment`/`CreateDepartment`

**From Solution:** SOL-PRF-01
**Priority:** P1
**Service:** `tenant-service`
**File:** `backend-go/services/tenant-service/internal/usecase/update_department.go`
**Depends on:** TASK-PRF-01-01, TASK-PRF-01-03, TASK-PRF-01-04
**Status:** `[ ]` TODO

---

## Context

`UpdateDepartment`/`CreateDepartment` have no role gate, no settings
validation, and `CreateDepartment` has no name-uniqueness check. This wires
in `requireDepartmentAccess` (admin, or lead of the same department),
`ValidateDepartmentSettings`, a new `ExistsByName` repository method, and
audit emission on both.

## Changes to make

### `ports.go` — add `ExistsByName` to `DepartmentRepository`

```go
type DepartmentRepository interface {
	Create(ctx context.Context, department domain.Department) (domain.Department, error)
	Get(ctx context.Context, companyID, id string) (domain.Department, bool, error)
	List(ctx context.Context, companyID string) ([]domain.Department, error)
	Update(ctx context.Context, companyID, id string, patch domain.DepartmentSettingsPatch) (domain.Department, bool, error)
	// ExistsByName backs CreateDepartment's name-uniqueness check — scoped by
	// companyID, same isolation posture as every other DepartmentRepository
	// method (tenant-service.md §9). NEW.
	ExistsByName(ctx context.Context, companyID, name string) (bool, error)
}
```

Implement in `backend-go/services/tenant-service/internal/adapter/postgres/department_repository.go`:

```go
func (r *DepartmentRepository) ExistsByName(ctx context.Context, companyID, name string) (bool, error) {
	var exists bool
	err := r.pool.QueryRow(ctx, `
		SELECT EXISTS(SELECT 1 FROM tenant.departments WHERE company_id = $1 AND name = $2)
	`, companyID, name).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("postgres: check department name exists: %w", err)
	}
	return exists, nil
}
```

### `update_department.go` — RBAC gate + validation + audit

```go
type UpdateDepartment struct {
	departments  DepartmentRepository
	profiles     UserProfileRepository
	cache        ProfileCache
	invalidation CacheInvalidationPublisher
	opa          OPAClient      // NEW
	audit        AuditPublisher // NEW
}

func NewUpdateDepartment(departments DepartmentRepository, profiles UserProfileRepository, cache ProfileCache, invalidation CacheInvalidationPublisher, opa OPAClient, audit AuditPublisher) *UpdateDepartment {
	return &UpdateDepartment{departments: departments, profiles: profiles, cache: cache, invalidation: invalidation, opa: opa, audit: audit}
}

func (uc *UpdateDepartment) Execute(ctx context.Context, in UpdateDepartmentInput) (domain.Department, error) {
	companyID, err := tenant.RequireTenantID(ctx)
	if err != nil {
		return domain.Department{}, apperrors.New(apperrors.KindUnauthenticated, "TENANT_NO_TENANT", "no tenant in request context", err)
	}

	// sameDepartment: does the caller's own department match the one being
	// edited? A lead may edit only their own department — see
	// requireDepartmentAccess's doc comment.
	actorID, _ := tenant.UserID(ctx)
	actorProfile, _, err := uc.profiles.Get(ctx, companyID, actorID)
	if err != nil {
		return domain.Department{}, apperrors.New(apperrors.KindInternal, "TENANT_PROFILE_LOOKUP_FAILED", "failed to look up caller profile", err)
	}
	sameDepartment := actorProfile.DepartmentID == in.ID
	if err := requireDepartmentAccess(ctx, uc.opa, sameDepartment); err != nil {
		return domain.Department{}, err
	}

	if in.Patch.SettingsJSON != "" {
		var settings domain.Settings
		if err := json.Unmarshal([]byte(in.Patch.SettingsJSON), &settings); err != nil {
			return domain.Department{}, apperrors.New(apperrors.KindInvalidArgument, "TENANT_INVALID_SETTINGS_JSON", "malformed settings_json", err)
		}
		if err := domain.ValidateDepartmentSettings(settings); err != nil {
			return domain.Department{}, apperrors.New(apperrors.KindInvalidArgument, "TENANT_INVALID_DEPARTMENT_SETTINGS", err.Error(), err)
		}
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

	if uc.audit != nil {
		_ = uc.audit.PublishAuditEvent(ctx, companyID, actorID, "department.profile.updated", in.ID)
	}
	return dept, nil
}
```

Add `"encoding/json"` import.

### `create_department.go` — RBAC gate + name uniqueness + audit

```go
type CreateDepartment struct {
	companies   CompanyRepository
	departments DepartmentRepository
	opa         OPAClient      // NEW
	audit       AuditPublisher // NEW
}

func NewCreateDepartment(companies CompanyRepository, departments DepartmentRepository, opa OPAClient, audit AuditPublisher) *CreateDepartment {
	return &CreateDepartment{companies: companies, departments: departments, opa: opa, audit: audit}
}

func (uc *CreateDepartment) Execute(ctx context.Context, in CreateDepartmentInput) (domain.Department, error) {
	// sameDepartment is always false here — a lead can't create a department
	// that doesn't exist yet to be "their own", so only caller_role=="admin"
	// can pass this gate (matches BL-PRF-01's flow, which only shows Admin
	// as the actor for department creation).
	if err := requireDepartmentAccess(ctx, uc.opa, false); err != nil {
		return domain.Department{}, err
	}

	exists, err := uc.companies.Exists(ctx, in.CompanyID)
	if err != nil {
		return domain.Department{}, apperrors.New(apperrors.KindInternal, "TENANT_COMPANY_LOOKUP_FAILED", "failed to check company existence", err)
	}
	if !exists {
		return domain.Department{}, apperrors.New(apperrors.KindNotFound, "TENANT_COMPANY_NOT_FOUND", "company does not exist", nil)
	}

	nameTaken, err := uc.departments.ExistsByName(ctx, in.CompanyID, in.Name)
	if err != nil {
		return domain.Department{}, apperrors.New(apperrors.KindInternal, "TENANT_DEPARTMENT_NAME_LOOKUP_FAILED", "failed to check department name uniqueness", err)
	}
	if nameTaken {
		return domain.Department{}, apperrors.New(apperrors.KindInvalidArgument, "TENANT_DEPARTMENT_NAME_TAKEN", "a department with this name already exists", nil)
	}

	department, err := domain.NewDepartment(uuid.NewString(), in.CompanyID, in.Name, nil)
	if err != nil {
		return domain.Department{}, apperrors.New(apperrors.KindInvalidArgument, "TENANT_INVALID_DEPARTMENT", err.Error(), err)
	}

	created, err := uc.departments.Create(ctx, department)
	if err != nil {
		return domain.Department{}, apperrors.New(apperrors.KindInternal, "TENANT_CREATE_DEPARTMENT_FAILED", "failed to persist department", err)
	}

	if uc.audit != nil {
		actorID, _ := tenant.UserID(ctx)
		_ = uc.audit.PublishAuditEvent(ctx, in.CompanyID, actorID, "department.created", created.ID)
	}
	return created, nil
}
```

Add `"github.com/stablyai/orca-go/common/tenant"` import.

## Verify

```bash
cd /opt/repos/orca/backend-go
go vet ./services/tenant-service/internal/usecase/update_department.go ./services/tenant-service/internal/usecase/create_department.go
go build ./services/tenant-service/internal/adapter/postgres/...
```

Add `internal/adapter/postgres/department_repository_test.go` coverage for
`ExistsByName` (needs a real/test Postgres — follow this file's existing
test setup convention). Add usecase test cases per SOL-PRF-01's Test plan:
`sameDepartment` computed correctly from a fake `UserProfileRepository.Get`
(lead editing own dept passes, different dept denied, admin passes
regardless); `CreateDepartment` denies a non-admin unconditionally, and a
fake `ExistsByName` returning `true` short-circuits before `Create`. Full
build/test lands with TASK-PRF-01-08.
