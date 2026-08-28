# TASK-AT-04-04: `automation-service` — `worktree_cleanup_log` table + `WriteCleanupReport` RPC (BR-AT-14)

**From Solution:** SOL-AT-04
**Priority:** P1
**Service:** `automation-service`
**File:** `backend-go/services/automation-service/internal/adapter/postgres/` (+ migration), `backend-go/proto/orca/automation/v1/automation.proto`
**Depends on:** none
**Status:** `[ ]` TODO

---

## Context

`workflow-service`'s cleanup executor (TASK-AT-04-05) needs a place to
record one row per worktree per cleanup run — a real per-worktree,
per-reason audit trail (BR-AT-14), not just the aggregate counts already in
`automation_runs.output_json`. `automation-service` already owns run
bookkeeping, so this is a new reverse-direction gRPC call:
`workflow-service` → `automation-service`.

## Changes to make

Add a migration:

```sql
CREATE TABLE automation.worktree_cleanup_log (
  id             UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  tenant_id      UUID NOT NULL,
  run_id         UUID NOT NULL REFERENCES automation.automation_runs (id) ON DELETE CASCADE,
  worktree_id    TEXT NOT NULL,
  action         TEXT NOT NULL CHECK (action IN ('deleted','skipped','would_delete')),
  reason         TEXT,
  created_at     TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_worktree_cleanup_log_run ON automation.worktree_cleanup_log (run_id);
```

Add a new RPC to `automation.proto`:

```protobuf
rpc WriteCleanupReport(WriteCleanupReportRequest) returns (google.protobuf.Empty);

message WriteCleanupReportRequest {
  string run_id = 1;
  repeated CleanupLogEntry entries = 2;
}
message CleanupLogEntry {
  string worktree_id = 1;
  string action = 2; // "deleted" | "skipped" | "would_delete"
  string reason = 3;
}
```

Implement the gRPC handler and the corresponding repository method
`WriteCleanupReport(ctx, tenantID, runID string, entries []domain.CleanupLogEntry) error`
in `internal/adapter/postgres/` — a single batched multi-row `INSERT`.

## Verify

```bash
cd /opt/repos/orca/backend-go
buf generate proto
go build ./services/automation-service/...
go test ./services/automation-service/internal/adapter/postgres/... -run TestWriteCleanupReport
```

Expected: `WriteCleanupReport` round-trip — N entries in, N rows queryable
by `run_id`; `buf breaking` clean (additive RPC only).
