# TASK-016: `TenantProfileResolver.GetResolvedProfile` calls `withTenantMetadata` before the outbound RPC

**From Solution:** SOL-008
**Priority:** P1
**Service:** `project-service`
**File:** `internal/adapter/grpcclient/profile_resolver.go`
**Depends on:** none
**Status:** `[x]` DONE — `TenantProfileResolver.GetResolvedProfile` now calls `withTenantMetadata(ctx)` before the outbound RPC, exactly as sketched below. `go build`/`go vet`/`go test ./services/project-service/...` all clean.

---

## Context

`TenantProfileResolver.GetResolvedProfile` forwards the caller's `ctx`
unchanged to tenant-service's `GetResolvedProfile` RPC. This package's own
`tenant_forwarding.go` already defines `withTenantMetadata(ctx)
(context.Context, error)`, used correctly by this package's two other
outbound-call sites (`workflow_execution_checker.go`,
`task_execution_checker.go`) but not by this one — see BUG-008.

## Changes to make

Current code (`profile_resolver.go:36-49`):

```go
func (r *TenantProfileResolver) GetResolvedProfile(ctx context.Context, tenantID, userID string) (usecase.ResolvedProfileView, error) {
	resp, err := r.tenant.GetResolvedProfile(ctx, &tenantv1.GetResolvedProfileRequest{UserId: userID})
	if err != nil {
		return usecase.ResolvedProfileView{}, fmt.Errorf("grpcclient: tenant-service GetResolvedProfile: %w", err)
	}
	var decoded resolvedFleetSection
	if err := json.Unmarshal([]byte(resp.GetResolvedSettingsJson()), &decoded); err != nil {
		return usecase.ResolvedProfileView{}, fmt.Errorf("grpcclient: unmarshal resolved_settings_json: %w", err)
	}
	if decoded.Fleet == nil || decoded.Fleet.AllowedServerTags == nil {
		return usecase.NewResolvedProfileView(nil, false), nil
	}
	return usecase.NewResolvedProfileView(*decoded.Fleet.AllowedServerTags, true), nil
}
```

Replace with:

```go
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
	var decoded resolvedFleetSection
	if err := json.Unmarshal([]byte(resp.GetResolvedSettingsJson()), &decoded); err != nil {
		return usecase.ResolvedProfileView{}, fmt.Errorf("grpcclient: unmarshal resolved_settings_json: %w", err)
	}
	if decoded.Fleet == nil || decoded.Fleet.AllowedServerTags == nil {
		return usecase.NewResolvedProfileView(nil, false), nil
	}
	return usecase.NewResolvedProfileView(*decoded.Fleet.AllowedServerTags, true), nil
}
```

No import changes (`withTenantMetadata` is already in this package via
`tenant_forwarding.go`; `fmt` is already imported in this file).

Per SOL-008's scope note: `DevServerTags` (this same struct's other method,
calling `infra-fleet-service.ListDevServers`) is out of scope for this task —
verify separately whether it needs the same fix before assuming it's fine.

## Verify

```bash
cd backend-go
go build ./services/project-service/...
go vet ./services/project-service/...
go test ./services/project-service/... -count=1
```

Expected: clean build, all existing tests pass. TASK-017 adds the tests
specific to this fix.
