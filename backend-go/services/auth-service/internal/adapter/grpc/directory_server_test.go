package grpc

import (
	"context"
	"errors"
	"testing"

	"github.com/stablyai/orca-go/common/tenant"
	authv1 "github.com/stablyai/orca-go/proto/gen/go/orca/auth/v1"
	"github.com/stablyai/orca-go/services/auth-service/internal/domain"
	"github.com/stablyai/orca-go/services/auth-service/internal/usecase"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type directoryUsers struct {
	usecase.UserRepository
	users []domain.User
	err   error
}

func (d directoryUsers) ListUsers(context.Context, string, string, int32) ([]domain.User, string, error) {
	return d.users, "", d.err
}

func TestDirectoryServer_ListActiveTenantAdmins(t *testing.T) {
	repo := directoryUsers{users: []domain.User{
		{ID: "a1", TenantID: "t1", Role: domain.RoleAdmin, IsActive: true},
		{ID: "u1", TenantID: "t1", Role: domain.RoleUser, IsActive: true},
		{ID: "a2", TenantID: "t1", Role: domain.RoleAdmin, IsActive: false},
	}}
	srv := WithDirectory(authv1.UnimplementedAuthServiceServer{}, usecase.NewListActiveTenantAdmins(repo))

	resp, err := srv.ListActiveTenantAdmins(tenant.WithTenantID(context.Background(), "t1"), &authv1.ListActiveTenantAdminsRequest{})
	if err != nil || len(resp.GetUserIds()) != 1 || resp.GetUserIds()[0] != "a1" || resp.GetTruncated() {
		t.Fatalf("resp = %v err = %v", resp, err)
	}

	if _, err := srv.ListActiveTenantAdmins(context.Background(), &authv1.ListActiveTenantAdminsRequest{}); status.Code(err) != codes.Unauthenticated {
		t.Fatalf("without a tenant: %v, want Unauthenticated", err)
	}

	failing := WithDirectory(authv1.UnimplementedAuthServiceServer{}, usecase.NewListActiveTenantAdmins(directoryUsers{err: errors.New("db")}))
	if _, err := failing.ListActiveTenantAdmins(tenant.WithTenantID(context.Background(), "t1"), &authv1.ListActiveTenantAdminsRequest{}); status.Code(err) != codes.Internal {
		t.Fatalf("repository failure: %v, want Internal", err)
	}
}

func TestDirectoryServer_KeepsTheWrappedServerMethods(t *testing.T) {
	var _ authv1.AuthServiceServer = WithDirectory(authv1.UnimplementedAuthServiceServer{}, nil)
}
