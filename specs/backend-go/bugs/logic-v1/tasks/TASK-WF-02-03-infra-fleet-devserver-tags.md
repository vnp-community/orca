# TASK-WF-02-03: Add `DevServer.tags` + `ListDevServersByTag` to `infra-fleet-service`

**From Solution:** SOL-WF-02
**Priority:** P1
**Service:** `infra-fleet-service`
**File:** `backend-go/proto/orca/infrafleet/v1/infrafleet.proto`
**Depends on:** none
**Status:** `[ ]` TODO

---

## ⚠️ Cross-team sign-off required

This is the one piece of SOL-WF-02 that touches a service other than
`workflow-service`'s own data model, and it is **not described in
`infra-fleet-service`'s own TDD document** — `infrafleet-service.md` was
not read as part of designing this change, and `DevServer` today has no
`tags` concept at all (`infrafleet.proto:117-124`: `id/tenant_id/host/
mode/ssh_target_id` only). Get explicit sign-off from whoever owns
`infra-fleet-service` before implementing this task — it adds new schema,
proto surface, and query semantics to a service this solution set does
not otherwise touch, and the shape below should be reconciled against
that service's own TDD/conventions before landing.

## Context

`workflow-service`'s `fleet:tag:<tag>` dispatch-target shape (TASK-WF-02-02,
TASK-WF-02-04) needs a way to list healthy dev servers carrying a given
tag. Nothing in `infra-fleet-service` supports tag-based lookup today.

## Changes to make

In `backend-go/proto/orca/infrafleet/v1/infrafleet.proto`, extend
`DevServer`:

```protobuf
message DevServer {
  // ... existing fields unchanged ...
  repeated string tags = 6; // free-form, tenant-scoped; e.g. "gpu", "region:us-east"
}

rpc ListDevServersByTag(ListDevServersByTagRequest) returns (ListDevServersByTagResponse);
message ListDevServersByTagRequest {
  string tag = 1;
  bool healthy_only = 2; // filters against GetFleetHealth's own reachability check
}
message ListDevServersByTagResponse {
  repeated DevServer dev_servers = 1;
}
```

Create a new migration in
`backend-go/services/infra-fleet-service/migrations/` (next sequential
number after the existing `0006_browser_profiles`, i.e. `0007_dev_server_tags`):

```sql
-- 0007_dev_server_tags.up.sql
ALTER TABLE dev_servers ADD COLUMN tags TEXT[] NOT NULL DEFAULT '{}';
CREATE INDEX idx_dev_servers_tags ON dev_servers USING GIN (tags);
```

```sql
-- 0007_dev_server_tags.down.sql
DROP INDEX IF EXISTS idx_dev_servers_tags;
ALTER TABLE dev_servers DROP COLUMN tags;
```

Implement `ListDevServersByTag`'s usecase joining against the existing
health-check mechanism `GetFleetHealth` already uses (`DevServerHealth.reachable`)
to implement `healthy_only` — reuse that existing health-tracking, do not
add a new one.

## Regenerate stubs

```bash
cd /opt/repos/orca/backend-go
buf generate proto
buf breaking proto --against '.git#branch=main'
```

## Verify

```bash
cd /opt/repos/orca/backend-go
go build ./services/infra-fleet-service/...
go test ./services/infra-fleet-service/... -run TestListDevServersByTag
```

Expected: clean build; `buf breaking` clean; `ListDevServersByTag` with
`healthy_only=true` excludes unreachable servers, tag filtering matches
only servers carrying the exact tag.
