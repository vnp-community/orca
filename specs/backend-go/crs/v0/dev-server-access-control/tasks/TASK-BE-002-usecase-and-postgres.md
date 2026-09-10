# TASK-BE-002: Usecase + Postgres Repository

> **Status: ✅ COMPLETED** — 2026-08-28
> **Files modified/created:**
> - `backend-go/services/infra-fleet-service/internal/usecase/ports.go` (modified — `DevServerGroupRepository`)
> - `backend-go/services/infra-fleet-service/internal/usecase/create_dev_server_group.go` (new)
> - `backend-go/services/infra-fleet-service/internal/usecase/list_dev_server_groups.go` (new)
> - `backend-go/services/infra-fleet-service/internal/adapter/postgres/repository.go` (modified — status/group_id read/write)
> - `backend-go/services/infra-fleet-service/internal/adapter/postgres/dev_server_group_repository.go` (new)

**Solution:** [BE-SOL-001](../solutions/BE-SOL-001-dev-server-status-and-groups.md) | **CR:** CR-DS-006
**Depends on:** TASK-BE-001

---

## Goal

Wire the new domain types through the usecase and persistence layers,
following this service's established DI/port conventions exactly (compare
`RegisterDevServer`/`Repository.Register` for the shape to match).

## Steps

1. `DevServerGroupRepository` port (`Create`, `List(tenantID)`) — split from
   `DevServerRepository` for the same reason `SshTargetRepository` is split
   (Go method-set name collision).
2. `CreateDevServerGroup`/`ListDevServerGroups` usecases — tenant from
   context, never request body.
3. `Repository.Register`/`Get`/`List`/`FindBySshTarget` extended to
   read/write `status`/`group_id` — UUID-typed `group_id` needs the same
   `NULLIF(..., '')::uuid` cast pattern `ssh_target_id` already uses (empty
   string → NULL, not an invalid-UUID error).
4. `DevServerGroupStore` (new file) implementing `DevServerGroupRepository`.

## Acceptance Criteria

- [x] `go build ./...` clean.
- [x] `go vet ./...` clean.
- [x] Existing `Repository`/usecase tests still pass unmodified (additive
      change, no existing test needed updating).
