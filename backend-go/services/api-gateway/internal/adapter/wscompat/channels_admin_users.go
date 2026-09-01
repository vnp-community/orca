// channels_admin_users.go wires auth-service's user-management RPCs
// (CreateUser/ListUsers/UpdateUserRole/DeactivateUser/ReactivateUser) to the
// frontend — found live to be completely unwired (CR-DS-006/007/008
// follow-up): the only user in the system was the bootstrap admin, seeded
// via an env var at service startup, with no UI anywhere to see or manage
// any other user.
//
// CreateUser was initially left unwired here — its usecase used to
// generate a random, never-returned password ("there is no invite/reset-
// link flow implemented in this scaffold"), so exposing it would let an
// admin create an account that could never log in. Fixed instead of
// deferred: CreateUser now accepts an optional caller-supplied password,
// and returns the generated one (once) when the caller doesn't supply one
// — see CreateUserRequest.password's doc comment in auth.proto. The admin
// is still responsible for relaying the credential to the new user out of
// band (no invite email exists) — this channel's response surfaces
// generatedPassword so the Admin Console can show it once.
//
// Every channel here is admin-gated at the wscompat layer (id.Role check,
// same as profile.createCompany/createDept in channels_tenant_project.go) —
// auth-service's own usecases ALSO gate via requireAdminActor (a DB lookup
// + OPA policy decision, independent of the Role metadata this layer
// checks), so a caller that somehow got past this layer's check would still
// be stopped server-side.
package wscompat

import (
	"context"
	"encoding/json"

	authv1 "github.com/stablyai/orca-go/proto/gen/go/orca/auth/v1"
	tenantv1 "github.com/stablyai/orca-go/proto/gen/go/orca/tenant/v1"

	gatewaygrpc "github.com/stablyai/orca-go/services/api-gateway/internal/adapter/grpc"
	"github.com/stablyai/orca-go/services/api-gateway/internal/usecase"
)

// userView — same snake_case-JSON bug as every other raw proto return this
// session found and fixed (authv1.User.TenantId/IsActive/CreatedAt all have
// snake_case json tags).
// DepartmentID defaults to "" (empty string, JSON `""`, never omitted) —
// this is the live-bug fix (Users tab "Assign department" looked like it
// silently reverted on refresh): the assign button really did persist via
// profile.updateUser (tenant-service's tenant.user_profiles table), but
// admin.listUsers sourced its rows from auth-service's auth.users only,
// which has no department column at all — so the Select never had
// anything to seed its value from and always reset to the placeholder.
type userView struct {
	ID              string `json:"id"`
	TenantID        string `json:"tenantId"`
	Email           string `json:"email"`
	Name            string `json:"name"`
	Role            string `json:"role"`
	IsActive        bool   `json:"isActive"`
	CreatedAtUnixMs int64  `json:"createdAtUnixMs"`
	DepartmentID    string `json:"departmentId"`
}

func toUserView(u *authv1.User) userView {
	var createdAtUnixMs int64
	if ts := u.GetCreatedAt(); ts != nil {
		createdAtUnixMs = ts.AsTime().UnixMilli()
	}
	return userView{
		ID:              u.GetId(),
		TenantID:        u.GetTenantId(),
		Email:           u.GetEmail(),
		Name:            u.GetName(),
		Role:            adminUserRoleWire(u.GetRole()),
		IsActive:        u.GetIsActive(),
		CreatedAtUnixMs: createdAtUnixMs,
	}
}

// attachDepartmentID looks up userID's tenant-service profile and fills in
// DepartmentID — a separate call because auth-service (this view's source)
// and tenant-service (department assignment's actual store) are different
// services with no join between them. TENANT_PROFILE_NOT_FOUND means "no
// department ever assigned", a normal state, not an error; any other error
// is swallowed too — a missing department on one row in an admin list is
// far less harmful than admin.listUsers failing outright over it.
func attachDepartmentID(ctx context.Context, tenantClient tenantv1.TenantServiceClient, v userView) userView {
	if tenantClient == nil {
		return v
	}
	rpcCtx, cancel := context.WithTimeout(ctx, rpcTimeout)
	defer cancel()
	resp, err := tenantClient.GetUserProfile(rpcCtx, &tenantv1.GetUserProfileRequest{UserId: v.ID})
	if err != nil {
		return v
	}
	v.DepartmentID = resp.GetProfile().GetDepartmentId()
	return v
}

// adminUserRoleWire/toProtoAdminUserRole mirror auth-service's own
// toProtoRole/toDomainRole (internal/adapter/grpc/server.go) — duplicated
// rather than imported since that package is internal to auth-service.
func adminUserRoleWire(r authv1.Role) string {
	switch r {
	case authv1.Role_ROLE_ADMIN:
		return "admin"
	case authv1.Role_ROLE_USER:
		return "user"
	default:
		return ""
	}
}

func toProtoAdminUserRole(role string) authv1.Role {
	switch role {
	case "admin":
		return authv1.Role_ROLE_ADMIN
	case "user":
		return authv1.Role_ROLE_USER
	default:
		return authv1.Role_ROLE_UNSPECIFIED
	}
}

