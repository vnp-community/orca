# BE-SOL-004: Dev Server Access Request Flow (Phase 3)

**CR:** [CR-DS-008](../../../../../docs/crs/v2/dev-server/CR-DS-008-first-login-department-gate-and-access-request.md)
**Service:** infra-fleet-service
**Status:** ✅ Backend COMPLETED (2026-08-28) — usecases + wscompat channels + tests done. Frontend (access-request form, first-login gate) pending.

---

## Scope (see CR-DS-008 §2.3)

- New table `infra.dev_server_access_requests(id, tenant_id, user_id, dev_server_group_id, status, message, created_at, resolved_at, resolved_by)`.
- New usecases: `CreateAccessRequest`, `ListPendingAccessRequests` (admin), `ResolveAccessRequest` (approve → calls BE-SOL-003's `GrantDevServerGroupAccess` for the user's department/team; reject → status update only, no grant).
- New RPC + wscompat channel for each.
- Frontend: request form (user side) + pending-requests list (admin side, can share UI real estate with BE-SOL-002's approval console).

## Why this isn't started yet

Same reasoning as BE-SOL-003 — CR-DS-008 §3 has open decisions (does admin get gated too, is a request scoped to one group or an open-ended "please review my access", is notification in scope). Also structurally depends on BE-SOL-002/003 existing first (a request has nowhere to resolve to without them).

## Acceptance Criteria

(see CR-DS-008 §4)
