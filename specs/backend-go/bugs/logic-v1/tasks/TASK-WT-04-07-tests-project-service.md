# TASK-WT-04-07: Tests for `base_ref` persistence in `project-service`

**From Solution:** SOL-WT-04
**Priority:** P1
**Service:** `project-service`
**File:** `backend-go/services/project-service/internal/domain/worktree_test.go`
**Depends on:** TASK-WT-04-02, TASK-WT-04-03
**Status:** `[ ]` TODO

---

## Context

Regression coverage for the `base_ref` backfill's `project-service` half, per [SOL-WT-04](../solutions/SOL-WT-04-so-sanh-worktree.md)'s Test plan.

## Changes to make

`backend-go/services/project-service/internal/domain/worktree_test.go` — add `TestNewWorktree_RoundTripsBaseRef`: construct with a non-empty `baseRef`, assert `wt.BaseRef` is a non-nil pointer to the same string; construct with `""`, assert `wt.BaseRef` is `nil` (matches `nonEmptyPtr`'s existing convention for every other optional field on this type).

`backend-go/services/project-service/internal/adapter/postgres/worktree_repository_test.go` — add:
- `TestRecordWorktreeCreated_PersistsAndReturnsBaseRef` (integration, against this package's existing test Postgres instance) — insert a worktree with a `BaseRef`, assert the returned `domain.Worktree.BaseRef` round-trips.
- `TestGetWorktree_ReturnsByID` — insert then `GetWorktree`, assert the row matches; assert `domain.ErrWorktreeNotFound` for an unknown id.
- `TestMigration0010_UpDown` — apply `0010_worktree_base_ref.up.sql` (or its actual assigned number, see [TASK-WT-04-01](./TASK-WT-04-01-schema-base-ref.md)) against a scratch schema, confirm `base_ref` column exists, apply `.down.sql`, confirm it's gone — per `05-data-architecture.md`'s migration CI requirement.

## Verify

```bash
cd /opt/repos/orca/backend-go
go test ./services/project-service/internal/domain/... ./services/project-service/internal/adapter/postgres/...
```

Expected: all cases pass.