func registerAdminUserChannels(r *Registry, client authv1.AuthServiceClient, tenantClient tenantv1.TenantServiceClient) {
	r.Register("admin.createUser", func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
		if id.Role != "admin" {
			return nil, errNotAdmin
		}
		type createArgs struct {
			Email    string `json:"email"`
			Name     string `json:"name"`
			Role     string `json:"role"`
			TenantID string `json:"tenantId"`
			Password string `json:"password"`
		}
		in, err := decodeArg[createArgs](args, 0)
		if err != nil {
			return nil, err
		}
		// Why default to id.TenantID: the common case is "add a teammate to
		// my own org" — an explicit tenantId is only needed to bootstrap a
		// brand-new company's first admin (see profile.createCompany), a
		// deliberately advanced/rare path.
		tenantID := in.TenantID
		if tenantID == "" {
			tenantID = id.TenantID
		}
		ctx = gatewaygrpc.AttachIdentity(ctx, usecase.Identity{TenantID: id.TenantID, UserID: id.UserID, Role: id.Role})
		rpcCtx, cancel := context.WithTimeout(ctx, rpcTimeout)
		defer cancel()
		resp, err := client.CreateUser(rpcCtx, &authv1.CreateUserRequest{
			Email: in.Email, Name: in.Name, TenantId: tenantID, Role: toProtoAdminUserRole(in.Role), Password: in.Password,
		})
		if err != nil {
			return nil, err
		}
		return map[string]any{"user": toUserView(resp.GetUser()), "generatedPassword": resp.GetGeneratedPassword()}, nil
	})

	r.Register("admin.listUsers", func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
		if id.Role != "admin" {
			return nil, errNotAdmin
		}
		type listArgs struct {
			PageToken string `json:"pageToken"`
			PageSize  int32  `json:"pageSize"`
		}
		in := decodeOptionalArg[listArgs](args, 0)
		ctx = gatewaygrpc.AttachIdentity(ctx, usecase.Identity{TenantID: id.TenantID, UserID: id.UserID, Role: id.Role})
		rpcCtx, cancel := context.WithTimeout(ctx, rpcTimeout)
		defer cancel()
		resp, err := client.ListUsers(rpcCtx, &authv1.ListUsersRequest{
			TenantId: id.TenantID, PageToken: in.PageToken, PageSize: in.PageSize,
		})
		if err != nil {
			return nil, err
		}
		views := make([]userView, 0, len(resp.GetUsers()))
		for _, u := range resp.GetUsers() {
			views = append(views, attachDepartmentID(ctx, tenantClient, toUserView(u)))
		}
		return map[string]any{"users": views, "nextPageToken": resp.GetNextPageToken()}, nil
	})

	r.Register("admin.updateUserRole", func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
		if id.Role != "admin" {
			return nil, errNotAdmin
		}
		type updateArgs struct {
			UserID string `json:"userId"`
			Role   string `json:"role"`
		}
		in, err := decodeArg[updateArgs](args, 0)
		if err != nil {
			return nil, err
		}
		ctx = gatewaygrpc.AttachIdentity(ctx, usecase.Identity{TenantID: id.TenantID, UserID: id.UserID, Role: id.Role})
		rpcCtx, cancel := context.WithTimeout(ctx, rpcTimeout)
		defer cancel()
		resp, err := client.UpdateUserRole(rpcCtx, &authv1.UpdateUserRoleRequest{
			UserId: in.UserID, Role: toProtoAdminUserRole(in.Role),
		})
		if err != nil {
			return nil, err
		}
		return toUserView(resp.GetUser()), nil
	})

	r.Register("admin.deactivateUser", func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
		if id.Role != "admin" {
			return nil, errNotAdmin
		}
		type userArgs struct {
			UserID string `json:"userId"`
		}
		in, err := decodeArg[userArgs](args, 0)
		if err != nil {
			return nil, err
		}
		ctx = gatewaygrpc.AttachIdentity(ctx, usecase.Identity{TenantID: id.TenantID, UserID: id.UserID, Role: id.Role})
		rpcCtx, cancel := context.WithTimeout(ctx, rpcTimeout)
		defer cancel()
		resp, err := client.DeactivateUser(rpcCtx, &authv1.DeactivateUserRequest{UserId: in.UserID})
		if err != nil {
			return nil, err
		}
		return toUserView(resp.GetUser()), nil
	})

	r.Register("admin.reactivateUser", func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
		if id.Role != "admin" {
			return nil, errNotAdmin
		}
		type userArgs struct {
			UserID string `json:"userId"`
		}
		in, err := decodeArg[userArgs](args, 0)
		if err != nil {
			return nil, err
		}
		ctx = gatewaygrpc.AttachIdentity(ctx, usecase.Identity{TenantID: id.TenantID, UserID: id.UserID, Role: id.Role})
		rpcCtx, cancel := context.WithTimeout(ctx, rpcTimeout)
		defer cancel()
		resp, err := client.ReactivateUser(rpcCtx, &authv1.ReactivateUserRequest{UserId: in.UserID})
		if err != nil {
			return nil, err
		}
		return toUserView(resp.GetUser()), nil
	})
}
