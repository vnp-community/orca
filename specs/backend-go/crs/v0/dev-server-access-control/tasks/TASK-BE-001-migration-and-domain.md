# TASK-BE-001: Migration + Domain Model

> **Status: ✅ COMPLETED** — 2026-08-28
> **Files modified/created:**
> - `backend-go/services/infra-fleet-service/migrations/0008_dev_server_approval_status_and_groups.{up,down}.sql`
> - `backend-go/services/infra-fleet-service/internal/domain/dev_server.go` (modified)
> - `backend-go/services/infra-fleet-service/internal/domain/dev_server_group.go` (new)

**Solution:** [BE-SOL-001](../solutions/BE-SOL-001-dev-server-status-and-groups.md) | **CR:** CR-DS-006
**Depends on:** _(none — base task)_

---

## Goal

Add the Postgres schema + Go domain types for dev-server approval status
and grouping, additive-only (no existing column/field renamed or removed).

## Steps

1. Migration `0008`: `infra.dev_server_groups` table (UUID id/tenant_id,
   self-referencing `parent_group_id`, RLS policy matching `0001_init`'s
   pattern) + `infra.dev_servers.approval_status`/`group_id` columns (approval_status, not status — see BE-SOL-001).
2. `domain.DevServerStatus` type (`pending_approval|approved|rejected`) +
   `Valid()`.
3. `domain.DevServer` gains `Status`/`GroupID` fields — `NewDevServer`'s
   signature stays unchanged, sets `Status: DevServerStatusPendingApproval`
   internally.
4. `domain.DevServerGroup` + `NewDevServerGroup` (new file, mirrors
   `dev_server.go`'s invariant-enforcing constructor pattern).

## Impact analysis note

`gitnexus_impact` on `NewDevServer`/`DevServer` returned **CRITICAL**
(thousands of hits across unrelated old-TS-backend files and other
backend-go services' own `main.go`/`run` functions) — verified as a
monorepo-wide symbol-name collision false positive by building all 17
backend-go services after the change (`go build ./...` per service, zero
errors). Keeping `NewDevServer`'s call signature unchanged is what makes
this verification meaningful — no caller needed to change, so "builds
clean everywhere" really does mean "nothing broke."

## Acceptance Criteria

- [x] Migration up/down runs clean.
- [x] `go build ./...` clean in infra-fleet-service AND all 16 other
      backend-go services.
- [x] `domain` package unit tests pass (see TASK-BE-003).
