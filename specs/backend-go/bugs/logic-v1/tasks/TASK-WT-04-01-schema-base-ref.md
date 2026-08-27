# TASK-WT-04-01: Persist `base_ref` — migration + `project.proto` fields

**From Solution:** SOL-WT-04
**Priority:** P0 — every other task in this set depends on `base_ref` existing on the wire/schema
**Service:** `project-service`
**File:** `backend-go/services/project-service/migrations/0010_worktree_base_ref.up.sql`
**Depends on:** none
**Status:** `[ ]` TODO

---

## Context

[SOL-WT-04](../solutions/SOL-WT-04-so-sanh-worktree.md) resolves the part of [BUG-WT-04](../BUG-WT-04-so-sanh-worktree-partial.md) that blocks BR-WT-13/14: enforcing "same base branch"/"same base SHA" across compared worktrees needs each worktree's own `base_ref`, which is specified in `project-service.md` §5's schema sketch but was never migrated — `domain.Worktree` has no `BaseRef` field, `worktreeColumns` doesn't select it, and `git-gateway-service.CreateWorktree.Execute` receives `in.BaseRef` but never forwards it (`create_worktree.go:60`, confirmed in this task set's own research). **Note**: if [TASK-WT-01-06](./TASK-WT-01-06-project-service-outbox-event.md) already landed, `project-service`'s next migration number is `0011`, not `0010` — check `ls backend-go/services/project-service/migrations/` before naming this file and adjust to the next free number.

Field-number correction against the SOL's own sketch: the real `project.proto`'s `Worktree` message already uses fields 1-15 (`created_at_unix_ms = 15` is the highest, confirmed in this task set's research) and `RecordWorktreeCreatedRequest` already uses fields 1-11 — the SOL's suggested field numbers (12 and 6, respectively) collide with existing fields (`coordinator_handle`/`origin`). This task uses the next free field number on each message instead: `Worktree.base_ref = 16`, `RecordWorktreeCreatedRequest.base_ref = 12`.

## Changes to make

`backend-go/services/project-service/migrations/0010_worktree_base_ref.up.sql` (renumber if a newer migration already exists):

```sql
ALTER TABLE project.worktrees ADD COLUMN base_ref TEXT;
```

`backend-go/services/project-service/migrations/0010_worktree_base_ref.down.sql`:

```sql
ALTER TABLE project.worktrees DROP COLUMN base_ref;
```

`backend-go/proto/orca/project/v1/project.proto` — add to `RecordWorktreeCreatedRequest` (currently fields 1-11, `project.proto:272-290`):

```protobuf
message RecordWorktreeCreatedRequest {
  string project_id = 1;
  string repo_id = 2;
  string path = 3;
  string branch = 4;

  optional string parent_worktree_id = 5;
  optional string origin = 6;
  optional string capture_source = 7;
  optional string task_id = 8;
  optional string orchestration_run_id = 9;
  optional string coordinator_handle = 10;
  optional string created_by_terminal_handle = 11;
  optional string base_ref = 12; // NEW — the branch/tag/sha this worktree was created from
}
```

Add to `Worktree` (currently fields 1-15, `project.proto:250-270`):

```protobuf
message Worktree {
  string id = 1;
  string project_id = 2;
  string repo_id = 3;
  string path = 4;
  string branch = 5;
  bool active = 6;

  optional string parent_worktree_id = 7;
  optional string origin = 8;
  optional string capture_source = 9;
  optional string capture_confidence = 10;
  optional string task_id = 11;
  optional string orchestration_run_id = 12;
  optional string coordinator_handle = 13;
  optional string created_by_terminal_handle = 14;
  int64 created_at_unix_ms = 15;
  optional string base_ref = 16; // NEW
}

// NEW — single-worktree lookup, the same class of gap SOL-031 already
// flagged for GetRepo ("project.proto has no single-repo-by-id lookup RPC").
message GetWorktreeRequest { string worktree_id = 1; }
```

Add the RPC to `service ProjectService` (near `RecordWorktreeCreated`, `project.proto:38-40`):

```protobuf
  rpc GetWorktree(GetWorktreeRequest) returns (Worktree);
```

## Verify

```bash
cd /opt/repos/orca/backend-go
ls services/project-service/migrations/ | tail -5   # confirm the next free migration number before naming the .sql files
buf generate proto
buf breaking proto --against '.git#branch=main'
go build ./proto/...
```

Expected: clean build; `buf breaking` reports only additions (new optional fields, new message, new RPC — no renumbering of any existing field).
