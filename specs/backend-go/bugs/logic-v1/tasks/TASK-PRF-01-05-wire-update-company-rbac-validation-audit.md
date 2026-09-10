# TASK-PRF-01-05: Wire RBAC gate, settings validation, and audit emission into `UpdateCompany`

**From Solution:** SOL-PRF-01
**Priority:** P1
**Service:** `tenant-service`
**File:** `backend-go/services/tenant-service/internal/usecase/update_company.go`
**Depends on:** TASK-PRF-01-01, TASK-PRF-01-03, TASK-PRF-01-04
**Status:** `[x]` DONE — UpdateCompany wired with requireCompanyAdmin + ValidateCompanySettings + audit emit; full test suite green

---

## Context

`UpdateCompany.Execute` currently persists any patch unconditionally — no
role check, no settings validation. This wires in the admin-only gate and
`ValidateCompanySettings` before the write, and an audit emission after it.

## Changes to make

In `backend-go/services/tenant-service/internal/usecase/update_company.go`,
add an `audit AuditPublisher` field/constructor param and edit `Execute`:

```go
type UpdateCompany struct {
	companies    CompanyRepository
	profiles     UserProfileRepository
	cache        ProfileCache
	invalidation CacheInvalidationPublisher
	opa          OPAClient      // NEW
	audit        AuditPublisher // NEW
}

func NewUpdateCompany(companies CompanyRepository, profiles UserProfileRepository, cache ProfileCache, invalidation CacheInvalidationPublisher, opa OPAClient, audit AuditPublisher) *UpdateCompany {
	return &UpdateCompany{companies: companies, profiles: profiles, cache: cache, invalidation: invalidation, opa: opa, audit: audit}
}

func (uc *UpdateCompany) Execute(ctx context.Context, in UpdateCompanyInput) (domain.Company, error) {
	if err := requireCompanyAdmin(ctx, uc.opa); err != nil {
		return domain.Company{}, err
	}
	if in.Patch.SettingsJSON != "" {
		var settings domain.Settings
		if err := json.Unmarshal([]byte(in.Patch.SettingsJSON), &settings); err != nil {
			return domain.Company{}, apperrors.New(apperrors.KindInvalidArgument, "TENANT_INVALID_SETTINGS_JSON", "malformed settings_json", err)
		}
		if err := domain.ValidateCompanySettings(settings); err != nil {
			return domain.Company{}, apperrors.New(apperrors.KindInvalidArgument, "TENANT_INVALID_COMPANY_SETTINGS", err.Error(), err)
		}
	}

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

	if uc.audit != nil {
		actorID, _ := tenant.UserID(ctx)
		_ = uc.audit.PublishAuditEvent(ctx, in.ID, actorID, "company.profile.updated", in.ID)
	}
	return company, nil
}
```

Add `"encoding/json"` and `"github.com/stablyai/orca-go/common/tenant"` to
the file's imports.

Update the `NewUpdateCompany(...)` call site in
`backend-go/services/tenant-service/cmd/server/main.go` — this is completed
in TASK-PRF-01-08 alongside the other usecase constructors; a standalone
`go build` of this package will fail until that task lands, `go vet` on this
file in isolation is still useful to confirm syntax.

## Verify

```bash
cd /opt/repos/orca/backend-go
go vet ./services/tenant-service/internal/usecase/update_company.go ./services/tenant-service/internal/usecase/ports.go
```

Update `update_company_test.go` per SOL-PRF-01's Test plan: fake `OPAClient`
returning `false` for `"lead"`/`"user"`/`""` roles denies without calling
`companies.Update`; `"admin"` proceeds; invalid settings JSON short-circuits
before/after the OPA call (assert one consistent ordering, matching this
task's code — OPA check runs first). Full package build/test lands with
TASK-PRF-01-08.
