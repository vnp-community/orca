package wscompat

import (
	"context"
	"errors"
	"testing"

	"google.golang.org/grpc"

	authv1 "github.com/stablyai/orca-go/proto/gen/go/orca/auth/v1"
	tenantv1 "github.com/stablyai/orca-go/proto/gen/go/orca/tenant/v1"
)

// fakeTenantServiceClientForAdmin embeds the nil interface and overrides
// only GetUserProfile — the one method admin.listUsers' department-
// enrichment calls (attachDepartmentID).
type fakeTenantServiceClientForAdmin struct {
	tenantv1.TenantServiceClient

	getUserProfileFunc func(context.Context, *tenantv1.GetUserProfileRequest) (*tenantv1.GetUserProfileResponse, error)
}

func (f *fakeTenantServiceClientForAdmin) GetUserProfile(ctx context.Context, in *tenantv1.GetUserProfileRequest, _ ...grpc.CallOption) (*tenantv1.GetUserProfileResponse, error) {
	return f.getUserProfileFunc(ctx, in)
}

// fakeAuthServiceClientForAdmin embeds the nil interface and overrides only
// the methods channels_admin_users.go's handlers call — same
// embed-the-nil-interface pattern as fakeInfraFleetClient/
// fakeTenantServiceClient2 elsewhere in this package.
type fakeAuthServiceClientForAdmin struct {
	authv1.AuthServiceClient

	createUserFunc     func(context.Context, *authv1.CreateUserRequest) (*authv1.CreateUserResponse, error)
	listUsersFunc      func(context.Context, *authv1.ListUsersRequest) (*authv1.ListUsersResponse, error)
	updateUserRoleFunc func(context.Context, *authv1.UpdateUserRoleRequest) (*authv1.UpdateUserRoleResponse, error)
	deactivateUserFunc func(context.Context, *authv1.DeactivateUserRequest) (*authv1.DeactivateUserResponse, error)
	reactivateUserFunc func(context.Context, *authv1.ReactivateUserRequest) (*authv1.ReactivateUserResponse, error)
}

func (f *fakeAuthServiceClientForAdmin) CreateUser(ctx context.Context, in *authv1.CreateUserRequest, _ ...grpc.CallOption) (*authv1.CreateUserResponse, error) {
	return f.createUserFunc(ctx, in)
}
func (f *fakeAuthServiceClientForAdmin) ListUsers(ctx context.Context, in *authv1.ListUsersRequest, _ ...grpc.CallOption) (*authv1.ListUsersResponse, error) {
	return f.listUsersFunc(ctx, in)
}
func (f *fakeAuthServiceClientForAdmin) UpdateUserRole(ctx context.Context, in *authv1.UpdateUserRoleRequest, _ ...grpc.CallOption) (*authv1.UpdateUserRoleResponse, error) {
	return f.updateUserRoleFunc(ctx, in)
}
func (f *fakeAuthServiceClientForAdmin) DeactivateUser(ctx context.Context, in *authv1.DeactivateUserRequest, _ ...grpc.CallOption) (*authv1.DeactivateUserResponse, error) {
	return f.deactivateUserFunc(ctx, in)
}
func (f *fakeAuthServiceClientForAdmin) ReactivateUser(ctx context.Context, in *authv1.ReactivateUserRequest, _ ...grpc.CallOption) (*authv1.ReactivateUserResponse, error) {
	return f.reactivateUserFunc(ctx, in)
}

func TestAdminCreateUserChannel_RequiresAdmin(t *testing.T) {
	r := NewRegistry()
	registerAdminUserChannels(r, &fakeAuthServiceClientForAdmin{}, nil)

	_, err := r.Dispatch(context.Background(), Identity{TenantID: "tenant-1", UserID: "user-1", Role: "user"}, "admin.createUser", argsJSON(t, map[string]any{"email": "a@b.com"}))
	if !errors.Is(err, errNotAdmin) {
		t.Fatalf("want errNotAdmin, got %v", err)
	}
}

