# TASK-PI-02-06: `issue_status_sync_enabled` migration + read path on `project-service`

**From Solution:** SOL-PI-02 (BR-PI-06 durable opt-out; also the field SOL-PI-03's consumer belt-and-braces-checks — see `TASK-PI-03-07`)
**Priority:** P0
**Service:** `project-service`
**File:** `backend-go/services/project-service/migrations/0010_issue_status_sync_enabled.up.sql` (new), `backend-go/services/project-service/internal/domain/project.go`, `backend-go/services/project-service/internal/adapter/postgres/project_repository.go`, `backend-go/services/project-service/internal/adapter/grpc/server.go`
**Depends on:** TASK-PI-02-02
**Status:** `[ ]` TODO

---

## Context

BR-PI-06 requires the sync-on-worktree-creation behavior to be durably
disableable per project, not just skippable per-call
(`CreateWorktreeFromIssueRequest.skip_status_update` already covers the
per-call case, TASK-PI-02-01). This adds the persisted column plus the read
path `git-gateway-service`'s `ProjectClient.IsIssueStatusSyncEnabled`
(TASK-PI-02-05) calls, and the write path via `UpdateProject`'s existing
field mask — the same "extend the mask" pattern `RebindDevServer` already
established for `dev_server_id`.

## Changes to make

`migrations/0010_issue_status_sync_enabled.up.sql` (new, next number after
`0009_worktree_lineage`):

```sql
ALTER TABLE project.projects ADD COLUMN issue_status_sync_enabled BOOLEAN NOT NULL DEFAULT true;
```

Corresponding `.down.sql`: `ALTER TABLE project.projects DROP COLUMN issue_status_sync_enabled;`

`internal/domain/project.go` — add `IssueStatusSyncEnabled bool` to the
`Project` struct, defaulting to `true` in `NewProject`.

`internal/adapter/postgres/project_repository.go` — add
`issue_status_sync_enabled` to every `SELECT`/`INSERT`/scan touching
`project.projects` (mirror how `default_branch`/`visibility` are already
threaded through that file).

`internal/adapter/grpc/server.go` — in `GetProject`/`ListProjects`, map
`domain.Project.IssueStatusSyncEnabled` onto the new
`Project.issue_status_sync_enabled` proto field (TASK-PI-02-02). In
`UpdateProject`'s field-mask handling, add `issue_status_sync_enabled` as a
maskable field, following the exact single-path pattern
`RebindDevServer`'s `dev_server_id` carve-out already uses in this file.

## Verify

```bash
cd /opt/repos/orca/backend-go
go build ./services/project-service/...
go vet ./services/project-service/...
```
