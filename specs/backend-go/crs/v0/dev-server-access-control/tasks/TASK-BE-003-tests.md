# TASK-BE-003: Unit + Integration Tests

> **Status: ✅ COMPLETED** — 2026-08-28
> **Files created:**
> - `backend-go/services/infra-fleet-service/internal/domain/dev_server_group_test.go`
> - `backend-go/services/infra-fleet-service/internal/domain/dev_server_test.go` (modified — added cases)
> - `backend-go/services/infra-fleet-service/internal/usecase/create_dev_server_group_test.go`
> - `backend-go/services/infra-fleet-service/internal/usecase/list_dev_server_groups_test.go`
> - `backend-go/services/infra-fleet-service/internal/adapter/postgres/dev_server_group_repository_test.go` (integration)

**Solution:** [BE-SOL-001](../solutions/BE-SOL-001-dev-server-status-and-groups.md) | **CR:** CR-DS-006
**Depends on:** TASK-BE-001, TASK-BE-002

---

## Goal

Cover the new domain invariants, usecases, and the real Postgres round-trip
— specifically guarding against the "field silently dropped between the Go
struct and the SQL layer" bug class this same session found and fixed
elsewhere (`repo.list`'s `PROJECT_MEMBERSHIP_LOOKUP_FAILED` investigation) —
`TestRepository_RegisterAndGet_PersistsStatusAndGroupID` exists specifically
to catch a regression of that shape here.

## Results

- `go test ./...` (infra-fleet-service, unit): all pass.
- `go test -tags=integration ./...` (Testcontainers Postgres): all new
  tests pass reliably (re-ran the new tests in isolation and as part of the
  full suite, twice). One PRE-EXISTING, unrelated flake reproduced
  (`TestRepository_ResolveConnection_FoundAndNotFound` — a documented
  Testcontainers startup race also seen and root-caused in project-service's
  `setupPool` doc comment) — confirmed to pass in isolation, not caused by
  this change.

## Acceptance Criteria

- [x] `domain.NewDevServerGroup` invariants covered (empty tenant, empty
      name, valid root/child).
- [x] `domain.NewDevServer` defaults to `pending_approval` — regression
      guard.
- [x] `CreateDevServerGroup`/`ListDevServerGroups` usecase tests (tenant
      isolation, validation, repository-failure propagation).
- [x] Postgres integration: tenant-scoped list, parent/child round-trip,
      `DevServer.Status`/`GroupID` round-trip through a real database.
