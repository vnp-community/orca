# TASK-WF-002-04: `infra-fleet-service` — add `PickByTag` RPC (blocking dependency of `TargetKindFleetTag`)

**From Solution:** BE-SOL-002 (indirectly — this task exists because [TASK-WF-002-01](./TASK-WF-002-01-target-spec-server-resolver.md) discovered a real blocker BE-SOL-002 flagged but didn't scope a fix for)
**Priority:** P1 — blocks `TargetKindFleetTag` only; `TargetKindProject`/`TargetKindServer` do not depend on this
**Service:** `infra-fleet-service`
**File:** `backend-go/proto/orca/infrafleet/v1/infrafleet.proto`, `backend-go/services/infra-fleet-service/internal/usecase/pick_by_tag.go` (new), `backend-go/services/infra-fleet-service/internal/adapter/grpc/server.go`
**Depends on:** None — can land independently, before or after [TASK-WF-002-01](./TASK-WF-002-01-target-spec-server-resolver.md)
**Status:** `[x]` DONE

## Execution notes (2026-09-09)

Re-verified live: no `PickByTag` RPC, no selection usecase under another
name — confirmed. Also confirmed a real-code gap the task's own sketch
glossed over: **there is no "tag" concept in this service at all.**
`internal/usecase/pick_by_tag.go`'s sketch assumed a `s.ConnectionID`/
`s.Connected` shape directly on a "server" result that doesn't exist in
the real domain. The real primitives are:
- `domain.DevServerGroup{ID, TenantID, Name, ParentGroupID}` — no `Tag`
  field; **`tag` is mapped onto `DevServerGroup.Name`**, the closest real
  grouping concept.
- `domain.DevServer.GroupID` — the real membership field (not a separate
  tags table/column).
- Connectivity is `DevServerAgentClient.IsConnected(devServerID) bool`
  (`is_dev_server_connected.go`'s pattern), not a `Connected` field on the
  server struct.
- A "connection id" Relay's `connectionId` param actually expects is a
  `domain.Connection` row (worktree/repo binding), resolved via
  `ConnectionResolver.ResolveConnectionByDevServer(ctx, tenantID,
  devServerID)` — not the bare `DevServer.ID`.

Implemented `usecase.PickByTag` accordingly: list tenant's groups, match
`Name == tag` → `groupID`; list tenant's dev servers, filter by
`GroupID == groupID`; for the first one with `agent.IsConnected(id) ==
true`, resolve its connection via `ResolveConnectionByDevServer` and
return that connection's `ID`. Naive "first match" selection, per the
task's own explicit scope limit (load-balancing algorithm deferred).

**Changes made:**
1. `infrafleet.proto`: added `PickByTag` RPC + `PickByTagRequest`
   (`tag` only, tenant from context per `ListDevServerGroupsRequest`'s
   convention) / `PickByTagResponse` (`connection_id`); regenerated via
   `buf generate`.
2. `internal/usecase/pick_by_tag.go` (new): as described above, reusing
   `DevServerGroupRepository`, `DevServerRepository`, `ConnectionResolver`,
   `DevServerAgentClient` — no new port interfaces needed, all four
   already existed.
3. `internal/adapter/grpc/server.go`: added `pickByTag *usecase.PickByTag`
   field + constructor param (appended at the end, matching
   `teardownConnection`'s precedent of appending rather than reordering
   the 40+ existing positional params) + the `PickByTag` handler.
4. `cmd/server/main.go`: wired `usecase.NewPickByTag(devServerGroupStore,
   repo, repo, agentClient)` — `repo` satisfies both
   `DevServerRepository` and `ConnectionResolver` (same combined
   repository every other usecase already passes as both).
5. Rebuilt every cross-service consumer of
   `infrafleetv1.InfraFleetServiceClient` (api-gateway, workflow-service,
   project-service, git-gateway-service, task-service,
   ai-provider-service) — all embed the interface in their test fakes
   (never implement it standalone), so the new RPC didn't break any of
   them; confirmed via a full rebuild of all six.

**Verify output:**
```
go build ./services/infra-fleet-service/...   # clean
go vet   ./services/infra-fleet-service/...   # clean
go test  ./services/infra-fleet-service/internal/usecase/... -run TestPickByTag -v
  # 6/6 PASS (tenant/tag validation, no-matching-group, first-connected-member
  #           selection, no-connected-member, group-list-error-propagates)
go test  ./services/infra-fleet-service/...   # full suite, all ok
go build ./services/api-gateway/... ./services/workflow-service/... \
         ./services/project-service/... ./services/git-gateway-service/... \
         ./services/task-service/... ./services/ai-provider-service/...   # all clean
```

## Unblocks

TASK-WF-002-01's `TargetKindFleetTag` branch can now call the real
`PickByTag` RPC instead of returning the "not yet supported" stub.

---

## Context

Discovered while implementing [TASK-WF-002-01](./TASK-WF-002-01-target-spec-server-resolver.md):
`infra-fleet-service`'s real `infrafleet.proto` has **no `PickByTag` RPC**
and **no equivalent usecase under a different name** — confirmed directly,
`internal/usecase/` only has group-**management** primitives
(`assign_dev_server_group.go`, `create_dev_server_group.go`,
`list_dev_server_groups.go`, `grant_dev_server_group_access.go`,
`revoke_dev_server_group_access.go`, `list_dev_server_group_grants.go`) —
none of these select/load-balance across a group's members. This is a real
gap in `infra-fleet-service` itself, not scope creep from the workflow
series — it was never tracked as its own task until now.

**Before starting:** check whether `infra-fleet-service`'s own CR/task
backlog (outside `docs/crs/v4/`) already has this RPC planned, to avoid a
duplicate proto change landing from two directions.

## Changes to make

```protobuf
// infrafleet.proto — near the other ListDevServerGroup* RPCs
rpc PickByTag(PickByTagRequest) returns (PickByTagResponse);

message PickByTagRequest {
  string tenant_id = 1;
  string tag = 2; // matches whatever tagging/grouping concept ListDevServerGroups already uses — verify the real group-membership field name before wiring the query
}
message PickByTagResponse {
  string connection_id = 1;
}
```

```go
// internal/usecase/pick_by_tag.go (new)
// Minimal correctness first: any live, connected server carrying the tag.
// Load-balancing algorithm (round-robin, least-loaded, etc.) is explicitly
// NOT this task's scope — BE-SOL-002 already deferred that choice, and
// this task keeps deferring it. A correct-but-naive "first match" is an
// acceptable v1.
func (uc *PickByTag) Execute(ctx context.Context, tenantID, tag string) (string, error) {
    servers, err := uc.repo.ListByTag(ctx, tenantID, tag) // reuse whatever query backs ListDevServerGroups' membership lookup
    if err != nil { return "", err }
    for _, s := range servers {
        if s.Connected { return s.ConnectionID, nil }
    }
    return "", apperrors.New(apperrors.KindFailedPrecondition, "INFRAFLEET_NO_SERVER_FOR_TAG", "no connected server found for this tag", nil)
}
```

## Verify

```bash
cd /opt/repos/orca/backend-go
go build ./services/infra-fleet-service/...
go test ./services/infra-fleet-service/internal/usecase/... -run TestPickByTag -v
```

Expected: a tag with ≥1 connected server returns a `connection_id`; a tag
with zero connected servers (or unknown tag) returns
`INFRAFLEET_NO_SERVER_FOR_TAG`, not a panic or an empty-string success.

## Unblocks

[TASK-WF-002-01](./TASK-WF-002-01-target-spec-server-resolver.md)'s
`TargetKindFleetTag` branch — until this lands, that branch should return a
clear "not yet supported" error rather than being blocked entirely (per
that task's own note).
