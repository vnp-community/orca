# BE-SOL-002: Admin Approval RPC + Group Assignment (Phase 2)

**CR:** [CR-DS-006](../../../../../docs/crs/v2/dev-server/CR-DS-006-dev-server-approval-and-grouping.md) §3 (remaining items)
**Service:** infra-fleet-service + api-gateway (wscompat)
**Status:** ✅ Backend COMPLETED (2026-08-28) — admin-gating wired via common/tenant.Role (new plumbing, see this pass), usecases + wscompat channels done, tests pass. Frontend (FE-SOL-001) pending.

---

## Scope

- New proto RPCs: `ApproveDevServer(devServerId)`, `RejectDevServer(devServerId, reason)`, `AssignDevServerGroup(devServerId, groupId)`, `CreateDevServerGroup`/`ListDevServerGroups` (already have usecases from BE-SOL-001, just need proto + gRPC handler + wscompat wiring).
- New usecases: `ApproveDevServer`, `RejectDevServer`, `AssignDevServerGroup` — all admin-gated (reuse `requireAdminActor`-equivalent pattern already established in auth-service/project-service, see `project-service/internal/usecase/authorization.go`).
- New wscompat channels: `devServer.approve`, `devServer.reject`, `devServer.assignGroup`, `devServerGroup.create`, `devServerGroup.list`.
- New admin UI screen (see [FE-SOL-001](../../../../frontend/crs/v3/dev-server-access-control/solutions/FE-SOL-001-admin-approval-console.md)) listing `status = pending_approval` dev servers with Approve/Reject/Assign-group actions.

## Open questions before implementation

1. Does `RegisterDevServer`'s caller (today: any authenticated tenant user) need to change now that new servers start `pending_approval`? **No functional change needed in this phase** — status still isn't enforced, so a pending server works exactly like an approved one until CR-DS-007's access-control gate lands. Confirm this sequencing is acceptable (ship grouping/approval UI before enforcement) rather than enforcing immediately.
2. Where does an already-`approved` dev server (migrated from before 0008) show up in the new admin approval list — should the admin console default to hiding approved servers, or show all with a status filter? (Recommend: status filter, default to `pending_approval`.)

## Acceptance Criteria

- [ ] Admin can see every `pending_approval` dev server for their tenant.
- [ ] Admin can approve/reject; rejected servers are NOT enforced-blocked yet (Phase 2 stays data-only for reject too — actual enforcement is CR-DS-007).
- [ ] Admin can create a group and assign a dev server to it.
- [ ] Non-admin caller gets `PermissionDenied` from all four new mutating RPCs.
