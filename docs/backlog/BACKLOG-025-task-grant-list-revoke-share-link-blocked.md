# BACKLOG-025: Task Grant modal's list/revoke/share-link — blocked on unbuilt backend RPCs

**Origin:** `specs/frontend/crs/v4/task-graph/tasks/FE-TASK-004-task-grant-modal-access-tab.md` (FE-SOL-001, CR-TG-007)
**Priority:** Low-Medium — the core Access-tab flow (grant a user, see permission badge) works today; only the view/manage-existing-grants part is missing
**Blocked on:** `BE-SOL-003` (`task-access-control-team-scope-and-sharing`, backend-go, **📋 Proposed**) — `task.listGrants`/`task.revokeGrant`/`task.generateShareLink` don't exist at any layer
**Owner:** whoever owns backend-go's task-service access-control roadmap

---

## What this is

`TaskGrantModal` (built 2026-09-09) has a working Access tab: adding a
grant (`task.grant`) and showing the current effective permission badge
(`task.resolvePermission`) both work against real, existing RPCs on the
real `GrantLevel` scale (`owner/admin/user/team/company`).

## Why part of it is mocked

`task.listGrants`, `task.revokeGrant`, and `task.generateShareLink` don't
exist at any layer (proto, usecase, or gRPC) — confirmed against
`BE-SOL-003`'s own status line. The UI shows a clear "coming soon, pending
BE-SOL-003" message for these rather than faking data or silently no-op'ing
— a user adding a grant today has no way to see the list of existing grants,
revoke one, or generate a share link.

## What unblocks this

`BE-SOL-003` ships its `ListGrants`/`RevokeGrant`/`GenerateShareLink` RPCs.
Once they exist, wiring them into `TaskGrantModal` is a small, additive
change — the modal's structure and the mocked call sites are already in
place, just swap the "coming soon" toast for a real call.
