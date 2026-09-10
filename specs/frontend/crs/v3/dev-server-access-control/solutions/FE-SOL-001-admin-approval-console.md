# FE-SOL-001: Admin Approval Console

**CR:** [CR-DS-006](../../../../../docs/crs/v2/dev-server/CR-DS-006-dev-server-approval-and-grouping.md)
**Backend:** [BE-SOL-002](../../../../backend-go/crs/v0/dev-server-access-control/solutions/BE-SOL-002-admin-approval-rpc.md)
**Status:** ✅ Completed — 2026-08-28
**Files:** `frontend/src/renderer/src/components/settings/AdminDevServerConsole.tsx` (new),
`frontend/src/renderer/src/components/settings/Settings.tsx` (+admin-gated section),
`frontend/src/renderer/src/hooks/useSettingsNavigationMetadata.ts` (+admin-gated nav entry,
new `isAdmin` param on `buildSettingsNavigationMetadata`)

---

## Shipped shape (differs slightly from the original sketch)

One `AdminDevServerConsole` component, gated behind `currentUser.role === 'admin'`
at both the Settings nav-registration level and the section-render level (so
it neither appears in the sidebar nor mounts for a non-admin). Uses shadcn
`Tabs` instead of separate always-visible "Pending approval"/"Approved"
tabs, covering the full requested scope in three tabs:

- **Approvals** — every dev server (`devServer.list()`), status badge,
  inline Approve/Reject for `pending_approval`, inline group-assign
  `Select` + "Save group" for `approved` servers (`devServerGroup.list()` +
  `devServer.assignGroup()`).
- **Groups & access** — create a group (`devServerGroup.create()`), and per
  group: existing grants as removable badges, plus a department picker +
  "Grant" button (`devServerGroup.grant()`/`.revoke()`). Team-based grants
  are not exposed (see FE-SOL-003's "known gap" note — no team-list RPC
  exists yet).
- **Access requests** — pending requests (`devServer.listPendingAccessRequests()`),
  resolved with Approve/Reject (`devServer.resolveAccessRequest()`), group
  name resolved by joining against the already-loaded group list (the
  request's own view doesn't carry a group name, only `devServerGroupId`).

## Goal

New Settings screen (admin-only, gated the same way existing admin-only
settings panes already are — reuse whatever role check `GeneralSupportSection`-
adjacent admin panes use) listing dev servers by status, with actions:

- **Pending approval** tab (default): Approve / Reject buttons per server.
- **Approved** tab: Assign-to-group control (a `Select` populated from
  `devServerGroup.list`, plus a "+ New group" affordance calling
  `devServerGroup.create`).

## Design notes (to revisit once BE-SOL-002 ships)

- Reuse `DevServerStatusBadge`-equivalent styling patterns already fixed
  this session for `DevServerStep.tsx` (shadcn `Select`/`Button`, vertical
  `space-y-*` layout per `docs/STYLEGUIDE.md`) — do not reintroduce the
  unstyled-BEM-class pattern found and worked around earlier in this file.
- No task breakdown yet — see this topic's tasks/README.md.
