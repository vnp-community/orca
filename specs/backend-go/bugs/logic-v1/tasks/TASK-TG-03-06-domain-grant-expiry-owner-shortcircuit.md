# TASK-TG-03-06: `ResolveGrant` expiry filter + explicit `now`, owner-intrinsic short-circuit, real `action` wiring

**From Solution:** SOL-TG-03
**Priority:** P1
**Service:** `task-service`
**File:** `backend-go/services/task-service/internal/domain/grant_resolution.go`, `backend-go/services/task-service/internal/usecase/resolve_permission.go`
**Depends on:** TASK-TG-01-01 (`Task.OwnerID`), TASK-TG-03-04 (proto `Grant.ID`/`expires_at`/`ResolvePermissionRequest.action`), TASK-TG-03-05 (migration)
**Status:** `[x]` DONE — domain.Grant widened (ID/ExpiresAt); ResolveGrant now takes explicit 'now' and filters expired grants (boundary: !After); a shared Clock/SystemClock port added (mirrors auth-service) and injected into ResolvePermission/GetSubtree; grpc ResolvePermission handler threads the real req.GetAction() instead of hardcoding "read". go test ./internal/domain/... -run TestResolveGrant, ./internal/usecase/... -run TestResolvePermission, and ./internal/adapter/grpc/... all pass.

---

## Context

Three related fixes in the same call chain: (1) `domain.Grant` needs `ID`/
`ExpiresAt`; `ResolveGrant` needs to ignore expired grants and take `now`
as an explicit parameter (kept a pure function — no `time.Now()` call
inside `domain/`); (2) the owner-intrinsic short-circuit — a caller whose
ID matches `Task.OwnerID` gets a synthesized `GrantLevelOwner` grant,
`ApplyTree=true`, without a stored row; (3) `internal/adapter/grpc.Server.ResolvePermission`
currently hardcodes `Action: "read"` — thread the real wire field through
instead.

## Changes to make

Widen `backend-go/services/task-service/internal/domain/grant.go`'s `Grant`
struct:

```go
type Grant struct {
	ID        string // new — needed by RevokeGrant
	TaskID    string
	SubjectID string
	Level     GrantLevel
	ApplyTree bool
	ExpiresAt *time.Time // new — nil = never expires
}
```

Add `"time"` to that file's imports.

Rewrite `backend-go/services/task-service/internal/domain/grant_resolution.go`'s
`ResolveGrant` to take `now time.Time` and filter expired grants:

```go
func ResolveGrant(ancestorChain []string, grantsByTask map[string][]Grant, caller CallerIdentity, maxDepth int, now time.Time) (GrantLevel, bool) {
	if maxDepth <= 0 {
		maxDepth = len(ancestorChain)
	}

	var best GrantLevel
	found := false

	for depth, taskID := range ancestorChain {
		if depth >= maxDepth {
			break
		}
		for _, g := range grantsByTask[taskID] {
			if depth > 0 && !g.ApplyTree {
				continue
			}
			if g.ExpiresAt != nil && !g.ExpiresAt.After(now) { // expired at or before `now` — !After, not !Before, is the boundary test
				continue
			}
			if !g.Matches(caller) {
				continue
			}
			if !found || g.Level.priority() < best.priority() {
				best = g.Level
				found = true
			}
		}
	}

	if !found {
		return GrantLevelUnspecified, false
	}
	return best, true
}
```

Add `"time"` to that file's imports.

Update EVERY call site of `domain.ResolveGrant` to pass `time.Now()` (the
one place per Clean Architecture allowed to know wall-clock time):
`resolve_permission.go`'s `Execute`, and `TASK-TG-01-06`'s `GetSubtree.Execute`.
Prefer a `Clock` interface (`Now() time.Time`) injected into both usecases
rather than a bare `time.Now()` call, matching this codebase's existing
testability convention where one exists — check `usecase/ports.go` for an
existing `Clock` port before adding a new one (`SOL-TG-04`'s `execute_task.go`
design also references `uc.clock.Now()`, so a shared port is worth adding
once here rather than duplicating).

Rewrite `backend-go/services/task-service/internal/usecase/resolve_permission.go`'s
`Execute` to add the owner-intrinsic short-circuit and real `now`:

