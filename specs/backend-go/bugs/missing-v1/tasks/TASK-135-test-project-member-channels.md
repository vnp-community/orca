# TASK-135: Tests for `project.*` (project-service usecases/repository + wscompat channels)

**From Solution:** SOL-020
**Priority:** P1
**Service:** `project-service`, `api-gateway`
**File:** `internal/domain/membership_test.go`, `internal/usecase/list_members_test.go`, `remove_member_test.go`, `update_member_role_test.go` (all new, `project-service`); `internal/adapter/postgres/repository_test.go`; `services/api-gateway/internal/adapter/wscompat/channels_test.go`
**Depends on:** TASK-131, TASK-132, TASK-133, TASK-134
**Status:** `[partial]` — usecase/wscompat tests written and passing. Postgres integration tests (testcontainers) not written/run — no live Postgres in this environment; migration SQL written but unapplied. Worktree `agent-a9271c5b2d89347e7`, committed as `19b216531`.

---

## Tests to add

### `project-service/internal/domain/membership_test.go`

`TestAssertNotLastOwnerRemoval` (table test):

- last owner removed (`targetIsCurrentlyOwner: true, targetRoleAfter: "",
  currentOwnerCount: 1`) → `ErrProjectWouldBeOwnerless`
- last owner demoted (`targetRoleAfter: ProjectRoleMember, currentOwnerCount: 1`)
  → `ErrProjectWouldBeOwnerless`
- non-last owner removed/demoted (`currentOwnerCount: 2`) → nil
- removing a non-owner never errors regardless of owner count
  (`targetIsCurrentlyOwner: false`)

### `project-service/internal/usecase/remove_member_test.go`

- `TestRemoveMember_RejectsWhenWouldBeOwnerless` — fake repo:
  `CountOwners` returns 1, target's `GetMembership` returns
  `ProjectRoleOwner`; assert `apperrors.KindFailedPrecondition` and assert
  `RemoveMember` (the mutation) was **never called** — the ownerless guard
  must fire before any repository mutation.
- `TestRemoveMember_AllowsWhenNotLastOwner` — `CountOwners` returns 2;
  assert `RemoveMember` was called.
- `TestRemoveMember_DeniesNonOwnerActor` — fake OPA denies
  `projectActionOwnerOnly`; assert `apperrors.KindPermissionDenied` (or
  whatever kind `requireProjectAccess` maps a deny to — match its existing
  contract), and no repository calls made.
- `TestRemoveMember_MembershipNotFound` — target has no membership row;
  assert `apperrors.KindNotFound`.

### `project-service/internal/usecase/update_member_role_test.go`

- Same shapes as `remove_member_test.go`, targeting `AssertNotLastOwnerRemoval`'s
  demotion branch (`targetRoleAfter: in.Role`).
- `TestUpdateMemberRole_PromotionNeverBlockedByGuard` — target is currently
  `member`, promoted to `owner`; assert no owner-count check blocks this
  regardless of `CountOwners`'s value.
- `TestUpdateMemberRole_RejectsInvalidRole` — an unrecognized role string
  (decoded to `domain.ProjectRole("")` by `toDomainRole`); assert
  `apperrors.KindInvalidArgument`.

### `project-service/internal/usecase/list_members_test.go`

- `TestListMembers_AnyMemberCanList` — a `member`-role actor (not owner)
  succeeds.
- `TestListMembers_DeniesNonMember` — fake OPA denies
  `projectActionAnyMember`; assert denial.

### `project-service/internal/adapter/postgres/repository_test.go` (`testcontainers-go`)

- `TestRepository_CountOwnersReflectsConcurrentMutations` — seed 2 owners +
  1 member; remove one owner via `RemoveMember`; assert `CountOwners`
  reflects exactly 1 afterward — this is the exact number the ownerless
  guard trusts, so an off-by-one here is a silent correctness bug, not just
  a test gap.
- `TestRepository_UpdateMemberRoleNotFoundReturnsSentinel` — update against
  a nonexistent `(project_id, user_id)` pair returns
  `domain.ErrMembershipNotFound`.
- `TestRepository_ListMembersOrdered` — asserts stable `ORDER BY user_id`.

### `api-gateway/internal/adapter/wscompat/channels_test.go`

7 tests total (4 wiring-only + 3 new), following
`TestDevServerListChannel_Success`'s `fake*Client` pattern:

- `TestProjectCreateChannel_Success` — asserts `TenantId` is set from
  `Identity.TenantID`, not left zero (the exact bug SOL-020's raw sketch
  had before adaptation).
- `TestProjectGetChannel_Success`
- `TestProjectListChannel_Success` — asserts `TenantId` set; asserts
  missing `args` (empty slice) doesn't panic (defaults applied).
- `TestProjectUpdateChannel_Success`
- `TestProjectGetMembersChannel_Success` — asserts channel name
  `project.getMembers` maps onto `client.ListMembers` (the name-mismatch
  case flagged in the SOL's test plan).
- `TestProjectRemoveMemberChannel_Success`
- `TestProjectUpdateMemberRoleChannel_Success` — asserts `role` string arg
  maps to the correct `projectv1.ProjectRole` enum value via
  `toProjectRoleArg`.

Plus:

- `TestProjectChannels_PropagateErrors` (table test over all 7).

## Verify

```bash
cd /opt/repos/orca/backend-go/services/project-service
go test ./internal/domain/... ./internal/usecase/... ./internal/adapter/postgres/... -count=1 -v

cd /opt/repos/orca/backend-go/services/api-gateway
go test ./internal/adapter/wscompat/... -run "TestProject" -v -race

cd /opt/repos/orca/backend-go
go build ./... && go vet ./...
```
