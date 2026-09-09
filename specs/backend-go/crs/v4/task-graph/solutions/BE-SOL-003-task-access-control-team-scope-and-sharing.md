# BE-SOL-003: Real `TeamScopeResolver`, owner-grant-on-create, expiry, Revoke/ListGrants, share-link, `action` wire field

**Resolves:** [CR-TG-003](../../../../../../docs/crs/v4/task-graph/CR-TG-003-task-access-control-team-scope-and-sharing.md)
**Service:** `task-service` only (+ `backend-go/policy/orca-authz/task_grant.rego`, unchanged — see correction below)
**Depends on:** [BE-SOL-001](./BE-SOL-001-orcatask-data-model-widening.md) for schema-widening groundwork (this solution does NOT use its `Task.owner_id` column for authorization — see §1)
**Affected files (proposed):**
- `backend-go/services/task-service/internal/adapter/grpcclient/team_scope_resolver.go` (replace stub with real `tenant-service` client)
- `backend-go/services/task-service/internal/usecase/create_task.go` (add `CreatorID` input, auto-insert owner `Grant`)
- `backend-go/services/task-service/internal/domain/grant.go` (add `ExpiresAt *time.Time` — additive, no enum change)
- `backend-go/services/task-service/internal/usecase/revoke_grant.go`, `list_grants.go` (new — public-facing, distinct from internal `ListGrantsForAncestors`)
- `backend-go/services/task-service/internal/usecase/generate_share_link.go`, `get_task_by_share_token.go` (new)
- `backend-go/services/task-service/internal/adapter/grpc/server.go` (read `action` from request instead of hardcoding `"read"`)
- `backend-go/proto/orca/task/v1/task.proto` (`ResolvePermissionRequest.action`, `CreateTaskRequest.creator_id`, `RevokeGrant`/`ListGrants`/`GenerateShareLink`/`GetTaskByShareToken` RPCs, `Task.share_token`)
- `backend-go/services/task-service/migrations/0004_task_fields_and_comments.up.sql` (fold `grants.expires_at`, `tasks.share_token` into the same migration as BE-SOL-001, since both touch `task.tasks`/`task.grants` — avoid two migrations touching adjacent tables back-to-back)
**Status:** 📋 Proposed — not yet implemented

> **⚠️ Cập nhật sau khi viết task (2026-09-09):** §"Design — real
> `TeamScopeResolver`" bên dưới đoán tên RPC `tenant-service`
> (`ListUserTeams`) mà không xác nhận — task đã đọc `tenant-service`'s
> proto thật: RPC đúng tên là **`ListTeamsForUser`**, request chỉ có
> `user_id` (không có `tenant_id` field riêng). Xem
> [TASK-TG-003-01](../tasks/TASK-TG-003-01-real-team-scope-resolver.md)
> cho signature đã xác nhận. `task_grant.rego`'s `level_actions` map cũng
> ở dòng 25-31 thật, không phải 16-24 như trích dẫn trước đó trong tài
> liệu này.

---

## ⚠️ Correction relative to CR-TG-003 — read before implementing

CR-TG-003 §2.2 proposes splitting `GrantLevel` into a `GranteeKind` enum
(User/Team/Company) and an ordered `PermissionLevel` scale
(`view<comment<edit<execute<manage`). **Reading the real code + TDD + OPA
bundle shows this is unnecessary and would be a strictly larger, riskier
change than the actual gap requires** — exactly the kind of thing "đánh giá
trạng thái hiện tại trước, ít thay đổi code nhất" is meant to catch:

- `internal/domain/grant.go:1-9`'s own doc comment: *"Unlike the design
  doc's sketch schema (`grantee_type` ∈ {user,team,company} as a field
  separate from a 3-value level), the generated proto's `GrantLevel` enum
  folds grantee-kind into the level itself... This scaffold follows the
  generated proto, **the authoritative wire contract**."* The "split" shape
  CR-TG-003 proposes is the *design doc's sketch*, already deliberately
  superseded by the real proto — re-introducing it means fighting the wire
  contract, not fixing a bug.
- `task-service.md` §4.1 and §9 independently confirm the 5-value
  `owner > admin > user > team > company` priority scale IS the intended
  design, with an explicit, already-built split: **`domain.ResolveGrant`
  computes the level (Go, pure function) — `task_grant.rego`'s
  `level_actions` map decides whether that level authorizes a given
  *action*** (`read|write|execute|admin`). This is the exact
  domain-computes/OPA-decides architecture CR-TG-003 was trying to invent
  from scratch with a new Go enum — **it already exists**, in
  `backend-go/policy/orca-authz/task_grant.rego:16-24`.