func TestAdminCreateUserChannel_DefaultsTenantIDToCaller(t *testing.T) {
	var gotReq *authv1.CreateUserRequest
	fake := &fakeAuthServiceClientForAdmin{
		createUserFunc: func(ctx context.Context, in *authv1.CreateUserRequest) (*authv1.CreateUserResponse, error) {
			gotReq = in
			return &authv1.CreateUserResponse{
				User:              &authv1.User{Id: "u2", TenantId: in.GetTenantId(), Email: in.GetEmail(), Role: in.GetRole()},
				GeneratedPassword: "one-time-secret",
			}, nil
		},
	}
	r := NewRegistry()
	registerAdminUserChannels(r, fake, nil)

	result, err := r.Dispatch(context.Background(), Identity{TenantID: "tenant-1", UserID: "admin-1", Role: "admin"}, "admin.createUser", argsJSON(t, map[string]any{
		"email": "new@example.com", "name": "New", "role": "user",
	}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotReq.GetTenantId() != "tenant-1" {
		t.Errorf("want tenantId defaulting to caller's tenant tenant-1, got %q", gotReq.GetTenantId())
	}
	wrapped, ok := result.(map[string]any)
	if !ok {
		t.Fatalf("unexpected result type: %+v", result)
	}
	if wrapped["generatedPassword"] != "one-time-secret" {
		t.Errorf("want generatedPassword surfaced, got %v", wrapped["generatedPassword"])
	}
	user, ok := wrapped["user"].(userView)
	if !ok || user.Email != "new@example.com" {
		t.Fatalf("unexpected user: %+v", wrapped["user"])
	}
}

func TestAdminCreateUserChannel_ExplicitTenantIDForCrossTenantBootstrap(t *testing.T) {
	var gotReq *authv1.CreateUserRequest
	fake := &fakeAuthServiceClientForAdmin{
		createUserFunc: func(ctx context.Context, in *authv1.CreateUserRequest) (*authv1.CreateUserResponse, error) {
			gotReq = in
			return &authv1.CreateUserResponse{User: &authv1.User{Id: "u2", TenantId: in.GetTenantId()}}, nil
		},
	}
	r := NewRegistry()
	registerAdminUserChannels(r, fake, nil)

	_, err := r.Dispatch(context.Background(), Identity{TenantID: "tenant-1", UserID: "admin-1", Role: "admin"}, "admin.createUser", argsJSON(t, map[string]any{
		"email": "first-admin@newco.com", "tenantId": "tenant-2",
	}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotReq.GetTenantId() != "tenant-2" {
		t.Errorf("want explicit tenantId tenant-2 to be used as-is, got %q", gotReq.GetTenantId())
	}
}

func TestAdminListUsersChannel_RequiresAdmin(t *testing.T) {
	r := NewRegistry()
	registerAdminUserChannels(r, &fakeAuthServiceClientForAdmin{}, nil)

	_, err := r.Dispatch(context.Background(), Identity{TenantID: "tenant-1", UserID: "user-1", Role: "user"}, "admin.listUsers", argsJSON(t, map[string]any{}))
	if !errors.Is(err, errNotAdmin) {
		t.Fatalf("want errNotAdmin, got %v", err)
	}
}

func TestAdminListUsersChannel_Success(t *testing.T) {
	fake := &fakeAuthServiceClientForAdmin{
		listUsersFunc: func(ctx context.Context, in *authv1.ListUsersRequest) (*authv1.ListUsersResponse, error) {
			if in.GetTenantId() != "tenant-1" {
				t.Errorf("want tenantId tenant-1, got %q", in.GetTenantId())
			}
			return &authv1.ListUsersResponse{
				Users: []*authv1.User{
					{Id: "u1", TenantId: "tenant-1", Email: "a@b.com", Name: "A", Role: authv1.Role_ROLE_ADMIN, IsActive: true},
				},
				NextPageToken: "next",
			}, nil
		},
	}
	r := NewRegistry()
	registerAdminUserChannels(r, fake, nil)

	result, err := r.Dispatch(context.Background(), Identity{TenantID: "tenant-1", UserID: "admin-1", Role: "admin"}, "admin.listUsers", argsJSON(t, map[string]any{}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	wrapped, ok := result.(map[string]any)
	if !ok {
		t.Fatalf("unexpected result type: %+v", result)
	}
	users, ok := wrapped["users"].([]userView)
	if !ok || len(users) != 1 || users[0].Role != "admin" || users[0].Email != "a@b.com" {
		t.Fatalf("unexpected users: %+v", wrapped["users"])
	}
	if wrapped["nextPageToken"] != "next" {
		t.Errorf("want nextPageToken 'next', got %v", wrapped["nextPageToken"])
	}
}

// TestAdminListUsersChannel_EnrichesDepartmentID is the live-bug
// regression: the Users tab's "Assign department" looked like it silently
// reverted on refresh because admin.listUsers never surfaced the
// tenant-service-side department assignment at all.
func TestAdminListUsersChannel_EnrichesDepartmentID(t *testing.T) {
	authFake := &fakeAuthServiceClientForAdmin{
		listUsersFunc: func(ctx context.Context, in *authv1.ListUsersRequest) (*authv1.ListUsersResponse, error) {
			return &authv1.ListUsersResponse{
				Users: []*authv1.User{{Id: "u1", TenantId: "tenant-1", Email: "a@b.com"}},
			}, nil
		},
	}
	tenantFake := &fakeTenantServiceClientForAdmin{
		getUserProfileFunc: func(ctx context.Context, in *tenantv1.GetUserProfileRequest) (*tenantv1.GetUserProfileResponse, error) {
			if in.GetUserId() != "u1" {
				t.Errorf("want GetUserProfile userId=u1, got %q", in.GetUserId())
			}
			return &tenantv1.GetUserProfileResponse{
				Profile: &tenantv1.UserProfile{UserId: "u1", DepartmentId: "dept-eng"},
			}, nil
		},
	}
	r := NewRegistry()
	registerAdminUserChannels(r, authFake, tenantFake)

	result, err := r.Dispatch(context.Background(), Identity{TenantID: "tenant-1", UserID: "admin-1", Role: "admin"}, "admin.listUsers", argsJSON(t, map[string]any{}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	wrapped := result.(map[string]any)
	users := wrapped["users"].([]userView)
	if len(users) != 1 || users[0].DepartmentID != "dept-eng" {
		t.Fatalf("want DepartmentID=dept-eng, got %+v", users)
	}
}

// TestAdminListUsersChannel_NoProfileYetLeavesDepartmentIDEmpty verifies
// TENANT_PROFILE_NOT_FOUND (no department ever assigned) degrades to an
// empty DepartmentID instead of failing the whole list.
func TestAdminListUsersChannel_NoProfileYetLeavesDepartmentIDEmpty(t *testing.T) {
	authFake := &fakeAuthServiceClientForAdmin{
		listUsersFunc: func(ctx context.Context, in *authv1.ListUsersRequest) (*authv1.ListUsersResponse, error) {
			return &authv1.ListUsersResponse{
				Users: []*authv1.User{{Id: "u1", TenantId: "tenant-1", Email: "a@b.com"}},
			}, nil
		},
	}
	tenantFake := &fakeTenantServiceClientForAdmin{
		getUserProfileFunc: func(ctx context.Context, in *tenantv1.GetUserProfileRequest) (*tenantv1.GetUserProfileResponse, error) {
			return nil, errors.New("rpc error: code = NotFound desc = TENANT_PROFILE_NOT_FOUND")
		},
	}
	r := NewRegistry()
	registerAdminUserChannels(r, authFake, tenantFake)

	result, err := r.Dispatch(context.Background(), Identity{TenantID: "tenant-1", UserID: "admin-1", Role: "admin"}, "admin.listUsers", argsJSON(t, map[string]any{}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	wrapped := result.(map[string]any)
	users := wrapped["users"].([]userView)
	if len(users) != 1 || users[0].DepartmentID != "" {
		t.Fatalf("want DepartmentID empty, got %+v", users)
	}
}

func TestAdminUpdateUserRoleChannel_RequiresAdmin(t *testing.T) {
	r := NewRegistry()
	registerAdminUserChannels(r, &fakeAuthServiceClientForAdmin{}, nil)

	_, err := r.Dispatch(context.Background(), Identity{TenantID: "tenant-1", UserID: "user-1", Role: ""}, "admin.updateUserRole", argsJSON(t, map[string]any{"userId": "u1", "role": "admin"}))
	if !errors.Is(err, errNotAdmin) {
		t.Fatalf("want errNotAdmin, got %v", err)
	}
}

func TestAdminUpdateUserRoleChannel_Success(t *testing.T) {
	var gotReq *authv1.UpdateUserRoleRequest
	fake := &fakeAuthServiceClientForAdmin{
		updateUserRoleFunc: func(ctx context.Context, in *authv1.UpdateUserRoleRequest) (*authv1.UpdateUserRoleResponse, error) {
			gotReq = in
			return &authv1.UpdateUserRoleResponse{User: &authv1.User{Id: in.GetUserId(), Role: in.GetRole()}}, nil
		},
	}
	r := NewRegistry()
	registerAdminUserChannels(r, fake, nil)

	result, err := r.Dispatch(context.Background(), Identity{TenantID: "tenant-1", UserID: "admin-1", Role: "admin"}, "admin.updateUserRole", argsJSON(t, map[string]any{"userId": "u1", "role": "admin"}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotReq.GetUserId() != "u1" || gotReq.GetRole() != authv1.Role_ROLE_ADMIN {
		t.Errorf("unexpected request: %+v", gotReq)
	}
	user, ok := result.(userView)
	if !ok || user.Role != "admin" {
		t.Fatalf("unexpected result: %+v", result)
	}
}

func TestAdminDeactivateReactivateUserChannels_Success(t *testing.T) {
	fake := &fakeAuthServiceClientForAdmin{
		deactivateUserFunc: func(ctx context.Context, in *authv1.DeactivateUserRequest) (*authv1.DeactivateUserResponse, error) {
			return &authv1.DeactivateUserResponse{User: &authv1.User{Id: in.GetUserId(), IsActive: false}}, nil
		},
		reactivateUserFunc: func(ctx context.Context, in *authv1.ReactivateUserRequest) (*authv1.ReactivateUserResponse, error) {
			return &authv1.ReactivateUserResponse{User: &authv1.User{Id: in.GetUserId(), IsActive: true}}, nil
		},
	}
	r := NewRegistry()
	registerAdminUserChannels(r, fake, nil)
	adminID := Identity{TenantID: "tenant-1", UserID: "admin-1", Role: "admin"}

	deactivated, err := r.Dispatch(context.Background(), adminID, "admin.deactivateUser", argsJSON(t, map[string]any{"userId": "u1"}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if v, ok := deactivated.(userView); !ok || v.IsActive {
		t.Fatalf("want isActive=false, got %+v", deactivated)
	}

	reactivated, err := r.Dispatch(context.Background(), adminID, "admin.reactivateUser", argsJSON(t, map[string]any{"userId": "u1"}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if v, ok := reactivated.(userView); !ok || !v.IsActive {
		t.Fatalf("want isActive=true, got %+v", reactivated)
	}
}
