# TASK-WF-002-04: `infra-fleet-service` — add `PickByTag` RPC (blocking dependency of `TargetKindFleetTag`)

**From Solution:** BE-SOL-002 (indirectly — this task exists because [TASK-WF-002-01](./TASK-WF-002-01-target-spec-server-resolver.md) discovered a real blocker BE-SOL-002 flagged but didn't scope a fix for)
**Priority:** P1 — blocks `TargetKindFleetTag` only; `TargetKindProject`/`TargetKindServer` do not depend on this
**Service:** `infra-fleet-service`
**File:** `backend-go/proto/orca/infrafleet/v1/infrafleet.proto`, `backend-go/services/infra-fleet-service/internal/usecase/pick_by_tag.go` (new), `backend-go/services/infra-fleet-service/internal/adapter/grpc/server.go`
**Depends on:** None — can land independently, before or after [TASK-WF-002-01](./TASK-WF-002-01-target-spec-server-resolver.md)
**Status:** `[ ]` TODO

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
