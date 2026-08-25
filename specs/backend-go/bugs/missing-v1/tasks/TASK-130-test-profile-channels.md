# TASK-130: Tests for `profile.*` (tenant-service usecases/repository + wscompat channels)

**From Solution:** SOL-019
**Priority:** P1
**Service:** `tenant-service`, `api-gateway`
**File:** `internal/usecase/get_user_profile_test.go`, `list_departments_test.go`, `update_company_test.go`, `update_department_test.go`, `update_user_profile_test.go` (all new, `tenant-service`); `internal/adapter/postgres/*_test.go` (`tenant-service`); `services/api-gateway/internal/adapter/wscompat/channels_test.go`
**Depends on:** TASK-126, TASK-127, TASK-128, TASK-129
**Status:** `[partial]` — tenant-service usecase tests (`get_user_profile_test.go`, `list_departments_test.go`, `update_company_test.go`, `update_department_test.go`, `update_user_profile_test.go`) and all 8 api-gateway `channels_tenant_project_test.go` profile.* tests (6 per-channel + PropagateErrors + AttachIdentity) are implemented and green. `internal/adapter/postgres/*_test.go` (testcontainers-go, needs live Postgres) were NOT written/run — no Postgres available in this environment, per task instructions.

---

## Tests to add

### `tenant-service/internal/usecase/get_user_profile_test.go`

- `TestGetUserProfile_Found` — fake `UserProfileRepository` returns a
  profile; asserts passthrough.
- `TestGetUserProfile_NotFound` — fake returns `found=false`; asserts
  `apperrors.KindNotFound`.

### `tenant-service/internal/usecase/list_departments_test.go`

- `TestListDepartments_ScopedByCompany` — fake repository seeded with
  departments across two `company_id`s; asserts only the requested
  company's departments are returned.

### `tenant-service/internal/usecase/update_company_test.go`

- `TestUpdateCompany_AppliesPatch` — asserts the returned `Company`
  reflects the patch.
- `TestUpdateCompany_InvalidatesEveryCompanyUser` — fake
  `UserProfileRepository.ListUserIDsByCompany` returns 3 user IDs; fake
  `ProfileCache` records `Invalidate` calls; assert all 3 were invalidated
  (production-readiness-checklist-gated invariant from tenant-service.md
  §8, not optional coverage).
- `TestUpdateCompany_NotFound` — repository `Update` returns
  `found=false`; asserts `apperrors.KindNotFound`, and asserts
  `ListUserIDsByCompany`/`Invalidate` were never called (no invalidation
  on a failed write).

### `tenant-service/internal/usecase/update_department_test.go`

- Same three shapes as `update_company_test.go`, scoped to
  `ListUserIDsByDepartment` instead.
- `TestUpdateDepartment_CrossCompanyIsNotFound` — patch targets a
  department id that exists but under a different `company_id` (resolved
  from context); asserts not-found, not a leaked cross-tenant update
  (tenant-service.md §9).

### `tenant-service/internal/usecase/update_user_profile_test.go`

- `TestUpdateUserProfile_ClearDepartmentClearsField` — `ClearDepartment:
  true` on a profile with a set `DepartmentID`; asserts the persisted
  profile's `DepartmentID` is empty.
- `TestUpdateUserProfile_EmptyDepartmentIDWithoutClearIsNoChange` —
  `DepartmentID: ""`, `ClearDepartment: false`; asserts the existing
  `DepartmentID` is preserved (the no-change-vs-clear contract TASK-127's
  proto doc comment specifies).
- `TestUpdateUserProfile_SetSettingsFalseIsNoChange` — asserts existing
  `Settings` preserved when `SetSettings: false`.
- `TestUpdateUserProfile_InvalidatesOnlyTargetUser` — asserts
  `cache.Invalidate` called exactly once, for `UserID` only (narrowest
  invalidation scope of the three `Update*` usecases).

### `tenant-service/internal/adapter/postgres/*_test.go` (`testcontainers-go`)

- `TestCompanyRepository_UpdatePartialPatchLeavesOtherFieldsUnchanged` —
  update only `name`; assert `settings_json` unchanged.
- `TestDepartmentRepository_ListIsScopedByCompanyID` — a department
  belonging to a different `company_id` must never appear (mirrors the
  existing `TestDepartmentRepository_GetIsScopedByCompanyID` pattern).
- `TestDepartmentRepository_UpdateCrossCompanyReturnsNotFound`.
- `TestUserProfileRepository_ListUserIDsByDepartment` /
  `TestUserProfileRepository_ListUserIDsByCompany` — seed profiles across
  two departments/companies, assert exact membership of each list.

### `api-gateway/internal/adapter/wscompat/channels_test.go`

One test per channel (6 total), following `TestDevServerListChannel_Success`'s
shape — a `fakeTenantServiceClient` test double (embed the nil interface,
override only the methods this file's handlers call, same pattern as
`fakeInfraFleetClient`):

- `TestProfileGetResolvedChannel_Success`
- `TestProfileGetUserProfileChannel_Success`
- `TestProfileListDeptsChannel_Success`
- `TestProfileUpdateCompanyChannel_Success`
- `TestProfileUpdateDeptChannel_Success`
- `TestProfileUpdateUserChannel_Success` — assert `clearDepartment`/
  `settingsJson` args map onto the right request fields.

Plus:

- `TestProfileChannels_PropagateErrors` (table test over all 6) — fake
  client returns an error, asserts it passes through unwrapped.
- `TestProfileChannels_AttachIdentity` — asserts `AttachIdentity` ran, using
  `outgoingTenantUser(ctx)` (already defined in `channels_test.go`).

## Verify

```bash
cd /opt/repos/orca/backend-go/services/tenant-service
go test ./internal/usecase/... ./internal/adapter/postgres/... -count=1 -v

cd /opt/repos/orca/backend-go/services/api-gateway
go test ./internal/adapter/wscompat/... -run "TestProfile" -v -race

cd /opt/repos/orca/backend-go
go build ./... && go vet ./...
```