```go
func (uc *ResolvePermission) Execute(ctx context.Context, in ResolvePermissionInput) (domain.GrantLevel, error) {
	tenantID, err := tenant.RequireTenantID(ctx)
	if err != nil {
		return domain.GrantLevelUnspecified, apperrors.New(apperrors.KindUnauthenticated, "TASK_NO_TENANT", "no tenant in request context", err)
	}

	ancestors, err := uc.tasks.GetAncestors(ctx, tenantID, in.TaskID, uc.maxDepth)
	if err != nil {
		return domain.GrantLevelUnspecified, apperrors.New(apperrors.KindNotFound, "TASK_NOT_FOUND", "task not found while resolving ancestors", err)
	}
	chain := make([]string, 0, len(ancestors))
	for _, a := range ancestors {
		chain = append(chain, a.ID)
	}

	grantsByTask, err := uc.grants.ListGrantsForAncestors(ctx, tenantID, chain)
	if err != nil {
		return domain.GrantLevelUnspecified, apperrors.New(apperrors.KindInternal, "TASK_GRANT_LIST_FAILED", "failed to list grants for ancestor chain", err)
	}

	// Owner-intrinsic short-circuit: synthesize an Owner-level grant at the
	// target task, ApplyTree=true, so an owner behaves identically to a
	// real owner grant for the whole subtree they'd expect to manage —
	// without a stored row. ancestors[0] is the target task itself (same
	// convention GetAncestors always returns).
	if len(ancestors) > 0 && in.UserID != "" && ancestors[0].OwnerID == in.UserID {
		grantsByTask[ancestors[0].ID] = append(grantsByTask[ancestors[0].ID], domain.Grant{
			TaskID: ancestors[0].ID, SubjectID: in.UserID, Level: domain.GrantLevelOwner, ApplyTree: true,
		})
	}

	teamIDs, err := uc.teams.ResolveTeams(ctx, tenantID, in.UserID)
	if err != nil {
		return domain.GrantLevelUnspecified, apperrors.New(apperrors.KindInternal, "TASK_TEAM_RESOLVE_FAILED", "failed to resolve caller's team membership", err)
	}

	caller := domain.CallerIdentity{UserID: in.UserID, TeamIDs: teamIDs, CompanyID: tenantID}
	level, found := domain.ResolveGrant(chain, grantsByTask, caller, uc.maxDepth, time.Now())
	if !found {
		return domain.GrantLevelUnspecified, errNoGrant(nil)
	}

	allowed, err := uc.opa.Decision(ctx, level, in.Action, tenantID)
	if err != nil || !allowed {
		return domain.GrantLevelUnspecified, errNoGrant(err)
	}
	return level, nil
}
```

Add `"time"` to `resolve_permission.go`'s imports.

Wire the real `action` field end-to-end in
`backend-go/services/task-service/internal/adapter/grpc/server.go`'s
`ResolvePermission` handler — replace the hardcoded `Action: "read"`:

```go
func (s *Server) ResolvePermission(ctx context.Context, req *taskv1.ResolvePermissionRequest) (*taskv1.ResolvePermissionResponse, error) {
	level, err := s.resolvePermission.Execute(ctx, usecase.ResolvePermissionInput{
		TaskID: req.GetTaskId(),
		UserID: req.GetUserId(),
		Action: req.GetAction(), // real field now — closes README.md's "not reachable through the RPC surface yet" gap
	})
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	return &taskv1.ResolvePermissionResponse{EffectiveLevel: toProtoGrantLevel(level)}, nil
}
```

Every OTHER RPC that internally calls `ResolvePermission` (`Grant`,
`RevokeGrant` from `TASK-TG-03-01`/`TASK-TG-03-07`, `GetSubtree` from
`TASK-TG-01-06`) already passes its own real action (`"manage"`, `"manage"`,
`"read"` respectively) at the usecase-call-site level — this task only
fixes the ONE gRPC handler that hardcoded a constant instead of reading the
wire field.

## Verify

```bash
cd /opt/repos/orca/backend-go
go build ./services/task-service/...
go test ./services/task-service/internal/domain/... -run TestResolveGrant -v
go test ./services/task-service/internal/usecase/... -run TestResolvePermission -v
go test ./services/task-service/internal/adapter/grpc/... -v
```

Expected: `grant_resolution_test.go` — expired non-inherited grant on the
target task is ignored; expired `ApplyTree=true` ancestor grant is ignored
but a non-expired one at the same depth still wins; `now` exactly equal to
`expires_at` counts as expired (explicit boundary test). `resolve_permission_test.go` —
owner short-circuit resolves `GrantLevelOwner` with zero rows in
`grantsByTask`; a non-owner with a real `GrantLevelUser` grant still
resolves that grant, not owner; owner short-circuit composes correctly with
expiry (an owner is never "expired" — synthesized fresh every call). gRPC
server test confirms a real non-"read" `action` on the wire now reaches
OPA (regression guard against `README.md`'s previously-undocumented gap).
