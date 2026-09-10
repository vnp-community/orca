# TASK-BE-005: Admin Approval + Group Assignment

> **Status: ✅ COMPLETED** — 2026-08-28
> **Files created:** `internal/usecase/authorization.go` (`requireAdmin`),
> `approve_dev_server.go`, `reject_dev_server.go`, `assign_dev_server_group.go`
> (+ `*_test.go` for each)
> **Files modified:** `internal/usecase/ports.go` (`DevServerRepository`
> gains `UpdateApprovalStatus`/`AssignGroup`), `internal/adapter/postgres/repository.go`
> (implements both), `create_dev_server_group.go` (now admin-gated, was
> ungated in Phase 1 since Role propagation didn't exist yet)

**Solution:** [BE-SOL-002](../solutions/BE-SOL-002-admin-approval-rpc.md) | **CR:** CR-DS-006 Phase 2
**Depends on:** TASK-BE-001/002/003 (Phase 1), TASK-BE-004 (Role plumbing)

---

## Goal

`ApproveDevServer`/`RejectDevServer`/`AssignDevServerGroup` usecases —
admin-gated via `requireAdmin(ctx)` (reads `tenant.Role(ctx)`, fails closed
on anything but exactly `"admin"`).

## Acceptance Criteria

- [x] All three usecases deny a non-admin/role-absent caller.
- [x] `Repository.UpdateApprovalStatus`/`AssignGroup` — real Postgres
      `UPDATE ... RETURNING`, tested against Testcontainers Postgres
      (`TestRepository_UpdateApprovalStatus_And_AssignGroup`).
- [x] `CreateDevServerGroup` (Phase 1, previously ungated) now also
      requires admin — existing Phase 1 tests updated to use a new
      `withAdminTenant` test helper; added
      `TestCreateDevServerGroup_RequiresAdmin` regression guard.
