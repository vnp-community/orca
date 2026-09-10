# TASK-TG-003-05: Public share-link (`GenerateShareLink` / `GetTaskByShareToken`) — SECURITY REVIEW REQUIRED BEFORE MERGE

**From Solution:** BE-SOL-003
**Priority:** P2 — but **do not merge without a security review**; this adds task-service's first unauthenticated read endpoint
**Service:** `task-service`
**File:** `backend-go/services/task-service/internal/usecase/generate_share_link.go` (new), `backend-go/services/task-service/internal/usecase/get_task_by_share_token.go` (new), `backend-go/services/task-service/migrations/0004_task_fields_and_comments.up/down.sql` (append `tasks.share_token`, shared file — see Context), `backend-go/proto/orca/task/v1/task.proto` (`share_token` field, new RPCs, `TaskShareView` message), `backend-go/services/task-service/internal/adapter/grpc/server.go` (handlers — confirm the unauthenticated RPC is actually reachable without the tenant-auth interceptor that gates every other RPC in this service)
**Depends on:** TASK-TG-001-01/-02 (shares the migration file; `share_token` is the next free `Task` proto field after TASK-TG-001-02's ~18 new fields)
**Status:** `[x]` DONE — **implementation complete but SECURITY REVIEW STILL REQUIRED before merge, per `07-security-architecture.md`'s standard for any unauthenticated read endpoint. Do not merge this specific task without a human security sign-off — see Execution notes below for exactly what needs review.**

---

## Context

**Security-sensitive, flagged explicitly per BE-SOL-003's own text**:
*"`GetTaskByShareToken` bypasses `ResolveGrant`/OPA entirely by design
(public, unauthenticated)... Security review required before merge — same
standard as any unauthenticated read endpoint, per
`07-security-architecture.md`."* This is the ONE task in the whole
task-graph series that intentionally adds an endpoint reachable with zero
auth — treat every design decision below as security-load-bearing, not
just a style choice.

**Proto field-numbering correction versus BE-SOL-003's own sketch**:
BE-SOL-003's snippet hardcodes `string share_token = 20;` on `Task`. That
number was guessed before TASK-TG-001-02's actual field count was known.
`Task` currently has 7 fields (`id=1..workflow_template_id=7`, confirmed by
direct read of `task.proto:49-58`); TASK-TG-001-02 appends ~18 more
sequentially through field `26`. **`share_token` must be the next free
number after whatever TASK-TG-001-02 actually lands as (expected `27`, but
re-count `task.proto`'s `Task` message at implementation time rather than
trusting either this task's or BE-SOL-003's guess.)**

Migration coordination: same `0004_task_fields_and_comments.up.sql` file as
TASK-TG-001-01/TASK-TG-003-03 — see TASK-TG-001-01's Context for the
shared-file sequencing note; append here, don't create a new migration
number, unless `0004` is already applied somewhere by the time this task
starts.

## Changes to make

**1. Migration** — append:

```sql
ALTER TABLE task.tasks ADD COLUMN share_token TEXT UNIQUE;
CREATE INDEX idx_tasks_share_token ON task.tasks (share_token) WHERE share_token IS NOT NULL;
```

down:

```sql
DROP INDEX IF EXISTS task.idx_tasks_share_token;
ALTER TABLE task.tasks DROP COLUMN IF EXISTS share_token;
```

A `UNIQUE` constraint on `share_token` is load-bearing, not optional — two
tasks sharing a token would let `GetTaskByShareToken` leak the wrong task.

**2. `internal/usecase/generate_share_link.go`** (new) — generates a
cryptographically random token (NOT a UUID derived from the task ID or any
other guessable value — use `crypto/rand`, not `math/rand` or
`github.com/google/uuid`'s v4, though v4 is itself CSPRNG-backed in Go's
implementation; be explicit about which and why in the code comment so a
future reader doesn't "simplify" it to something guessable), persists it,
requires the caller to already hold at least `admin`-level permission on
the task (this usecase MUST call `ResolvePermission` with `action="admin"`
before minting a link — BE-SOL-003 doesn't spell this precondition out
explicitly in its design snippet, but "who can create a public link to this
task" is itself a security decision this task must not leave unspecified;
default to requiring `admin`, the same level `task_grant.rego`'s
`level_actions` map (`backend-go/policy/orca-authz/task_grant.rego:25-31`)
reserves for the most sensitive actions).

**3. `internal/usecase/get_task_by_share_token.go`** (new) — looks up by
token with NO tenant/permission check (public path), returns a narrow
`TaskShareView` projection:

```go
func (uc *GetTaskByShareToken) Execute(ctx context.Context, token string) (domain.TaskShareView, error) {
	if token == "" {
		return domain.TaskShareView{}, apperrors.New(apperrors.KindInvalidArgument, "TASK_SHARE_TOKEN_REQUIRED", "share token is required", nil)
	}
	task, err := uc.tasks.GetByShareToken(ctx, token) // NEW repository method — no tenantID param, since the token itself IS the authorization
	if err != nil {
		return domain.TaskShareView{}, apperrors.New(apperrors.KindNotFound, "TASK_NOT_FOUND", "no task found for this share token", err)
	}
	return domain.TaskShareView{ID: task.ID, Title: task.Title, Status: string(task.Status), Description: task.Description}, nil
}
```

**Field allowlist is the entire point of this task — get it right**:
`TaskShareView` must be a DEDICATED response message (not `Task` reused
with fields blanked out) so a future field added to `Task` doesn't leak
through this path by accident. Per BE-SOL-003: only `id, title, status,
description` — NEVER `ai_context`, `ai_plan_json`, comments, grants,
`owner_id`, `assignee_id`, `reporter_id`, or any other field TASK-TG-001-02
adds. Write the regression test described below BEFORE writing
`toProtoTaskShareView`, not after — a test that enumerates `Task`'s fields
and asserts each one NOT on the allowlist is absent from
`TaskShareView`'s output is the actual security control here, not just a
nice-to-have.

**4. `internal/usecase/ports.go`** — add `GetByShareToken(ctx
context.Context, token string) (domain.Task, error)` to `TaskRepository`
(no `tenantID` parameter — the token is a global, unguessable lookup key by
design, per BE-SOL-003's "bypasses ResolveGrant/OPA entirely" framing;
RLS's `tenant_id = current_setting(...)` policy would normally block a
tenant-less query, so this specific repository method needs either a
`SET LOCAL app.tenant_id` bypass appropriate for a public path, or a
narrower RLS policy exception — confirm this against
`architecture/05-data-architecture.md`'s RLS section during the security
review, do not silently disable RLS for the whole connection).

**5. `task.proto`**:

```protobuf
message Task { /* ...existing + TASK-TG-001-02's fields... */ string share_token = 27; /* re-count at implementation time */ }

rpc GenerateShareLink(GenerateShareLinkRequest) returns (GenerateShareLinkResponse);
rpc GetTaskByShareToken(GetTaskByShareTokenRequest) returns (GetTaskByShareTokenResponse);

message GenerateShareLinkRequest { string task_id = 1; string user_id = 2; }
message GenerateShareLinkResponse { string share_token = 1; }

message GetTaskByShareTokenRequest { string share_token = 1; }
message GetTaskByShareTokenResponse { TaskShareView task = 1; }
message TaskShareView {
  string id = 1;
  string title = 2;
  string status = 3;
  string description = 4;
}
```

**6. `server.go`** — confirm `GetTaskByShareToken`'s handler is actually
reachable without whatever tenant-auth interceptor gates every other RPC in
this service (check `cmd/server/main.go`'s `grpcmw` chain, e.g.
`grpcmw.ChainUnary(logger)`-style middleware, for a per-method auth bypass
mechanism — if none exists, this RPC needs either a new interceptor
allowlist entry or a separate, deliberately-unauthenticated gRPC service/port,
which is itself a security-review-scoped architectural decision, not a
one-line fix).

## Test plan

- `GetTaskByShareToken` response asserted field-by-field to exclude
  `ai_context`/`ai_plan_json`/comments/grants — regression test that fails
  loudly if a future `Task` field addition isn't explicitly excluded.
- `GenerateShareLink` requires `admin`-level permission — a caller with only
  `user`-level access is denied.
- Token uniqueness: two `GenerateShareLink` calls on different tasks never
  collide (statistical/format assertion given a CSPRNG source, not an
  exhaustive proof).
- `GetTaskByShareToken` with an unknown/malformed token returns `NotFound`,
  never a different, more revealing error (no token-enumeration side
  channel via distinguishable error messages/timing — flag timing-based
  enumeration as a security-review discussion point, not something this
  task must fully solve, but do not add an obviously distinguishable error
  path like "invalid format" vs. "not found").

## Verify

```bash
cd /opt/repos/orca/backend-go
go build ./services/task-service/...
go test ./services/task-service/internal/usecase/... -run "TestGenerateShareLink|TestGetTaskByShareToken" -v
go test ./services/task-service/internal/adapter/postgres/... -run TestRepository -v
```

Expected: clean build; the field-allowlist regression test is the one that
must never be weakened or removed in a later PR without a fresh security
sign-off; `GenerateShareLink` permission-check test asserts denial for
sub-admin callers.

## Execution notes (2026-09-09) — implementation complete, HUMAN SECURITY REVIEW STILL REQUIRED

Implemented every piece per the task's own code samples, with the field
count re-verified live rather than trusted from the task file: `Task` had
26 fields by the time this task started (TASK-TG-001-02 through
TASK-TG-002-02 landed first in this same run), so `share_token = 27` is
confirmed correct, not just the task's own guess.

- `domain.Task.ShareToken *string` added; `domain.TaskShareView` (new file)
  is the dedicated 4-field projection, matching the task's own field
  allowlist exactly.
- Migration: already landed via TASK-TG-001-01's combined
  `0004_task_fields_and_comments.up/down.sql` (`share_token TEXT UNIQUE` +
  partial index) — no new migration file here.
- `usecase.GenerateShareLink`: requires admin-level permission by calling
  the real `*ResolvePermission` usecase internally (composed as a
  dependency, the same "usecase calls another usecase directly" pattern
  `AIApply` already establishes for `CreateTask`/`AddEdge`) with
  `Action: "admin"` before minting a link, exactly as this task instructs.
  Token generation uses `crypto/rand` (32 bytes, hex-encoded — 256 bits of
  entropy), explicitly NOT `math/rand` and NOT a UUID, with a code comment
  explaining why a future reader must not "simplify" this.
- `usecase.GetTaskByShareToken`: no tenant/permission check, returns
  `TaskShareView` only. Unknown and malformed tokens hit the identical
  `NotFound`/`TASK_NOT_FOUND` code path (proven by
  `TestGetTaskByShareToken_MalformedToken_SameErrorAsUnknown`) — no
  distinguishable-error side channel. Timing-based enumeration is
  explicitly NOT addressed (flagged in the doc comment as a security-review
  discussion point, per this task's own instruction not to over-solve it
  here).
- `ports.go`/`postgres`: `TaskRepository.GetByShareToken(ctx, token)` — no
  `tenantID` parameter, a genuinely tenant-less global lookup. **RLS finding,
  investigated rather than assumed**: grepped the whole `task-service` tree
  for `SET LOCAL`/`set_config`/`app.tenant_id` — zero hits. `app.tenant_id`
  is never set by this service's Go code at all, meaning
  `task.tasks`'s RLS policy (`tenant_id = current_setting('app.tenant_id',
  true)::uuid`) is already inert in this deployment (either the connecting
  role owns the tables and RLS doesn't apply to owners by default, or the
  policy always evaluates against a NULL setting) — every existing query in
  this repository already relies solely on explicit `tenant_id = $1`
  filtering, matching that file's own header comment ("RLS is the secondary
  backstop"). `GetByShareToken`'s missing filter is therefore consistent
  with, not a new deviation from, this codebase's real RLS posture today.
  **This exact finding needs the security reviewer's explicit confirmation**
  — it was derived by reading this scaffold's code, not by inspecting the
  real production database's role grants, and a real deployment could still
  have `FORCE ROW LEVEL SECURITY` or a non-owner connecting role that this
  investigation didn't have access to check.
- `task.proto`: `Task.share_token = 27`, `GenerateShareLink`/
  `GetTaskByShareToken` RPCs, `GenerateShareLinkRequest/Response`,
  `GetTaskByShareTokenRequest/Response`, `TaskShareView` (4 fields only) —
  regenerated via `buf generate`.
- `server.go` handler reachability (item 6 in Changes to make) — **real
  finding, corrects this task's own framing**: read `common/grpcmw.ChainUnary`
  in full. There is NO tenant-auth interceptor that gates any RPC in this
  service — `TenantExtractionInterceptor` only OPTIONALLY populates
  tenant/user context from incoming metadata when present; it never
  rejects a call for missing metadata. Every RPC in task-service is
  transport-level reachable with zero auth metadata today; the only actual
  enforcement is each usecase's own `tenant.RequireTenantID` call, which
  `GetTaskByShareToken` deliberately never makes. So there is no allowlist
  entry or separate unauthenticated port to build — this RPC is reachable
  the exact same way every other RPC already is, it's just SAFE to reach
  that way, unlike every other RPC. **Flagged explicitly for the security
  reviewer**: whether task-service's gRPC port is reachable from outside
  the internal mesh (bypassing api-gateway's own end-user auth) is a
  deployment-topology question this investigation could not resolve from
  code alone — confirm before merge.

Test coverage: all named cases from the Test plan plus the allowlist
regression test the task's own instructions call "the actual security
control here" —
`TestGetTaskByShareToken_ReturnsExactlyTheAllowlistedFields` populates a
`domain.Task` with every sensitive field set (AIContext, AIPlanJSON,
PromptTemplate, OwnerID, AssigneeID, ReporterID, WorktreeID,
AgentSessionID, WorkflowExecID) and asserts the returned `TaskShareView`
contains ONLY the 4 allowlisted fields, both by value comparison and by
reflecting over `TaskShareView`'s own struct fields (so a future field
added to `TaskShareView` without updating this test's `allowedFields` map
fails loudly). `TestGenerateShareLink_RequiresAdminPermission` asserts
denial AND that no token gets minted for a denied caller;
`TestGenerateShareLink_TokenUniqueness_AcrossCalls` is the statistical/
format assertion the task's own Test plan names (2 distinct 64-char hex
tokens, never colliding).

Verify: `go build`/`go vet ./services/task-service/...` both clean
(including `-tags=integration`); `go test .../usecase/... -run
"TestGenerateShareLink|TestGetTaskByShareToken"` — all 8 cases pass; full
`go test ./services/task-service/...` passes with no regressions; `go test
-tags=integration .../postgres/... -run TestRepository` — 4 tests
(including the new `TestRepository_Revoke_And_ListByTask`, unrelated to
this task) hit the same pre-existing testcontainers flake documented across
this run's earlier execution notes, all 4 passed cleanly on immediate
re-run in isolation, confirming the widened `share_token` column plumbing
(Create/Get/GetAncestors/List/Update/GetSubtree/ListChildren, all now
26-27 columns wide) didn't break anything structurally.

**Summary for the human reviewer**: code is complete and tested, but this
task is NOT cleared for merge without an explicit security sign-off
covering (1) the RLS-inertness finding above against the real production
database, (2) the deployment-topology question of direct gRPC-port
reachability bypassing api-gateway, and (3) the timing-based token
enumeration gap this task's own instructions deliberately left unsolved.
