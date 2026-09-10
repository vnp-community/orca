# TASK-BE-006: Department/Team ↔ Group Grants + ListDevServersForUser

> **Status: ✅ COMPLETED** — 2026-08-28
> **Files created:** `internal/domain/dev_server_group_grant.go`,
> `internal/adapter/postgres/dev_server_group_grant_repository.go`,
> `internal/usecase/grant_dev_server_group_access.go`,
> `revoke_dev_server_group_access.go`, `list_dev_server_group_grants.go`,
> `list_dev_servers_for_user.go` (+ `*_test.go` for each);
> migration `0009_dev_server_group_grants_and_access_requests.{up,down}.sql`
> (shared with TASK-BE-007 — one migration, two tables)

**Solution:** [BE-SOL-003](../solutions/BE-SOL-003-department-group-mapping-and-opa.md) | **CR:** CR-DS-007
**Depends on:** TASK-BE-005

---

## Goal

`infra.dev_server_group_grants` table + `Grant`/`Revoke`/`ListGrants`
usecases (admin-gated) + `ListDevServersForUser` (NOT admin-gated — the
department/team-filtered view every regular user calls).

## Key logic: `ListDevServersForUser`

Composes `DevServerRepository.List` + `DevServerGroupRepository.List` +
`DevServerGroupGrantRepository.ListAll` in Go (not a single complex SQL
query, not OPA) — filters to `approval_status = approved`, excludes
ungrouped dev servers, walks each dev server's group's ancestor chain
(cycle-guarded) checking for a grant matching the caller's department OR
any of their teams.

## Acceptance Criteria

- [x] Grant/Revoke/ListGrants admin-gated, tested.
- [x] `ListDevServersForUser`: ungrouped excluded, non-approved excluded,
      direct department grant matches, direct team grant matches, grant
      inherited from parent group matches, no-matching-grant excluded — 6
      dedicated tests, all passing.
- [x] Postgres integration tests for the grant store (create/list/delete,
      tenant-scoped) — Testcontainers, passing.
