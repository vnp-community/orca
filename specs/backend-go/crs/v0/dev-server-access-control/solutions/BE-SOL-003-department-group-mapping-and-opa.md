# BE-SOL-003: Department/Team ↔ Group Grants + OPA Policy (Phase 2)

**CR:** [CR-DS-007](../../../../../docs/crs/v2/dev-server/CR-DS-007-department-based-access-control.md)
**Service:** infra-fleet-service (grants) + policy/orca-authz (rego)
**Status:** ✅ Backend COMPLETED (2026-08-28) — implemented via defaults, no separate OPA rego (see CR-DS-007's updated §3/§4). Frontend pending.

---

## Scope (see CR-DS-007 §2 for full detail)

- New table `infra.dev_server_group_grants(id, tenant_id, dev_server_group_id, grantee_kind, grantee_id)` — logical FK to `tenant.departments`/`tenant.teams` (cross-service, no physical FK — same pattern `project-service` uses to validate `tenant_id` via `tenant-service.ValidateTenant`).
- New usecases: `GrantDevServerGroupAccess`, `RevokeDevServerGroupAccess`, `ListDevServersForUser` (resolves caller's department + teams via a new gRPC call to `tenant-service.GetResolvedProfile`, then filters).
- New `dev_server_access.rego` policy, modeled on `task_grant.rego`'s level-matrix pattern.

## Why this isn't started yet

CR-DS-007 §3 has 3 unresolved product decisions (ungrouped-server visibility, group-hierarchy grant inheritance, department-vs-team grant conflict resolution) that change the shape of `ListDevServersForUser`'s query and the rego policy's rule set. Writing code against unconfirmed semantics here risks a rewrite; task breakdown (`TASK-BE-*`) for this solution is deferred until those are answered.

## Acceptance Criteria

- [ ] (see CR-DS-007 §4 — unchanged, repeated here for solution-doc completeness)
- [ ] Migration `infra.dev_server_group_grants` clean.
- [ ] `ListDevServersForUser` correct per confirmed decision on all 3 open questions.
- [ ] `dev_server_access.rego` has `opa test` coverage comparable to `project_test.rego`.
