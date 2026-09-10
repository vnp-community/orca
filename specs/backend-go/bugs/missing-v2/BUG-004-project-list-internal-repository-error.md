# BUG-004: `project.list` fails with an opaque internal repository error for the bootstrap admin

**Service:** `project-service`
**File:** `internal/usecase/list_projects.go`
**Severity:** Medium — blocks `project.list` for the bootstrap admin (and possibly any tenant with zero rows); exact root cause not diagnosable from the client alone
**Symptom:** `project.list`, called with a valid session and no params, returns:
```
Internal: PROJECT_LIST_FAILED: failed to list projects
```
**Status:** 🔴 Open, root cause confirmed — found live 2026-08-27 via `tests/client/rpc-catalog.spec.ts` against `172.20.2.39:6769`; see [SOL-004](./solutions/SOL-004-project-list-empty-pagetoken-uuid.md).

---

## Description

Unlike BUG-001 (missing identity attach) and BUG-003 (OPA eval), this
channel IS wired correctly — `channels_tenant_project.go:227` does attach
identity, and `list_projects.go` doesn't call `requireProjectAccess` at all
(it's a tenant-wide list, not a per-project one):

```go
// list_projects.go:29-43
func (uc *ListProjects) Execute(ctx context.Context, in ListProjectsInput) (ListProjectsOutput, error) {
	tenantID, err := tenant.RequireTenantID(ctx)
	if err != nil {
		return ListProjectsOutput{}, apperrors.New(apperrors.KindUnauthenticated, "PROJECT_NO_TENANT", "no tenant in request context", err)
	}
	// ... pageSize clamping ...
	projects, next, err := uc.repo.List(ctx, tenantID, in.PageToken, pageSize)
	if err != nil {
		return ListProjectsOutput{}, apperrors.New(apperrors.KindInternal, "PROJECT_LIST_FAILED", "failed to list projects", err)
	}
	return ListProjectsOutput{Projects: projects, NextPageToken: next}, nil
}
```

`tenant.RequireTenantID(ctx)` succeeds (confirmed — the error returned is
`PROJECT_LIST_FAILED`, not `PROJECT_NO_TENANT`), so the failure is inside
`uc.repo.List(ctx, tenantID, ...)` itself — a genuine repository/DB-layer
error, not an auth or wiring gap.

## Confirmed

- `services/project-service/internal/usecase/list_projects.go:29-43` — full
  `Execute` body; confirmed which branch fires by comparing against the
  `folderWorkspace.list` (`PROJECT_NO_TENANT`, BUG-001) and `repo.list`
  (`PROJECT_MEMBERSHIP_LOOKUP_FAILED`/`PROJECT_POLICY_EVAL_FAILED`, BUG-003)
  failure signatures — `project.list`'s is neither, so it's past both the
  tenant-context check and (there is no per-project authorization step in
  this usecase at all).
- Live-verified 2026-08-27 against `172.20.2.39:6769`, same admin session
  used for BUG-002/BUG-003: `project.list` with `{}` params reproduced
  `PROJECT_LIST_FAILED` consistently.

## Root Cause — CONFIRMED (updated after initial filing)

The wrapped `err` never crosses the gRPC→client boundary, but reading
`ProjectRepository.List`'s query directly resolves this without server logs:

```go
// internal/adapter/postgres/repository.go:74-84
func (r *Repository) List(ctx context.Context, tenantID, pageToken string, pageSize int32) ([]domain.Project, string, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT `+projectColumns+`
		FROM project.projects
		WHERE tenant_id = $1 AND id > $2
		ORDER BY id
		LIMIT $3
	`, tenantID, pageToken, pageSize)
	// ...
}
```

`id` is `UUID PRIMARY KEY` (`migrations/0001_init.up.sql:12`). `pageToken`
is bound straight into the `id > $2` comparison with no presence check.
`channels_tenant_project.go`'s `project.list` handler defaults `PageToken`
to Go's zero value (`""`) when the caller omits it —
[BUG-006](./BUG-006-wscompat-session-dialect-drops-null-params.md) makes
that the common case for a `WebSessionClient`-dialect caller not
explicitly paginating. Postgres then rejects the query outright:
`ERROR: invalid input syntax for type uuid: ""` — a real, generic SQL
error, exactly matching this bug's `Internal`/`PROJECT_LIST_FAILED`
classification (as opposed to `NotFound`, which is what a genuinely-empty
result set would never produce — this was already the stronger signal
pointing away from "empty tenant" and toward "malformed query", noted at
initial filing).

This has nothing to do with BUG-002's tenant-provisioning gap — first-page
`project.list` calls would fail this way for **any** tenant, populated or
not, any time `pageToken` is omitted.

See [SOL-004](./solutions/SOL-004-project-list-empty-pagetoken-uuid.md)
for the fix.
