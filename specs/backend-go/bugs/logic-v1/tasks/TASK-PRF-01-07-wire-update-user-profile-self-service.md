# TASK-PRF-01-07: Enforce self-service-only edits and settings validation in `UpdateUserProfile`

**From Solution:** SOL-PRF-01
**Priority:** P1
**Service:** `tenant-service`
**File:** `backend-go/services/tenant-service/internal/usecase/update_user_profile.go`
**Depends on:** TASK-PRF-01-01
**Status:** `[ ]` TODO

---

## Context

`UpdateUserProfile.Execute` accepts `in.UserID` from the request with no
check that it matches the caller — any authenticated user can currently edit
any other user's profile. BL-PRF-01 §4's flow shows only self-service edits
("User -> Settings -> My Profile -> Edit"), no admin-on-behalf-of path. This
task adds that check plus `ValidateUserSettings`. No OPA/RBAC port and no
audit call are added here — deliberately: this usecase is gated by identity
equality, not a role, and BL-PRF-01 §4 explicitly exempts personal-pref
updates from audit logging ("no audit log for personal prefs — privacy").

## Changes to make

In `backend-go/services/tenant-service/internal/usecase/update_user_profile.go`,
edit `Execute`:

```go
func (uc *UpdateUserProfile) Execute(ctx context.Context, in UpdateUserProfileInput) (domain.UserProfile, error) {
	companyID, err := tenant.RequireTenantID(ctx)
	if err != nil {
		return domain.UserProfile{}, apperrors.New(apperrors.KindUnauthenticated, "TENANT_NO_TENANT", "no tenant in request context", err)
	}
	actorID, ok := tenant.UserID(ctx)
	if !ok {
		return domain.UserProfile{}, apperrors.New(apperrors.KindUnauthenticated, "TENANT_NO_ACTOR", "no authenticated user in request context", nil)
	}
	if actorID != in.UserID {
		// Self-service only — no admin-on-behalf-of path exists in BL-PRF-01's
		// flow (§4 shows only "User -> Settings -> My Profile -> Edit");
		// adding one is a product decision, not this bug's scope.
		return domain.UserProfile{}, apperrors.New(apperrors.KindPermissionDenied, "TENANT_NOT_SELF", "users may only edit their own profile", nil)
	}
	if in.SetSettings {
		if err := domain.ValidateUserSettings(in.Settings); err != nil {
			return domain.UserProfile{}, apperrors.New(apperrors.KindInvalidArgument, "TENANT_INVALID_USER_SETTINGS", err.Error(), err)
		}
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
	// No PublishAuditEvent call here — deliberate, see this task's Context.
	return profile, nil
}
```

No constructor/struct change is needed — `UpdateUserProfile` already has no
`opa`/`audit` field and this task doesn't add one.

## Verify

```bash
cd /opt/repos/orca/backend-go
go build ./services/tenant-service/...
go test ./services/tenant-service/internal/usecase/... -run UpdateUserProfile -v
```

Add test cases per SOL-PRF-01's Test plan: `actorID != in.UserID` ->
`KindPermissionDenied`; a `security`/`integrations.githubOrg` key in
`in.Settings` -> `KindInvalidArgument`; confirm **no** `PublishAuditEvent`
call happens on a successful update (regression guard for the deliberate
audit exemption — this usecase has no `audit` field, so this is really
"confirm the field/dependency was never added").
