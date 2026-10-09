package main

import (
	"context"
	"io"
	"log/slog"
	"net"
	"reflect"
	"testing"

	"github.com/stablyai/orca-go/common/grpcmw"
	"github.com/stablyai/orca-go/common/internalcaller"
	"github.com/stablyai/orca-go/common/tenant"
	authv1 "github.com/stablyai/orca-go/proto/gen/go/orca/auth/v1"
	"github.com/stablyai/orca-go/services/request-service/internal/adapter/grpcclient"
	"github.com/stablyai/orca-go/services/request-service/internal/config"
	"github.com/stablyai/orca-go/services/request-service/internal/usecase"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// directoryAuth is auth-service's directory read as the real server sees it: guarded by the internal token, tenant
// from the metadata interceptor, no user anywhere.
type directoryAuth struct {
	authv1.UnimplementedAuthServiceServer
	admins map[string][]string
}

func (d directoryAuth) ListActiveTenantAdmins(ctx context.Context, _ *authv1.ListActiveTenantAdminsRequest) (*authv1.ListActiveTenantAdminsResponse, error) {
	tenantID, err := tenant.RequireTenantID(ctx)
	if err != nil {
		return nil, status.Error(codes.Unauthenticated, "AUTH_NO_TENANT")
	}
	return &authv1.ListActiveTenantAdminsResponse{UserIds: d.admins[tenantID]}, nil
}

func serveDirectoryAuth(t *testing.T, token string) string {
	t.Helper()
	lis, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	srv := grpc.NewServer(
		grpc.ChainUnaryInterceptor(grpcmw.TenantExtractionInterceptor(), internalcaller.Guard(token, authv1.AuthService_ListActiveTenantAdmins_FullMethodName)),
	)
	authv1.RegisterAuthServiceServer(srv, directoryAuth{admins: map[string][]string{"t1": {"a1"}, "t2": {"a2"}}})
	go func() { _ = srv.Serve(lis) }()
	t.Cleanup(srv.Stop)
	return lis.Addr().String()
}

func quietLog() *slog.Logger { return slog.New(slog.NewTextHandler(io.Discard, nil)) }

// The outbox relay's context carries a tenant and no user; the wired resolver must still get the admin list.
func TestDialApprovalDirectories_AdminsResolveFromBackgroundContext(t *testing.T) {
	addr := serveDirectoryAuth(t, "dir-secret")
	dirs, err := dialApprovalDirectories(config.Config{AuthServiceAddr: addr, AuthDirectoryCallerToken: "dir-secret"}, quietLog())
	if err != nil {
		t.Fatal(err)
	}
	defer dirs.close()

	got, err := dirs.admins.ListAdmins(tenant.WithTenantID(context.Background(), "t1"), "t1")
	if err != nil || !reflect.DeepEqual(got, []string{"a1"}) {
		t.Fatalf("admins = %v err = %v", got, err)
	}
	if _, err := dirs.admins.ListAdmins(tenant.WithTenantID(context.Background(), "t1"), "t2"); err == nil {
		t.Fatal("a tenant argument that differs from the context tenant must be refused")
	}
	if got, _ := dirs.admins.ListAdmins(tenant.WithTenantID(context.Background(), "t2"), "t2"); !reflect.DeepEqual(got, []string{"a2"}) {
		t.Fatalf("tenant isolation: t2 got %v", got)
	}
}

func TestDialApprovalDirectories_WrongTokenIsRefusedAndNotRetryable(t *testing.T) {
	addr := serveDirectoryAuth(t, "dir-secret")
	dirs, err := dialApprovalDirectories(config.Config{AuthServiceAddr: addr, AuthDirectoryCallerToken: "other"}, quietLog())
	if err != nil {
		t.Fatal(err)
	}
	defer dirs.close()
	_, err = dirs.admins.ListAdmins(tenant.WithTenantID(context.Background(), "t1"), "t1")
	if err == nil || !usecase.IsNotRetryable(err) {
		t.Fatalf("a refused credential must be reported as not retryable, got %v", err)
	}
}

// Fail closed: without the token the admin lookup is the unavailable directory, never an empty answer.
func TestDialApprovalDirectories_EmptyTokenFailsClosedWithoutDialing(t *testing.T) {
	addr := serveDirectoryAuth(t, "dir-secret")
	dirs, err := dialApprovalDirectories(config.Config{AuthServiceAddr: addr}, quietLog())
	if err != nil {
		t.Fatal(err)
	}
	defer dirs.close()
	if _, ok := dirs.admins.(grpcclient.UnavailableDirectory); !ok {
		t.Fatalf("admins = %T, want UnavailableDirectory", dirs.admins)
	}
	got, err := dirs.admins.ListAdmins(tenant.WithTenantID(context.Background(), "t1"), "t1")
	if got != nil || err == nil || !usecase.IsNotRetryable(err) {
		t.Fatalf("got %v %v", got, err)
	}
}
