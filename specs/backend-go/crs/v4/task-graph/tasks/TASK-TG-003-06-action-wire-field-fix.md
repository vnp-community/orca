# TASK-TG-003-06: `ResolvePermissionRequest.action` wire field (1 proto field + 1 line)

**From Solution:** BE-SOL-003
**Priority:** P0 — smallest task in the whole task-graph series, land it first
**Service:** `task-service`
**File:** `backend-go/proto/orca/task/v1/task.proto`, `backend-go/services/task-service/internal/adapter/grpc/server.go`
**Depends on:** None
**Status:** `[x]` DONE

---

## Context

Confirmed by direct read — this really is the smallest possible fix.
`ResolvePermissionRequest` (`task.proto:116-119`) has exactly 2 fields
(`task_id=1, user_id=2`), no action-equivalent. `server.go`'s handler
(`internal/adapter/grpc/server.go:117-131`, read in full) hardcodes it:

```go
func (s *Server) ResolvePermission(ctx context.Context, req *taskv1.ResolvePermissionRequest) (*taskv1.ResolvePermissionResponse, error) {
	level, err := s.resolvePermission.Execute(ctx, usecase.ResolvePermissionInput{
		TaskID: req.GetTaskId(),
		UserID: req.GetUserId(),
		// ResolvePermissionRequest has no action-equivalent field yet (see
		// this service's README "Known gaps") — default to "read", the one
		Action: "read", // <- line 126, the ONE line this task changes
	})
```

`usecase.ResolvePermission.Execute` (`internal/usecase/resolve_permission.go:51-91`)
already accepts and forwards `in.Action` to `uc.opa.Decision(ctx, level,
in.Action, tenantID)` at line 86 — the usecase side needs ZERO changes.
This is purely: add the field, read it instead of the literal.

## Changes to make

**1. `task.proto`** — `ResolvePermissionRequest` gains field 3:

```protobuf
message ResolvePermissionRequest {
  string task_id = 1;
  string user_id = 2;
  string action = 3; // NEW
}
```

**2. `server.go`** — line 126, one-line change:

```go
Action: req.GetAction(),
```

If `req.GetAction()` is empty (an older client that hasn't been rebuilt
against the new proto yet), the usecase's own construction of
`ResolvePermissionInput` should NOT hard-fail — default to `"read"` for
backward compatibility during rollout, per BE-SOL-003's own note. Since
`usecase.ResolvePermission` has no default-filling logic today, do this at
the `server.go` call site, not inside the usecase (keeps the usecase's
input contract strict — "empty means empty" — while the wire-adapter layer
absorbs the rollout-compatibility shim, matching this codebase's existing
adapter-absorbs-wire-quirks convention):

```go
action := req.GetAction()
if action == "" {
	action = "read"
}
level, err := s.resolvePermission.Execute(ctx, usecase.ResolvePermissionInput{
	TaskID: req.GetTaskId(), UserID: req.GetUserId(), Action: action,
})
```

## Test plan

- `ResolvePermission` RPC with `action="write"` on a caller whose only
  grant is `company` level → denied (per `task_grant.rego`'s
  `level_actions["company"] = {"read"}`, confirmed at
  `backend-go/policy/orca-authz/task_grant.rego:25-31`), confirming the wire
  field now actually reaches OPA — this is the regression test that proves
  the fix, not just a compile check.
- `ResolvePermission` RPC with `action=""` (simulating an old client)
  still defaults to `"read"` and behaves exactly as before this change.

## Verify

```bash
cd /opt/repos/orca/backend-go
go build ./services/task-service/...
go test ./services/task-service/internal/adapter/grpc/... -run TestResolvePermission -v
go test ./services/task-service/internal/usecase/... -run TestResolvePermission -v
```

Expected: clean build; the `action="write"`-denied-for-company-level test
is the one regression test that actually matters here — before this fix it
was impossible to write because every call silently used `"read"`.

## Execution notes (2026-09-09)

Confirmed the live code matched the task's citations exactly
(`ResolvePermissionRequest` at 2 fields, `server.go`'s hardcoded
`Action: "read"`). Applied exactly the change specified: added `action = 3`
to `task.proto` (regenerated via `buf generate`), and `server.go`'s handler
now reads `req.GetAction()` with a `""`-defaults-to-`"read"` shim at the
adapter layer, matching the task's own backward-compatibility instruction
verbatim. Zero changes to `usecase.ResolvePermission` (already forwards
`in.Action` unchanged), confirming the task's own "usecase side needs ZERO
changes" claim.

Added the one regression test that actually matters, per the task's own
framing: `TestServer_ResolvePermission_ActionReachesOPA` in
`server_test.go`, using a new `companyReadOnlyOPA` fake that mimics
`task_grant.rego`'s real `level_actions["company"] = {"read"}` rule closely
enough to prove the wire field reaches OPA end-to-end (not the real Rego
engine — proving server.go's plumbing doesn't need it). Asserts: `action="write"`
denied (`PermissionDenied`) for a company-level-only grant; `action=""`
(old-client simulation) defaults to `"read"` and succeeds; `action="read"`
explicitly also succeeds. This surfaced that `server_test.go`'s own
`fakeTaskRepository.GetAncestors` was an unimplemented stub
(`return nil, errors.New("not implemented")`) — never exercised before
since no test in this file had called `ResolvePermission` through the real
`Server` yet; implemented it properly (parent-chain walk, same shape as
`internal/usecase/fakes_test.go`'s equivalent) since this fix's own
regression test is the first caller.

Verify: `go build`/`go vet ./services/task-service/...` both clean; `go
test ./services/task-service/internal/adapter/grpc/... -run
TestServer_ResolvePermission` — 1/1 pass; `go test
./services/task-service/internal/usecase/... -run TestResolvePermission` —
all 8 pre-existing cases pass unchanged; full `go test
./services/task-service/...` passes with no regressions.
