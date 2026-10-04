# SOL-008: Fix BUG-008 — `TenantProfileResolver.GetResolvedProfile` must forward tenant metadata via `withTenantMetadata`

**Resolves:** BUG-008
**Service:** `project-service`
**Affected files:** `internal/adapter/grpcclient/profile_resolver.go` (`TenantProfileResolver.GetResolvedProfile`)
**Priority:** High
**Status:** ✅ IMPLEMENTED (2026-09-14) — TASK-016 + TASK-017

---

## Grounding

This package (`internal/adapter/grpcclient`) already documents and implements
the required pattern in `tenant_forwarding.go`:

> `withTenantMetadata` stamps the caller's already-validated tenant ID onto
> `ctx` as outbound gRPC metadata, using the same key every service's inbound
> `grpcmw.TenantExtractionInterceptor` reads... Without this, every outbound
> call from this package silently carries no tenant header, and the callee's
> own `TenantExtractionInterceptor` rejects it.

`workflow_execution_checker.go` and `task_execution_checker.go` — the other
two outbound-call sites in this same package — already follow this pattern.
`profile_resolver.go`'s `GetResolvedProfile` is the sole outlier; the fix is
to bring it in line with its siblings, not to invent a new mechanism.

## Design

```go
// internal/adapter/grpcclient/profile_resolver.go — sketch
func (r *TenantProfileResolver) GetResolvedProfile(ctx context.Context, tenantID, userID string) (usecase.ResolvedProfileView, error) {
	// tenant-service's GetResolvedProfile usecase calls tenant.RequireTenantID
	// against its own inbound-interceptor-populated context — forward the
	// caller's tenant as outbound metadata, same as every other call this
	// package makes (see tenant_forwarding.go's doc comment).
	outCtx, err := withTenantMetadata(ctx)
	if err != nil {
		return usecase.ResolvedProfileView{}, fmt.Errorf("grpcclient: profile resolver: %w", err)
	}
	resp, err := r.tenant.GetResolvedProfile(outCtx, &tenantv1.GetResolvedProfileRequest{UserId: userID})
	if err != nil {
		return usecase.ResolvedProfileView{}, fmt.Errorf("grpcclient: tenant-service GetResolvedProfile: %w", err)
	}
	// ... unchanged from here (unmarshal resolved_settings_json, build ResolvedProfileView) ...
}
```

No signature change, no new imports (`withTenantMetadata` is already defined
in this package's `tenant_forwarding.go`). `DevServerTags` (the other method
on `TenantProfileResolver`, calling `infra-fleet-service.ListDevServers`) is
unaffected by this fix and out of scope — it dials a different downstream
service and was not part of this bug's reproduction; worth a separate,
explicit check for the same gap before closing this file's own "cross-cutting
observations" style follow-up, not silently folded into this task.

### Why not fix it inside `ListProjects.Execute` instead

The tenant ID is already available in `ctx` (that's exactly what
`tenant.RequireTenantID` inside tenant-service's own usecase expects to read
back out) — the gap is purely "this adapter didn't stamp it as outbound
metadata before making the call," which is `TenantProfileResolver`'s
responsibility as the adapter owning the outbound RPC, not the usecase's.
Fixing it in `list_projects.go` would require reaching into
transport-layer concerns (gRPC metadata) from usecase code, breaking the
Clean Architecture boundary this codebase otherwise maintains (per
`architecture/03-clean-architecture-guidelines.md`, already cited elsewhere
in this bug/solution corpus).

## Testing Plan

- Unit test: `TenantProfileResolver.GetResolvedProfile` with a `ctx` carrying
  a valid tenant ID (via whatever fake/mock `tenantv1.TenantServiceClient`
  this package's existing tests use) → confirm the outbound call's context
  carries the expected `grpcmw.MetadataTenantID` outgoing-metadata key (mirror
  however `workflow_execution_checker_test.go`/`task_execution_checker_test.go`
  already assert this for their own call sites, if such assertions exist —
  otherwise add the equivalent).
- Unit test: `TenantProfileResolver.GetResolvedProfile` with a `ctx` carrying
  no tenant ID → returns the `withTenantMetadata` error wrapped as
  `grpcclient: profile resolver: ...`, and `r.tenant.GetResolvedProfile` is
  never called (mirrors this package's existing "missing tenant" test shape
  for its other two call sites).
- Regression: `ListProjects.Execute`'s existing tests (`list_projects_test.go`,
  if present) covering the `developer`-role visibility-filter branch should
  continue to pass unchanged — this fix doesn't change `ListProjects`'s own
  logic, only what `TenantProfileResolver` sends downstream.
- End-to-end (post-deploy): re-verify `project.list`/create-project flow
  against `b15.openledger.vn` logged in as a `developer`-role user (not the
  bootstrap admin, which is `admin`/never hits this branch) — should move
  from `PROJECT_PROFILE_RESOLVE_FAILED` to a real (possibly filtered) list.
