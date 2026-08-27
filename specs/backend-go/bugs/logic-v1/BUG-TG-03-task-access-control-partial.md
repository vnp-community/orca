# BUG-TG-03: Grant-resolution algorithm is real and well-tested, but team grants never match (stub resolver), and revoke/expiry/public-link/notification are entirely absent

**Business Logic:** [BL-TG-03](../../../../docs/logic/task-graph/BL-TG-03-task-access-control.md) — Task Access Control & Sharing
**Priority (per spec):** P0
**Status:** PARTIAL
**Severity:** High
**Symptom:** An Owner can grant a permission level to a specific user (or, on paper, a team/company) on a task, with real ancestor-tree inheritance (`apply_tree`) and priority-based resolution — this is genuine access-control logic, not just a schema field. But: any grant scoped to a **team** silently never matches any caller (the team-membership resolver is a hardcoded stub that always returns zero teams), there is **no way to revoke a grant** once created, grants **never expire** (no expiry field exists at all), there is **no public/anonymous share-link** flow, and a grantee receives **no notification** of any kind when access is granted.

---

## Spec summary

BL-TG-03 defines a 5-tier ordered permission scale (`view < comment < edit < execute < manage`) and a `hasTaskAccess()` resolution algorithm: owner always wins, admin always wins, then direct/team/company grants on the task itself plus inherited (`apply_tree=true`) grants from ancestors, filtered by expiry, resolved to the highest matching permission. It also specifies grant CRUD (add/revoke/list), a public-link share flow (anonymous read-only access via a random token), and a WebSocket push (`task.grant_received`) to the grantee when a grant is created.

## What backend-go has

- **A real, unit-tested BFS ancestor-walk resolution algorithm**, `domain.ResolveGrant` (`backend-go/services/task-service/internal/domain/grant_resolution.go:37-67`): walks the ancestor chain (target task's own grants always count; ancestors' grants only count with `ApplyTree=true`), collects every matching grant across the whole bounded walk, and picks the highest-priority match by a real 5-value `GrantLevel` (`owner > admin > user > team > company`, `internal/domain/grant.go:34-49`) — a faithful, tested analogue of the spec's `priority > proximity` resolution rule (20 unit tests per the service README, `grant_resolution_test.go`).
- **`apply_tree` inheritance is real**: `Grant.ApplyTree` (`internal/domain/grant.go:54-59`) and the depth-gated check in `ResolveGrant` (`grant_resolution.go:50-51`) implement exactly the spec's "grant on target always applies; ancestor grant only applies if `apply_tree=true`" rule.
- **`ResolvePermission` usecase** (`internal/usecase/resolve_permission.go:51-91`) wires `TaskRepository.GetAncestors` + `GrantRepository.ListGrantsForAncestors` + `TeamScopeResolver` into the domain walk, then asks an OPA policy (`common/policy.Evaluator` against `task_grant.rego`) for the final allow/deny, failing closed on any error — matching the spec's expiry-then-priority-then-decision shape structurally (see caveats below).
- **`Grant` usecase** (`internal/usecase/grant.go:30-50`) validates and persists a grant (owner/admin/user/team/company + `apply_tree`), wired end-to-end: proto (`task.proto:78-95`), gRPC server (`internal/adapter/grpc/server.go:104-115`), REST (`httpgateway/task_routes.go:131-155`, `POST /v1/tasks/{id}/grants`).
- **Owner check is real but implicit**: `domain.Task` has no `OwnerID` field (see BUG-TG-01) — "owner always has full manage" from the spec is not represented in `ResolveGrant`'s inputs at all; the closest analogue is a `GrantLevelOwner`-level `Grant` row, which is a *created grant*, not an intrinsic task-creator property. Likewise "admin has full access to all tasks in their org" is not implemented — there is no role-based short-circuit anywhere in `ResolveGrant` or `ResolvePermission`; every access decision goes through the grant table.

## What's missing

