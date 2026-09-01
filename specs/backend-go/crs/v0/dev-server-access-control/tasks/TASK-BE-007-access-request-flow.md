# TASK-BE-007: Access Request Flow

> **Status: ✅ COMPLETED** — 2026-08-28
> **Files created:** `internal/domain/dev_server_access_request.go`,
> `internal/adapter/postgres/dev_server_access_request_repository.go`,
> `internal/usecase/create_access_request.go`,
> `list_pending_access_requests.go`, `resolve_access_request.go`
> (+ `*_test.go` for each)

**Solution:** [BE-SOL-004](../solutions/BE-SOL-004-access-request-flow.md) | **CR:** CR-DS-008
**Depends on:** TASK-BE-006

---

## Goal

`infra.dev_server_access_requests` table + `CreateAccessRequest` (NOT
admin-gated) + `ListPendingAccessRequests`/`ResolveAccessRequest`
(admin-gated). Approving creates a `DevServerGroupGrant` for exactly the
`(GranteeKind, GranteeID)` the request captured at creation time (not
re-derived at resolve time — a department change after filing doesn't
retroactively change what gets granted).

## Acceptance Criteria

- [x] `CreateAccessRequest` requires tenant+user context, not admin.
- [x] `ResolveAccessRequest` approve path creates exactly one grant
      matching the request's captured grantee; reject path creates none.
- [x] Resolving an already-resolved request fails
      (`INFRA_ACCESS_REQUEST_ALREADY_RESOLVED`) — guards against
      double-granting.
- [x] Postgres integration test: create → get → list-pending →
      update-status round-trip, including `CreatedAtUnixMs` precision.
