# BUG-008: `project.list`/`project.create` fail with `PROJECT_PROFILE_RESOLVE_FAILED` for non-admin/lead roles — `TenantProfileResolver` never forwards tenant metadata

**Service:** `project-service`
**File:** `internal/adapter/grpcclient/profile_resolver.go` (`TenantProfileResolver.GetResolvedProfile`), triggered from `internal/usecase/list_projects.go` (`ListProjects.Execute`)
**Severity:** High — blocks project listing (and anything that lists projects as a precondition, e.g. create-project flows that re-list afterward) for every caller whose role is `developer` or unset; `admin`/`lead` callers are unaffected
**Symptom:**
```
rpc error: code = Internal desc = PROJECT_PROFILE_RESOLVE_FAILED: failed to resolve caller profile for visibility filtering
```
**Status:** ✅ Fixed (2026-09-14) — [SOL-008](./solutions/SOL-008-profile-resolver-forward-tenant-metadata.md), TASK-016 + TASK-017. Root cause confirmed by source inspection (CodeGraph/GitNexus), reported live on the `b15.openledger.vn` deployment while creating a project.

---

## Description

`ListProjects.Execute` (`internal/usecase/list_projects.go`) scopes visibility
by role: `admin`/`lead` get the full membership-scoped list unfiltered, but
`developer` (or an unknown/empty role — the safer, narrowest-filter branch)
must additionally filter by the caller's `fleet.allowedServerTags`, fetched
via:

```go
// list_projects.go
resolved, err := uc.profiles.GetResolvedProfile(ctx, tenantID, userID)
if err != nil {
    return ListProjectsOutput{}, apperrors.New(apperrors.KindInternal,
        "PROJECT_PROFILE_RESOLVE_FAILED",
        "failed to resolve caller profile for visibility filtering", err)
}
```

`uc.profiles` is `usecase.ProfileResolver`, implemented by
`TenantProfileResolver` (`internal/adapter/grpcclient/profile_resolver.go`),
which dials tenant-service's `GetResolvedProfile` RPC:

```go
// profile_resolver.go — current (buggy)
func (r *TenantProfileResolver) GetResolvedProfile(ctx context.Context, tenantID, userID string) (usecase.ResolvedProfileView, error) {
	resp, err := r.tenant.GetResolvedProfile(ctx, &tenantv1.GetResolvedProfileRequest{UserId: userID})
	if err != nil {
		return usecase.ResolvedProfileView{}, fmt.Errorf("grpcclient: tenant-service GetResolvedProfile: %w", err)
	}
	// ...
}
```

This calls `r.tenant.GetResolvedProfile` with the **inbound** `ctx` unchanged
— it never stamps the caller's tenant ID onto **outbound** gRPC metadata.

## Confirmed

- `internal/adapter/grpcclient/tenant_forwarding.go` already defines
  `withTenantMetadata(ctx) (context.Context, error)` in this exact package,
  whose own doc comment says: *"Without this, every outbound call from this
  package silently carries no tenant header, and the callee's own
  `TenantExtractionInterceptor` rejects it (e.g. workflow-service's
  `WORKFLOW_NO_TENANT`)."*
- The two other outbound-call sites in the same package —
  `workflow_execution_checker.go` and `task_execution_checker.go` — both call
  `withTenantMetadata(ctx)` before their RPC. `profile_resolver.go`'s
  `GetResolvedProfile` is the one call site in this package that does not.
- tenant-service's own `GetResolvedProfile` usecase
  (`services/tenant-service/internal/usecase/get_resolved_profile.go:34-38`)
  requires `tenant.RequireTenantID(ctx)` and fails closed
  (`Unauthenticated`/`TENANT_NO_TENANT`) when the inbound request carries no
  tenant metadata — exactly the shape a missing `withTenantMetadata` call
  produces.
- Deploy wiring is correct and not implicated: `deploy/dev/docker-compose.yml`
  sets `TENANT_SERVICE_ADDR: tenant-service:9090` for `project-service`, and
  `project-service` depends on `tenant-service` being started — tenant-service
  is reachable; this is a code-level metadata-forwarding gap, not an infra
  gap.
- `go build ./services/project-service/...` — unaffected either way (this is
  a Go compile-time-valid bug, only observable at runtime).
- `gitnexus impact({target: "GetResolvedProfile", direction: "upstream",
  file_path: ".../profile_resolver.go"})` → `risk: LOW`, blast radius
  confined to `ListProjects.Execute`'s visibility-filter branch (the only
  caller of `usecase.ProfileResolver.GetResolvedProfile` via
  `TenantProfileResolver`).

## Root Cause — CONFIRMED

`TenantProfileResolver.GetResolvedProfile` forwards the raw inbound `ctx` to
tenant-service's `GetResolvedProfile` RPC instead of calling this package's
own `withTenantMetadata(ctx)` first (as its sibling adapters in the same file
package already do). tenant-service's inbound `TenantExtractionInterceptor`
then has no tenant header to extract, `tenant.RequireTenantID(ctx)` inside
tenant-service's usecase fails, and the resulting error crosses the gRPC
boundary back into `project-service`, which wraps it as
`PROJECT_PROFILE_RESOLVE_FAILED`/`Internal`.

This only surfaces for `developer`/unset-role callers because `admin`/`lead`
never reach the `uc.profiles.GetResolvedProfile` call at all (early return in
`list_projects.go`) — this is the same shape of gap this directory's
cross-cutting note already calls out for BUG-001/BUG-006 ("N call sites must
each remember to call a shared helper").

See [SOL-008](./solutions/SOL-008-profile-resolver-forward-tenant-metadata.md)
for the fix design — implemented via TASK-016 (the one-call fix in
`profile_resolver.go`) + TASK-017 (regression tests, including a test proven
to fail against the pre-fix code — see that task's Status line).