- **Team-scoped grants never match anyone.** `TeamScopeResolver` (`internal/adapter/grpcclient/team_scope_resolver.go:11-26`) is `StubTeamScopeResolver`, hardcoded to return `nil, nil` (empty team list) for every call — wired as-is in production composition root (`cmd/server/main.go:87-89`, comment: *"team-scope resolution ... still STUB"*). Since `Grant.Matches` for `GrantLevelTeam` requires `caller.hasTeam(g.SubjectID)` (`internal/domain/grant.go:89-90`), and `caller.TeamIDs` is always empty, **every team grant is dead on arrival** — a Lead granting "Backend Team: execute" produces a row that can never resolve true for any caller, silently.
- **No grant expiry at all.** `domain.Grant` (`internal/domain/grant.go:54-59`) has no `ExpiresAt`/equivalent field; `GrantInput` (`internal/usecase/grant.go:11-16`) has no expiry parameter; the Postgres schema/insert (`internal/adapter/postgres/grants.go:26-39`) has no expiry column. The spec's "ignore expired grants" acceptance criterion cannot even be represented, let alone enforced.
- **No revoke.** Confirmed via grep across `backend-go/services/task-service/`: no `Revoke`/`revoke` symbol anywhere except the service README noting it as an unimplemented design-doc RPC (`README.md:153,237`). Once a grant is created there is no way to remove it short of a direct DB operation — the spec's "Revoke: delete grant → immediate effect" flow has no code path.
- **No grant listing on the public API surface.** `GrantRepository.ListGrantsForAncestors` (`internal/adapter/postgres/grants.go:45-74`) exists but is internal-only, consumed solely by `ResolvePermission`'s own walk — there is no RPC/REST endpoint that lets an Owner "see who has access to this task" (the spec's "manage" permission includes viewing/managing existing grants).
- **No public/anonymous share-link flow.** No `scope='public_link'`, no share-token generation, no anonymous read-only access path anywhere in the codebase (confirmed via grep for `public_link`/`share_token` — zero hits in `backend-go/`).
- **No grant notification.** No WebSocket push, event, or any other notification mechanism fires when a `Grant` is created — the spec's `{ type: 'task.grant_received', ... }` push has no equivalent; the service README itself notes "No event publishing... `Grant`/`RevokeGrant` should emit structured audit events" is an open gap (`README.md:237-241`).
- **Permission taxonomy mismatch**: the spec's ordered 5-tier `view < comment < edit < execute < manage` scale is not what's implemented — `GrantLevel` (`owner/admin/user/team/company`) conflates *who* the grant is for with *what* they can do, and the actual action-level mapping lives in `task_grant.rego` as (per the service's own README) "a first-cut permission matrix, not a final taxonomy" (`README.md:221-226`). There is also no `comment` action level surfaced anywhere (no comment table usage — see BUG-TG-01).
- **`ResolvePermissionRequest` has no `action` field on the wire** — `internal/adapter/grpc/server.go:117-132` hardcodes `Action: "read"` for every call, so the OPA deny path for `write`/`execute`/`manage`-shaped checks (e.g. "does this caller have `execute` to Run Agent") is exercised only in usecase unit tests, never reachable through the actual RPC surface yet (`README.md:227-236`).

## See also

- None found in `missing-v1`/`api-v1` — this domain's grant-resolution gap is not previously documented as a bug elsewhere; BUG-034 (task channels) only covers WS-wiring for unrelated task RPCs.

## References

- `docs/logic/task-graph/BL-TG-03-task-access-control.md` — full spec (permission levels, `hasTaskAccess`, grant/share flows, inheritance diagram)
- `backend-go/services/task-service/internal/domain/grant_resolution.go:37-67` — `ResolveGrant` BFS walk
- `backend-go/services/task-service/internal/domain/grant.go:11-96` — `GrantLevel`, `Grant`, `CallerIdentity`, `Matches`
- `backend-go/services/task-service/internal/usecase/resolve_permission.go:51-91` — `ResolvePermission.Execute` (OPA fail-closed check)
- `backend-go/services/task-service/internal/usecase/grant.go:30-50` — `Grant.Execute`
- `backend-go/services/task-service/internal/adapter/grpcclient/team_scope_resolver.go:11-26` — `StubTeamScopeResolver` (always empty)
- `backend-go/services/task-service/cmd/server/main.go:87-89` — stub wired into production composition root
- `backend-go/services/task-service/internal/adapter/postgres/grants.go:26-74` — `Grant`/`ListGrantsForAncestors` (no expiry column, no revoke method)
- `backend-go/services/task-service/README.md:153,221-241` — "Known gaps": no RevokeGrant, first-cut permission matrix, no event publishing
- `backend-go/proto/orca/task/v1/task.proto:78-95` — `GrantRequest`/`GrantResponse` (no expiry, no revoke RPC, no public-link RPC)