- The real gap is much narrower than CR-TG-003 describes: `ResolvePermission`
  usecase (`internal/usecase/resolve_permission.go:14-23,86`) **already**
  accepts and forwards an `Action` field to OPA (`uc.opa.Decision(ctx,
  level, in.Action, tenantID)`) — the comment there says *"Pass Action
  explicitly once the wire contract grows"*. `server.go:117-129`'s own
  comment confirms exactly why it's hardcoded: *"`ResolvePermissionRequest`
  has no action-equivalent field yet... default to `"read"`... until the
  wire contract grows a real field."* **The fix is one proto field + one
  line in `server.go` — not a domain redesign.**

This solution proceeds on the corrected, minimal-change basis: **`GrantLevel`
is untouched**. Everything below is additive (new field, new RPCs, new
usecase files) or a 1-2 line fix to code that already anticipates the gap
in its own comments.

## Design rationale (grounded in TDD + real code)

`task-service.md` §9: *"`tenant-service` resolves team membership during
the walk — `task-service` never reads `team_members` rows itself"*
(`task-service.md:152-153`, restated at §9's audit-events paragraph). The
real `StubTeamScopeResolver` (`team_scope_resolver.go:11-26`) already
documents its own fix in its doc comment: *"Real wiring needs: a
`tenantv1.TenantServiceClient` (gRPC) dialed to `tenant-service`... Until
that's wired, team-scoped grants (`GrantLevelTeam`) will never match any
caller."* This solution wires exactly that.

## Design — real `TeamScopeResolver`

```go
// team_scope_resolver.go — replaces StubTeamScopeResolver
type TeamScopeResolver struct {
    tenant tenantv1.TenantServiceClient
}

func (r *TeamScopeResolver) ResolveTeams(ctx context.Context, tenantID, userID string) ([]string, error) {
    resp, err := r.tenant.ListUserTeams(ctx, &tenantv1.ListUserTeamsRequest{TenantId: tenantID, UserId: userID})
    if err != nil { return nil, fmt.Errorf("team_scope_resolver: list_user_teams: %w", err) }
    return resp.GetTeamIds(), nil
}
```

**Verify before implementing:** confirm `tenant-service` actually exposes
`ListUserTeams` (or the real equivalent RPC name) — this solution assumes
it per the TDD's framing that `tenant-service` is the system of record for
team membership, but the exact RPC name needs a direct check of
`tenant-service`'s proto before coding (not assumed here to avoid
prescribing a wrong signature).

## Design — owner grant on task creation (not an `OwnerID` short-circuit)

Rather than adding an authorization special-case for `Task.owner_id`
(BE-SOL-001's column is informational/display-only — "who created this,"
shown in UI — it is **not** read by `ResolveGrant`), `CreateTask` inserts a
real `Grant{Level: GrantLevelOwner}` row for the creator. This reuses the
existing grant model exactly as designed — `task_grant.rego`'s
`level_actions["owner"]` already authorizes every action — so "owner always
has full access" falls out of the existing walk with zero changes to
`ResolveGrant`/the Rego bundle.

```go
// create_task.go
type CreateTaskInput struct {
    ID, Title, ParentID, ProjectID string
    CreatorID string // NEW — CreateTaskRequest currently has no equivalent;
                      // follow ResolvePermissionRequest's existing convention
                      // of passing user_id explicitly on the wire (task-service
                      // has no auth-context user-id extractor today — confirmed,
                      // 0 hits for a RequireUserID-style helper in common/).
}

func (uc *CreateTask) Execute(ctx context.Context, in CreateTaskInput) (domain.Task, error) {
    // ...existing validation + uc.repo.Create(...)...
    if in.CreatorID != "" {
        _ = uc.grants.Grant(ctx, tenantID, domain.Grant{
            TaskID: created.ID, SubjectID: in.CreatorID, Level: domain.GrantLevelOwner, ApplyTree: true,
        })
        // best-effort: a failed owner-grant insert should not fail task
        // creation outright in v1 — flag via structured log; a task with
        // no owner grant still resolves via any OTHER matching grant
        // (e.g. a parent's inherited grant), it just has no intrinsic
        // owner until re-granted. Revisit as a hard failure once
        // Grant/CreateTask share one transaction (needs TxRunner wiring
        // this usecase doesn't have today — out of scope here, see §Not
        // in scope).
    }
    return created, nil
}
```

## Design — expiry (additive field, filtered at read time — no `ResolveGrant` signature change)

```go
// domain/grant.go
type Grant struct {
    TaskID, SubjectID string
    Level             GrantLevel
    ApplyTree         bool
    ExpiresAt         *time.Time // NEW, nullable
}
```

Filtering happens where `grantsByTask` is assembled (the usecase layer,
`ResolvePermission.Execute`, before calling the pure `domain.ResolveGrant`)
— **not** by adding a `now time.Time` parameter to `ResolveGrant` itself,
keeping that function's signature (and its existing unit tests) untouched:

