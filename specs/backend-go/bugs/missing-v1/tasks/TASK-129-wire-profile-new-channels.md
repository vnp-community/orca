# TASK-129: Wire `profile.getUserProfile`/`listDepts`/`updateCompany`/`updateDept`/`updateUser`

**From Solution:** SOL-019
**Priority:** P1
**Service:** `api-gateway`
**File:** `services/api-gateway/internal/adapter/wscompat/channels.go`
**Depends on:** TASK-126 (creates `registerProfileChannels`), TASK-128 (generated `tenantv1` stubs for the new RPCs)
**Status:** `[ ]` TODO

---

## Context

Extends `registerProfileChannels` (added in TASK-126, currently only
`profile.getResolved`) with the 5 channels that call the new RPCs TASK-128
implemented. Same shape as every other channel in this file: decode
`args[0]`, `AttachIdentity`, per-RPC `rpcTimeout` deadline, call, return.

## Changes to make

**File:** `services/api-gateway/internal/adapter/wscompat/channels.go`

Find `registerProfileChannels` (added by TASK-126):

```go
func registerProfileChannels(r *Registry, client tenantv1.TenantServiceClient) {
	r.Register("profile.getResolved", func(ctx context.Context, id Identity, _ []json.RawMessage) (any, error) {
		ctx = gatewaygrpc.AttachIdentity(ctx, usecase.Identity{TenantID: id.TenantID, UserID: id.UserID})
		rpcCtx, cancel := context.WithTimeout(ctx, rpcTimeout)
		defer cancel()
		resp, err := client.GetResolvedProfile(rpcCtx, &tenantv1.GetResolvedProfileRequest{UserId: id.UserID})
		if err != nil {
			return nil, err
		}
		return resp, nil
	})
}
```

Replace with (adds the 5 new `r.Register` calls inside the same function):

```go
func registerProfileChannels(r *Registry, client tenantv1.TenantServiceClient) {
	r.Register("profile.getResolved", func(ctx context.Context, id Identity, _ []json.RawMessage) (any, error) {
		ctx = gatewaygrpc.AttachIdentity(ctx, usecase.Identity{TenantID: id.TenantID, UserID: id.UserID})
		rpcCtx, cancel := context.WithTimeout(ctx, rpcTimeout)
		defer cancel()
		resp, err := client.GetResolvedProfile(rpcCtx, &tenantv1.GetResolvedProfileRequest{UserId: id.UserID})
		if err != nil {
			return nil, err
		}
		return resp, nil
	})

	r.Register("profile.getUserProfile", func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
		type getArgs struct {
			UserID string `json:"userId"`
		}
		in, err := decodeArg[getArgs](args, 0)
		if err != nil {
			return nil, err
		}
		ctx = gatewaygrpc.AttachIdentity(ctx, usecase.Identity{TenantID: id.TenantID, UserID: id.UserID})
		rpcCtx, cancel := context.WithTimeout(ctx, rpcTimeout)
		defer cancel()
		resp, err := client.GetUserProfile(rpcCtx, &tenantv1.GetUserProfileRequest{UserId: in.UserID})
		if err != nil {
			return nil, err
		}
		return resp.GetProfile(), nil
	})

	r.Register("profile.listDepts", func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
		type listArgs struct {
			CompanyID string `json:"companyId"`
		}
		in, err := decodeArg[listArgs](args, 0)
		if err != nil {
			return nil, err
		}
		ctx = gatewaygrpc.AttachIdentity(ctx, usecase.Identity{TenantID: id.TenantID, UserID: id.UserID})
		rpcCtx, cancel := context.WithTimeout(ctx, rpcTimeout)
		defer cancel()
		resp, err := client.ListDepartments(rpcCtx, &tenantv1.ListDepartmentsRequest{CompanyId: in.CompanyID})
		if err != nil {
			return nil, err
		}
		return resp.GetDepartments(), nil
	})

	r.Register("profile.updateCompany", func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
		type updateArgs struct {
			ID           string `json:"id"`
			Name         string `json:"name"`
			SettingsJSON string `json:"settingsJson"`
		}
		in, err := decodeArg[updateArgs](args, 0)
		if err != nil {
			return nil, err
		}
		ctx = gatewaygrpc.AttachIdentity(ctx, usecase.Identity{TenantID: id.TenantID, UserID: id.UserID})
		rpcCtx, cancel := context.WithTimeout(ctx, rpcTimeout)
		defer cancel()
		resp, err := client.UpdateCompany(rpcCtx, &tenantv1.UpdateCompanyRequest{
			Id: in.ID, Name: in.Name, SettingsJson: in.SettingsJSON,
		})
		if err != nil {
			return nil, err
		}
		return resp.GetCompany(), nil
	})

	r.Register("profile.updateDept", func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
		type updateArgs struct {
			ID           string `json:"id"`
			Name         string `json:"name"`
			SettingsJSON string `json:"settingsJson"`
		}
		in, err := decodeArg[updateArgs](args, 0)
		if err != nil {
			return nil, err
		}
		ctx = gatewaygrpc.AttachIdentity(ctx, usecase.Identity{TenantID: id.TenantID, UserID: id.UserID})
		rpcCtx, cancel := context.WithTimeout(ctx, rpcTimeout)
		defer cancel()
		resp, err := client.UpdateDepartment(rpcCtx, &tenantv1.UpdateDepartmentRequest{
			Id: in.ID, Name: in.Name, SettingsJson: in.SettingsJSON,
		})
		if err != nil {
			return nil, err
		}
		return resp.GetDepartment(), nil
	})

	r.Register("profile.updateUser", func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
		type updateArgs struct {
			UserID          string `json:"userId"`
			DepartmentID    string `json:"departmentId"`
			ClearDepartment bool   `json:"clearDepartment"`
			SettingsJSON    string `json:"settingsJson"`
		}
		in, err := decodeArg[updateArgs](args, 0)
		if err != nil {
			return nil, err
		}
		ctx = gatewaygrpc.AttachIdentity(ctx, usecase.Identity{TenantID: id.TenantID, UserID: id.UserID})
		rpcCtx, cancel := context.WithTimeout(ctx, rpcTimeout)
		defer cancel()
		resp, err := client.UpdateUserProfile(rpcCtx, &tenantv1.UpdateUserProfileRequest{
			UserId: in.UserID, DepartmentId: in.DepartmentID, ClearDepartment: in.ClearDepartment, SettingsJson: in.SettingsJSON,
		})
		if err != nil {
			return nil, err
		}
		return resp.GetProfile(), nil
	})
}
```

## Verify

```bash
cd /opt/repos/orca/backend-go/services/api-gateway
go build ./... && go vet ./...
```

Expected: clean build. All 6 `profile.*` channels now resolve through
`wscompat.Registry`.
