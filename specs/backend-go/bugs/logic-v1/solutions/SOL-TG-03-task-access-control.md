# SOL-TG-03: Real team-scope resolution, grant expiry + revoke, owner/admin short-circuit, and grant notification

**Resolves:** [BUG-TG-03](../BUG-TG-03-task-access-control-partial.md)
**Service:** `task-service` (primary) + `tenant-service` (proto extension — new RPC) + `notification-service` (event consumer, no code change needed beyond its existing outbox-consumption pattern)
**Affected files (proposed):**
- `backend-go/proto/orca/tenant/v1/tenant.proto` (new `ListTeamsForUser` RPC — **scope addition to `tenant-service`**, see below)
- `backend-go/proto/orca/task/v1/task.proto` (`Grant`/`ResolvePermission` widen, `RevokeGrant`/`ListGrants` RPCs, `action` field)
- `backend-go/services/task-service/internal/domain/grant.go` (`ID`, `ExpiresAt` fields; `ResolveGrant` gains `now time.Time`)
- `backend-go/services/task-service/internal/domain/grant_resolution.go` (expiry filter, owner-intrinsic short-circuit input)
- `backend-go/services/task-service/internal/usecase/grant.go` (expiry param, audit event)
- `backend-go/services/task-service/internal/usecase/revoke_grant.go`, `list_grants.go` (new)
- `backend-go/services/task-service/internal/usecase/resolve_permission.go` (owner short-circuit, `now`)
- `backend-go/services/task-service/internal/adapter/grpcclient/team_scope_resolver.go` (real implementation, replaces `StubTeamScopeResolver`)
- `backend-go/services/task-service/internal/adapter/postgres/grants.go` (`expires_at` column, `Revoke`, expiry filter in `ListGrantsForAncestors`)
- `backend-go/services/task-service/internal/adapter/eventbus/` (new — outbox publisher, mirrors `usage-service`'s package)
- `backend-go/services/task-service/migrations/0004_grant_expiry_and_public_link.{up,down}.sql` (new)
- `backend-go/services/task-service/cmd/server/main.go` (dial tenant-service, wire real resolver)
**Status:** 📋 Proposed — not yet implemented

---

## Design rationale (grounded in TDD)

`07-security-architecture.md`'s AuthZ section names exactly this
mechanism as the thing OPA-centralization is meant to fix:
"`TaskGrantService.resolvePermission()` for task-graph BFS ancestor
resolution" is one of "two independent, non-unified permission
mechanisms" the TS system had (`07-security-architecture.md:26-28`).
`task-service.md §9` is explicit about the split that's *already*
correctly implemented here: "`task-service` computes the grant resolution
result itself... The resolved result becomes OPA's input, not OPA's
derivation" (`task-service.md:310-318`) — `ResolvePermission`
(`resolve_permission.go:51-91`) already does exactly this. **This
solution does not touch that split** — it fixes four things that split
depends on being correct: the team-membership input the BFS walk
consumes, the expiry filter the walk's "matching grant" step is
supposed to apply, a revoke path, and the audit-event requirement
`task-service.md §9` states explicitly ("`Grant`/`RevokeGrant` emit
structured audit events per `07-security-architecture.md`'s
audit-logging section," `task-service.md:325-327`).

**Team-scope resolution is the highest-priority fix, treated with the
care the task instructions call for.** §2/§9 are unambiguous that
`tenant-service` — never a local join — is the source of truth
("`tenant-service` resolves team membership during the walk —
`task-service` never reads `team_members` rows itself," `task-service.md:152-153`).
The stub (`team_scope_resolver.go:11-26`) satisfies the *interface*
contract (always returns a valid, if empty, list — never errors) but
silently violates the *semantic* contract every `GrantLevelTeam` grant
depends on. This is a correctness bug with no visible symptom short of an
audit: a Lead grants "Backend Team: execute," the write succeeds, no
error anywhere, and the grant is permanently inert. The fix must close
this without weakening `ResolvePermission`'s fail-closed posture
(`resolve_permission.go:82-89`'s explicit "an evaluation error is treated
exactly like an explicit deny").

**Genuine extension beyond the TDD, flagged explicitly**: `tenant-service.md`'s
sketched RPC surface (`tenant-service.md:79-86`) has `ListTeams`
(company→teams) and `ListTeamMembers` (team→members) but **no
user→teams query** — the direction `TeamScopeResolver` actually needs.
Resolving it via the sketched surface would mean `task-service` calling
`ListTeams` then `ListTeamMembers` per team and filtering client-side —
an N+1 fan-out on task-service's own hot path (`task-service.md §8`
explicitly calls grant resolution "hot-path... target p99 well under the
5s gRPC deadline"), and a violation of "`task-service` never reads
`team_members` rows itself" in spirit even if not literally (it would be
doing the join itself, just over the wire instead of in SQL). This
solution adds `rpc ListTeamsForUser(ListTeamsForUserRequest) returns
(ListTeamsForUserResponse)` to `tenant-service`'s proto — a single indexed
query tenant-service is already positioned to serve directly,
`idx_team_members_user(user_id)` (`tenant-service.md:164`) exists
specifically, per that doc's own comment, "for cascade team-layer
resolution" — the identical access pattern this RPC needs. Flagged the
same way SOL-009 flagged its own proto extension to `git-gateway-service`
and BUG-TG-02's solution flags its `task --> git` edge: a scope addition
to `tenant-service`'s TDD, not something already specified there.

## Design — schema

`migrations/0004_grant_expiry_and_public_link.up.sql`:

```sql
ALTER TABLE task.task_grants
  ADD COLUMN expires_at TIMESTAMPTZ; -- NULL = never expires

CREATE INDEX idx_task_grants_expires ON task.task_grants (expires_at) WHERE expires_at IS NOT NULL;

-- Public/anonymous share-link flow. One row per active link; revocation is
-- a soft-delete (revoked_at set) so an audit trail survives, matching the
-- append-only posture 07-security-architecture.md's audit section requires
-- for security-relevant state changes generally.
CREATE TABLE task.task_share_links (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id   UUID NOT NULL,
    task_id     UUID NOT NULL REFERENCES task.tasks(id) ON DELETE CASCADE,
    token_hash  TEXT NOT NULL UNIQUE, -- SHA-256 of the random token, never the plaintext — same "hash, not plaintext" posture 07-security-architecture.md requires for the Dev Server Agent's own bearer token
    created_by  UUID NOT NULL,
    level       TEXT NOT NULL DEFAULT 'user' CHECK (level = 'user'), -- spec: anonymous access is always read-only; enforced by ONLY allowing the lowest non-team/company level, mapped to Rego's "read" action
    expires_at  TIMESTAMPTZ,
    revoked_at  TIMESTAMPTZ,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_task_share_links_task ON task.task_share_links (tenant_id, task_id) WHERE revoked_at IS NULL;

ALTER TABLE task.task_share_links ENABLE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON task.task_share_links
    USING (tenant_id = current_setting('app.tenant_id', true)::uuid);
```

`task_grants` already has an `id UUID PRIMARY KEY`
(`migrations/0001_init.up.sql:68`) but `domain.Grant`/`GrantRepository`
never surface it — needed for `RevokeGrant`'s target identifier (see
domain section).

## Design — domain

```go
// internal/domain/grant.go — additive
type Grant struct {
    ID        string // new — needed by RevokeGrant
    TaskID    string
    SubjectID string
    Level     GrantLevel
    ApplyTree bool
    ExpiresAt *time.Time // new — nil = never expires
}
```

### `ResolveGrant` — expiry filter + explicit `now`

Kept a pure function (no `time.Now()` call inside `domain/`, per the
same "no DB, no gRPC, no `context.Context`" discipline the existing doc
comment states, `grant_resolution.go:10-16`) — `now` becomes an explicit
parameter, making expiry testable without wall-clock flakiness:

```go
func ResolveGrant(ancestorChain []string, grantsByTask map[string][]Grant, caller CallerIdentity, maxDepth int, now time.Time) (GrantLevel, bool) {
    // ...unchanged walk structure...
    for _, g := range grantsByTask[taskID] {
        if depth > 0 && !g.ApplyTree { continue }
        if g.ExpiresAt != nil && !g.ExpiresAt.After(now) { continue } // new — spec's "ignore expired grants"
        if !g.Matches(caller) { continue }
        // ...unchanged priority comparison...
    }
}
```

Every existing call site (`resolve_permission.go:77`, plus SOL-TG-01's
`GetSubtree`) passes `time.Now()` from the usecase layer — the one place
per Clean Architecture allowed to know wall-clock time exists at all.

### Owner-intrinsic short-circuit

BUG-TG-03 correctly identifies that "owner always has full manage" isn't
representable today because `domain.Task` has no `OwnerID`.
[SOL-TG-01](./SOL-TG-01-task-graph-structural-management.md) adds
`Task.OwnerID`; this solution wires it into resolution **without changing
`ResolveGrant`'s signature further** — the owner check is a synthesized
grant, not a new algorithm branch, keeping the BFS walk itself
untouched:

```go
// internal/usecase/resolve_permission.go
func (uc *ResolvePermission) Execute(ctx context.Context, in ResolvePermissionInput) (domain.GrantLevel, error) {
    tenantID, err := tenant.RequireTenantID(ctx)
    // ...

    task, err := uc.tasks.Get(ctx, tenantID, in.TaskID) // new — needed for OwnerID; GetAncestors below still fetches the chain separately since it returns []Task including this task at index 0, but the usecase reads .OwnerID off the already-fetched chain[0] rather than a second Get call
    ancestors, err := uc.tasks.GetAncestors(ctx, tenantID, in.TaskID, uc.maxDepth)
    // ...

    grantsByTask, err := uc.grants.ListGrantsForAncestors(ctx, tenantID, chain)
    if ancestors[0].OwnerID == in.UserID && in.UserID != "" {
        // Synthesize an intrinsic Owner-level grant at the target task,
        // ApplyTree=true so it behaves identically to a real owner grant
        // for the whole subtree an owner would expect to manage — matches
        // spec's "owner always has full manage" without a stored row.
        grantsByTask[ancestors[0].ID] = append(grantsByTask[ancestors[0].ID], domain.Grant{
            TaskID: ancestors[0].ID, SubjectID: in.UserID, Level: domain.GrantLevelOwner, ApplyTree: true,
        })
    }

    teamIDs, err := uc.teams.ResolveTeams(ctx, tenantID, in.UserID)
    caller := domain.CallerIdentity{UserID: in.UserID, TeamIDs: teamIDs, CompanyID: tenantID}
    level, found := domain.ResolveGrant(chain, grantsByTask, caller, uc.maxDepth, uc.clock.Now())
    // ...unchanged OPA decision step...
}
```

**Admin (tenant-wide) short-circuit is explicitly deferred**, flagged
here rather than silently dropped: it requires a role concept
`task-service` doesn't have any port for today (`auth-service` owns
RBAC/roles per `02-microservices-decomposition.md:44`'s "admin console...
folded in — admin operations are auth/RBAC operations on the same data").
Closing this needs a new `RoleResolver` port dialed to `auth-service`,
analogous to `TeamScopeResolver`'s relationship to `tenant-service` —
out of scope for this solution, called out so it isn't mistaken for
"already fixed" once owner short-circuit lands.

## Design — `TeamScopeResolver` — real implementation

```go
// internal/adapter/grpcclient/team_scope_resolver.go
type TeamScopeResolver struct {
    tenant tenantv1.TenantServiceClient // dialed to tenant-service, cmd/server/main.go
}

func NewTeamScopeResolver(client tenantv1.TenantServiceClient) *TeamScopeResolver {
    return &TeamScopeResolver{tenant: client}
}

func (r *TeamScopeResolver) ResolveTeams(ctx context.Context, tenantID, userID string) ([]string, error) {
    if userID == "" {
        return nil, nil // anonymous/system callers have no team membership — not an error
    }
    resp, err := r.tenant.ListTeamsForUser(ctx, &tenantv1.ListTeamsForUserRequest{TenantId: tenantID, UserId: userID})
    if err != nil {
        return nil, fmt.Errorf("grpcclient: resolve team membership: %w", err)
    }
    return resp.GetTeamIds(), nil
}
```

`ResolvePermission.Execute` already treats a `TeamScopeResolver` error as
`apperrors.KindInternal` → fails the whole request
(`resolve_permission.go:71-74`) — this is the correct posture to keep: a
tenant-service outage must not silently degrade into "no team grants
apply," which would be a *more* permissive failure mode than intended for
some callers and a *less* permissive one for others depending on what
else the caller has — fail loud, not soft, matching §9's fail-closed
principle already applied to the OPA call two lines below it.

`cmd/server/main.go` gains a `tenant-service` dial (mirrors the existing
`infraFleetConn`/`aiProviderConn` pattern, `main.go:89-105`) and swaps
`taskgrpcclient.NewStubTeamScopeResolver()` for
`taskgrpcclient.NewTeamScopeResolver(tenantClient)`.

## Design — `RevokeGrant` / `ListGrants`

```go
// internal/usecase/revoke_grant.go
type RevokeGrant struct { grants GrantRepository; events EventPublisher }

func (uc *RevokeGrant) Execute(ctx context.Context, in RevokeGrantInput) error {
    tenantID, err := tenant.RequireTenantID(ctx)
    // Revoke requires 'manage' on the task — checked via ResolvePermission
    // itself before deleting, same "every mutating RPC calls
    // ResolvePermission internally first" rule task-service.md §3 states
    // (task-service.md:70-72) — Grant's own usecase doesn't do this today
    // either (see "Known gaps" below); RevokeGrant is new code so it's
    // built with the check from the start rather than inheriting the gap.
    if _, err := uc.resolvePermission.Execute(ctx, ResolvePermissionInput{TaskID: in.TaskID, UserID: in.CallerUserID, Action: "manage"}); err != nil {
        return err
    }
    if err := uc.grants.Revoke(ctx, tenantID, in.GrantID); err != nil { return ... }
    uc.events.Publish(ctx, "task.grant_revoked", ...) // best-effort outbox write, see notification design below
    return nil
}
```

`Grant.Execute` (`grant.go:30-50`) gains the identical `ResolvePermission`
pre-check — flagged as a **separate, real gap this solution also closes**:
today any authenticated caller can call `Grant` on any task ID they can
name, with zero access check (`grant.go` never calls
`ResolvePermission`) — a caller doesn't need to already have access to a
task to grant *themselves* `owner` on it. This is arguably a more urgent
correctness issue than the team-grant stub, surfaced by reading the
existing code while implementing this fix; both `Grant` and the new
`RevokeGrant` require `Action: "manage"` before writing.

`ListGrants` (new, read-only) requires `Action: "manage"` too (spec: "the
'manage' permission includes viewing/managing existing grants",
per BUG-TG-03's own citation) and returns the raw grant rows for the
target task only (not the whole ancestor chain — an Owner sees who has
access *to this task*, not the union of every ancestor's grants, which
would leak ancestor-task grant details to someone who may not have
visibility into the ancestor task itself).

## Design — public share-link flow

New table above; three new RPCs, kept minimal per spec ("anonymous
read-only access via a random token"):

```protobuf
rpc CreatePublicLink(CreatePublicLinkRequest) returns (CreatePublicLinkResponse); // requires 'manage'; returns the plaintext token ONCE
rpc RevokePublicLink(RevokePublicLinkRequest) returns (google.protobuf.Empty);
rpc ResolvePublicLink(ResolvePublicLinkRequest) returns (ResolvePublicLinkResponse); // token -> task_id + read-only grant, no auth required
```

`ResolvePublicLink` is the one RPC in this service meaningfully callable
without a JWT — `07-security-architecture.md`'s AuthN table
(`07-security-architecture.md:5-10`) has no row for this case today
(every listed client mechanism assumes an authenticated identity).
**Flagged as a genuine gap in that table**, not something this solution
resolves unilaterally: `api-gateway` needs an explicit unauthenticated
route class for share-link resolution (bypassing normal JWT validation
for exactly this one path, scoped to `GET /v1/tasks/share/{token}`) —
sketched here at the proto/table level; the `api-gateway` routing change
itself needs its own design pass against `07-security-architecture.md`'s
AuthN section before implementation, since it's a new trust boundary,
not a mechanical wiring change like the rest of this solution.

Token handling: `CreatePublicLink` generates a random 256-bit token,
returns the plaintext exactly once, stores only `SHA-256(token)` — same
posture as the Dev Server Agent's own bearer token
(`07-security-architecture.md:10`'s "hashed at rest... SHA-256 hash, not
plaintext"). `ResolvePublicLink` hashes the incoming token and looks up
by `token_hash`, checking `revoked_at IS NULL AND (expires_at IS NULL OR
expires_at > now())` before returning a synthesized read-only
`GrantLevelUser`-equivalent access decision — not a real `Grant` row (no
`subject_id` exists for an anonymous caller), so `ResolveGrant` isn't the
path this uses; it's a distinct, deliberately narrower code path that
never touches the BFS walk.

## Design — grant notification via outbox

Per `05-data-architecture.md`'s transactional-outbox pattern
(`05-data-architecture.md:82-98`) and `task-service.md §7`'s
already-declared `ts -.events.-> notif` edge
(`task-service.md:266`, "Emits events via the outbox pattern;
`notification-service` consumes them asynchronously"): `Grant.Execute`
writes a `task.grant_received` outbox row in the **same transaction** as
the grant insert (needs `Grant`'s repository call moved behind
`TxRunner`, mirroring `AIApply`'s existing pattern) rather than a
best-effort post-commit call, so a grant is never silently created
without its notification eventually firing:

```go
func (uc *Grant) Execute(ctx context.Context, in GrantInput) error {
    // ...validation, ResolvePermission pre-check...
    return uc.txRunner.RunInTx(ctx, func(ctx context.Context, tasks TaskRepository, edges EdgeRepository, grants GrantRepository, outbox OutboxWriter) error {
        if err := grants.Grant(ctx, tenantID, grant); err != nil { return ... }
        return outbox.Write(ctx, tenantID, "task.grant_received", map[string]any{
            "task_id": in.TaskID, "subject_id": in.SubjectID, "level": in.Level.String(), "granted_by": callerID,
        })
    })
}
```

`internal/adapter/eventbus/` (new package) polls unpublished outbox rows
and publishes to NATS JetStream — same "start with polling" guidance
`05-data-architecture.md:89` gives, mirroring `usage-service`'s existing
`adapter/eventbus/` package structure (cited in `task-service`'s own
README as the pattern to follow, `README.md:239-241`).
`notification-service` is the consumer; no `task-service`-side code
change is needed for the WebSocket push itself — that's
`notification-service`'s existing fan-out responsibility per
`02-microservices-decomposition.md`'s service catalog (#11).

## Design — `ResolvePermissionRequest.action` wire field

Closes the last "known gap" BUG-TG-03 names: add `string action = 3;` to
`ResolvePermissionRequest`, thread it through
`internal/adapter/grpc.Server.ResolvePermission`
(`server.go:117-132`, currently hardcodes `Action: "read"`) instead of a
constant. Every other RPC that internally calls `ResolvePermission`
(`Grant`, `RevokeGrant`, `GetSubtree`) passes its own real action
(`"manage"`, `"manage"`, `"read"` respectively) rather than the
placeholder — this closes the "deny-on-a-real-action path... not
reachable through the RPC surface yet" gap `README.md:227-236` names.

## Test plan

- `domain/grant_resolution_test.go` — expiry cases: an expired
  non-inherited grant on the target task is ignored; an expired
  `ApplyTree=true` ancestor grant is ignored but a non-expired one at
  the same depth still wins; `now` exactly equal to `expires_at` counts
  as expired (`!After`, not `!Before`) — explicit boundary test.
- `usecase/resolve_permission_test.go` — owner short-circuit: a caller
  matching `task.OwnerID` resolves `GrantLevelOwner` with zero rows in
  `grantsByTask`; a non-owner with a real `GrantLevelUser` grant still
  resolves that grant, not owner; owner short-circuit composes correctly
  with expiry (an owner is never "expired" — it's synthesized fresh every
  call, not a stored row a TTL could lapse).
- `adapter/grpcclient/team_scope_resolver_test.go` — fake
  `TenantServiceClient`: `ResolveTeams` returns the RPC's team IDs
  verbatim; an RPC error propagates as a wrapped error, not an empty
  list (regression guard against silently reintroducing the stub's
  always-empty behavior).
- `usecase/grant_test.go` — `Grant.Execute` denies when the caller has no
  `manage`-level access to the target task (new pre-check); the outbox
  row is written in the same transaction — a fake `TxRunner` that fails
  the outbox write asserts the grant insert itself also rolled back.
- `usecase/revoke_grant_test.go` — same `manage`-gate assertion;
  revoking a nonexistent grant ID is a `NOT_FOUND`, not a silent no-op.
- `adapter/postgres/grants_test.go` (integration) — `ListGrantsForAncestors`
  excludes expired rows at the SQL layer too (defense-in-depth, matching
  the domain-layer filter — belt-and-suspenders per
  `05-data-architecture.md`'s RLS-as-backstop philosophy applied to
  expiry the same way).
- Share-link: `ResolvePublicLink` on a revoked/expired token returns
  not-found, never a stale grant; token is stored and compared only as
  its SHA-256 hash — a test asserting `task_share_links` never contains
  the plaintext.

## References

- `docs/logic/task-graph/BL-TG-03-task-access-control.md` — full spec
- `specs/backend-go/tdd/services/task-service.md:98-153` (§4 domain
  model, §4.1 BFS walk algorithm), `:286-330` (§8 hot-path NFR, §9
  security notes — domain-computes/OPA-decides split, audit-event
  requirement)
- `specs/backend-go/tdd/services/tenant-service.md:79-86` (existing
  `team.*` RPC surface — the gap this solution's `ListTeamsForUser`
  addition closes), `:163-164` (`idx_team_members_user` index, already
  positioned for this query)
- `specs/backend-go/tdd/architecture/07-security-architecture.md:5-10`
  (AuthN table — the gap the public-link flow surfaces), `:24-52` (AuthZ
  / OPA-centralization rationale), `:68-80` (audit-logging requirement)
- `backend-go/services/task-service/internal/domain/grant.go:1-96`,
  `grant_resolution.go:1-67`
- `backend-go/services/task-service/internal/adapter/grpcclient/team_scope_resolver.go:11-26`
  (the stub this solution replaces)
- `backend-go/services/task-service/internal/usecase/resolve_permission.go:51-91`,
  `grant.go:30-50`
- `backend-go/services/task-service/internal/adapter/postgres/grants.go:1-75`
- `backend-go/services/task-service/cmd/server/main.go:86-89`
  (stub wired into the production composition root today)
- `backend-go/services/task-service/README.md:127-138,190-236` (known
  deviations + gaps this solution closes: no expiry, no revoke, first-cut
  permission matrix, no `action` field on the wire, no event publishing)
- `specs/backend-go/bugs/logic-v1/BUG-TG-01-task-graph-structural-management-partial.md`
  — `Task.OwnerID` this solution's owner short-circuit depends on (see
  [SOL-TG-01](./SOL-TG-01-task-graph-structural-management.md))
- `specs/backend-go/bugs/missing-v1/solutions/SOL-009-files-channels.md`
  — precedent for flagging a proto extension as a scope addition rather
  than silently assuming it
