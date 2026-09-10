# TASK-PI-02-02: Add `issue_status_sync_enabled` + `linked_issue_provider`/`linked_issue_ref` to `project.proto`

**From Solution:** SOL-PI-02 (shared with SOL-PI-03 — that solution's `WorktreeLifecycleEvent` publisher reads these same fields)
**Priority:** P0
**Service:** `project-service`
**File:** `backend-go/proto/orca/project/v1/project.proto`
**Depends on:** none
**Status:** `[x] DONE — Worktree/RecordWorktreeCreatedRequest linked_issue_provider/ref + Project.issue_status_sync_enabled added to project.proto.`

---

## Context

BR-PI-06 needs a durable per-project opt-out (`Project.issue_status_sync_enabled`)
and `Worktree`/`RecordWorktreeCreatedRequest` need to carry the linked-issue
reference this saga resolves, so `project-service` can persist it and (per
SOL-PI-03) publish `worktree.created` with the link attached in the same
transaction as `RecordWorktreeCreated`. **Cross-SOL note**: SOL-PI-03's
`TASK-PI-03-01` (proto for the outbox event payloads) and `TASK-PI-03-03`
(publish wiring) both depend on this task's `linked_issue_provider`/
`linked_issue_ref` fields existing first.

## Changes to make

In `message Worktree { ... }` (currently ends at field 15,
`created_at_unix_ms`), add:

```protobuf
  optional string linked_issue_provider = 16;  // "github" | "gitlab" | "jira" | "linear"
  optional string linked_issue_ref = 17;       // provider-native ref: "owner/repo#123" or "ENG-123"
```

In `message RecordWorktreeCreatedRequest { ... }` (currently ends at field
11, `created_by_terminal_handle`), add:

```protobuf
  optional string linked_issue_provider = 12;
  optional string linked_issue_ref = 13;
```

In `message Project { ... }` (currently ends at field 10, `updated_at`), add:

```protobuf
  bool issue_status_sync_enabled = 11;  // BR-PI-07/BR-PI-06, default true
```

## Regenerate stubs

```bash
cd /opt/repos/orca/backend-go
buf generate proto
buf breaking proto --against '.git#branch=main'
```

## Verify

```bash
cd /opt/repos/orca/backend-go
go build ./proto/...
```

Expected: clean build, `buf breaking` reports no breaking changes (only additions).