```go
// resolve_permission.go — where grantsByTask is built
grantsByTask := uc.grants.ListGrantsForAncestors(ctx, tenantID, ancestorChain)
for taskID, grants := range grantsByTask {
    grantsByTask[taskID] = filterExpired(grants, time.Now())
}
```

## Design — `RevokeGrant` / `ListGrants` (public RPCs, distinct from internal `ListGrantsForAncestors`)

```protobuf
rpc RevokeGrant(RevokeGrantRequest) returns (google.protobuf.Empty);
rpc ListGrants(ListGrantsRequest) returns (ListGrantsResponse); // by task_id — for the Access tab (FE-SOL-001)
```

`RevokeGrant` deletes by `(task_id, subject_id, level)` composite (matching
how `Grant` is currently created/matched — no surrogate `grant_id` exists
today; adding one is a larger schema change this solution avoids unless the
composite-key delete proves ambiguous in practice).

## Design — public share-link

```protobuf
message Task { /* ...BE-SOL-001's fields... */ string share_token = 20; }
rpc GenerateShareLink(GenerateShareLinkRequest) returns (GenerateShareLinkResponse);
rpc GetTaskByShareToken(GetTaskByShareTokenRequest) returns (GetTaskByShareTokenResponse);
```

`GetTaskByShareToken` bypasses `ResolveGrant`/OPA entirely by design (public,
unauthenticated) but returns a **narrow projection** — `id, title, status,
description` only, never `ai_context`/comments/grants — enforced by a
dedicated response message (`TaskShareView`, not the full `Task` message)
so a future field added to `Task` doesn't leak through this path by
accident.

**Security review required before merge** — same standard as any
unauthenticated read endpoint, per `07-security-architecture.md`.

## Design — `action` wire field (the actual one-line-in-spirit fix)

```protobuf
message ResolvePermissionRequest {
  string task_id = 1;
  string user_id = 2;
  string action = 3; // NEW
}
```

```go
// server.go — ResolvePermission handler
Action: req.GetAction(), // was: hardcoded "read"
```

If `req.GetAction()` is empty (older client), default to `"read"` in the
usecase input construction to keep backward compatibility during rollout,
rather than making it a hard validation error immediately.

## Test plan

- `TeamScopeResolver.ResolveTeams` against a `tenant-service` test double —
  grant `scope=team` now matches a caller who is a member.
- Expired grant (`ExpiresAt` in the past) excluded from `grantsByTask`
  before `ResolveGrant` runs — test asserts a caller with only an expired
  grant resolves `Unspecified`/deny.
- `CreateTask` with `CreatorID` set → creator immediately resolves
  `GrantLevelOwner` via the normal `ResolvePermission` path (integration
  test, not a special-cased assertion on `Task.OwnerID`).
- `RevokeGrant` idempotent (revoke twice, second call no-ops, no error).
- `GetTaskByShareToken` response asserted field-by-field to exclude
  `ai_context`/comments — regression test that fails loudly if a future
  `Task` field addition isn't explicitly excluded.
- `ResolvePermission` RPC with `action="write"` on a caller whose only
  grant is `company` level → denied (per `task_grant.rego`'s
  `level_actions["company"] = {"read"}`), confirming the wire field now
  actually reaches OPA.

## Not in scope (per the CR, plus this solution's own correction)

- **No change to `GrantLevel`, `Grant.Matches`, or `task_grant.rego`'s
  `level_actions` map** — see correction above.
- Data migration mapping old `Owner/Admin/User/Team/Company` grants to a
  new scale — moot, since no new scale is introduced.
- Grant-received notification via outbox — additive follow-up, not blocking
  this solution; can land in a fast-follow once the outbox pattern from
  `docs/crs/v3/flow-task/CR-FLOW-TASK-003` is available to reuse (don't
  build a second notification mechanism).
- UI for revoke/share-link — [FE-SOL-001](../../../../../frontend/crs/v4/task-graph/solutions/FE-SOL-001-task-crud-board-grant-ui.md).
- Wrapping `CreateTask` + owner-grant insert in one transaction — needs
  `TxRunner` wiring this usecase doesn't have today; tracked as a follow-up,
  not silently assumed solved by "best-effort" in §Design above.

## References

- [CR-TG-003](../../../../../../docs/crs/v4/task-graph/CR-TG-003-task-access-control-team-scope-and-sharing.md)
- [SOL-TG-03](../../../../bugs/logic-v1/solutions/SOL-TG-03-task-access-control.md) — **this solution actually realigns with SOL-TG-03**, which already proposes the `action` wire-field fix (§"Design — `ResolvePermissionRequest.action` wire field", line 350) without any `GranteeKind`/`PermissionLevel` split; it was CR-TG-003 itself (not SOL-TG-03) that introduced that split, and this solution corrects course back to SOL-TG-03's original, already-minimal design
- `specs/backend-go/tdd/services/task-service.md` §4.1, §9
- `backend-go/policy/orca-authz/task_grant.rego`
