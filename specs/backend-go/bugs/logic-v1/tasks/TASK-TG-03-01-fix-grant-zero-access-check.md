# TASK-TG-03-01: Fix `Grant.Execute` — add the missing `manage`-level access check (live authorization gap)

**From Solution:** SOL-TG-03
**Priority:** P0 — this is a live authorization bug, not a missing feature: today ANY authenticated caller can call `Grant` on ANY task ID they can name, including granting themselves `owner`, with zero access check
**Service:** `task-service`
**File:** `backend-go/services/task-service/internal/usecase/grant.go`
**Depends on:** none — this fix stands alone, does not require any other TG-03 task
**Status:** `[x]` DONE — Grant now requires the caller to already have OPA-authorized 'manage' access via ResolvePermission before writing; CreateTask sets Task.OwnerID at creation and ResolvePermission gained the owner-intrinsic short-circuit (pulled forward from TASK-TG-03-06) so the creating user is never locked out; added "manage" to task_grant.rego's level_actions (owner/admin only). go test ./internal/usecase/... -run TestGrant and opa test policy/orca-authz/ both pass; regression test TestGrant_DeniesWhenCallerHasNoManageAccessToTarget confirms the self-service privilege-escalation gap is closed.

---

## Context

**This is the most urgent fix in this bug set.** `usecase.Grant.Execute`
(`grant.go:30-50`) validates its input shape (task_id/subject_id/level
non-empty, level is a recognized enum value) and then calls
`uc.grants.Grant(...)` directly — it never calls `ResolvePermission` first.
Concretely: any authenticated user who knows (or guesses) a `task_id` UUID
can call `Grant{TaskID: <any task>, SubjectID: <their own user ID>, Level:
GrantLevelOwner, ApplyTree: true}` and receive full `owner` access to a task
they have never been granted any visibility into — self-service privilege
escalation with no audit trail and no error anywhere. `ResolvePermission`
itself already exists and is correct (`resolve_permission.go:51-91`); this
task is exclusively about calling it from `Grant` before writing.

`RevokeGrant`/`ListGrants` (added in `TASK-TG-03-07`) are new code and are
built with this check from the start — this task closes the gap in the
existing `Grant` RPC, which predates it.

## Changes to make

`Grant` needs a `ResolvePermission` dependency. In
`backend-go/services/task-service/internal/usecase/grant.go`:

```go
package usecase

import (
	"context"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/task-service/internal/domain"
)

type GrantInput struct {
	TaskID    string
	SubjectID string
	Level     domain.GrantLevel
	ApplyTree bool
}

// Grant is task-service's grant-mutation usecase. Requires the CALLER to
// already have 'manage' access to TaskID before writing a new grant on it
// — closes a live authorization gap: previously any authenticated caller
// could call Grant on any task_id they could name, including granting
// themselves owner, with zero access check (found while grounding
// SOL-TG-03's design against the current code).
type Grant struct {
	grants            GrantRepository
	resolvePermission *ResolvePermission
}

func NewGrant(grants GrantRepository, resolvePermission *ResolvePermission) *Grant {
	return &Grant{grants: grants, resolvePermission: resolvePermission}
}

func (uc *Grant) Execute(ctx context.Context, in GrantInput) error {
	tenantID, err := tenant.RequireTenantID(ctx)
	if err != nil {
		return apperrors.New(apperrors.KindUnauthenticated, "TASK_NO_TENANT", "no tenant in request context", err)
	}
	if in.TaskID == "" {
		return apperrors.New(apperrors.KindInvalidArgument, "TASK_GRANT_INVALID", "task_id is required", nil)
	}
	if in.SubjectID == "" {
		return apperrors.New(apperrors.KindInvalidArgument, "TASK_GRANT_INVALID", "subject_id is required", nil)
	}
	if !in.Level.Valid() {
		return apperrors.New(apperrors.KindInvalidArgument, "TASK_GRANT_INVALID", "level is not a recognized grant level", nil)
	}

	callerID, _ := tenant.UserID(ctx)
	// The fix: require 'manage' on TaskID before writing ANY grant on it —
	// same "every mutating RPC calls ResolvePermission internally first"
	// rule task-service.md §3 already states for every other mutation.
	if _, err := uc.resolvePermission.Execute(ctx, ResolvePermissionInput{TaskID: in.TaskID, UserID: callerID, Action: "manage"}); err != nil {
		return err // PermissionDenied/TASK_NO_GRANT, unchanged shape
	}

	grant := domain.Grant{TaskID: in.TaskID, SubjectID: in.SubjectID, Level: in.Level, ApplyTree: in.ApplyTree}
	if err := uc.grants.Grant(ctx, tenantID, grant); err != nil {
		return apperrors.New(apperrors.KindInternal, "TASK_GRANT_FAILED", "failed to persist grant", err)
	}
	return nil
}
```

Update `backend-go/services/task-service/cmd/server/main.go`'s composition
root: `grantUC := usecase.NewGrant(repo, resolvePermissionUC)` — note
`resolvePermissionUC` must be constructed BEFORE `grantUC` now (it already
is, at `main.go:117`, before the current `grantUC` line at `main.go:116` —
move `grantUC`'s construction to after `resolvePermissionUC`'s).

Update every test that constructs `usecase.NewGrant(...)` directly
(`grant_test.go`) to pass a `ResolvePermission` instance (real or backed by
fakes that grant the caller `manage` by default, per that test file's
existing fake style) — a caller with no grant on the target task must now
get `PermissionDenied`, not a silent write.

**First-caller bootstrap note**: a brand-new task has no grants at all, so
the FIRST `Grant` call on it (typically the task's creator granting
themselves `owner`) would otherwise be denied by this same check. Confirm
against `CreateTask`'s usecase whether task creation already synthesizes an
implicit owner grant (or sets `Task.OwnerID` per `TASK-TG-01-01`/
`TASK-TG-03-06`'s owner-intrinsic short-circuit) for the creating user — if
not, `CreateTask` needs to set `OwnerID = callerID` at creation time so this
fix doesn't lock every new task's creator out of granting anyone else
access. This is a real dependency to verify during implementation, not
assumed away here.

## Verify

```bash
cd /opt/repos/orca/backend-go
go build ./services/task-service/...
go test ./services/task-service/internal/usecase/... -run TestGrant -v
```

Expected: new test `TestGrant_DeniesWhenCallerHasNoManageAccessToTarget`
(a caller with no grant, and no `OwnerID` match, on the target task gets
`PermissionDenied`); existing `TestGrant_PersistsAValidGrant` still passes
when the fake `ResolvePermission`/caller has `manage` access (task owner or
an existing `owner`/`admin` grant); a regression test confirms a caller
CANNOT grant themselves access to an arbitrary task they don't already have
`manage` on.
