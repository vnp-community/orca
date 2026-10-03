package grpc

import (
	"context"
	"testing"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	"github.com/stablyai/orca-go/common/internalcaller"
	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/auth-service/internal/domain"
	"github.com/stablyai/orca-go/services/auth-service/internal/usecase"

	authv1 "github.com/stablyai/orca-go/proto/gen/go/orca/auth/v1"
)

type suspensionRepoStub struct{ suspended map[string]bool }

func (s suspensionRepoStub) SetMcpPatSuspension(_ context.Context, t string, on bool, _ string, _ time.Time) error {
	s.suspended[t] = on
	return nil
}
func (s suspensionRepoStub) IsMcpPatSuspended(_ context.Context, t string) (bool, error) {
	return s.suspended[t], nil
}

type susAuditSink struct{}

func (susAuditSink) Append(context.Context, domain.AuditEntry) error { return nil }
func (susAuditSink) Query(context.Context, usecase.AuditQueryFilter, string, int32) ([]domain.AuditEntry, string, error) {
	return nil, "", nil
}

type susFixedClock struct{}

func (susFixedClock) Now() time.Time { return time.Unix(1_700_000_000, 0).UTC() }

func TestSetMcpPatSuspension_UsesMetadataTenant(t *testing.T) {
	repo := suspensionRepoStub{suspended: map[string]bool{}}
	s := &McpServer{mcp: McpUsecases{Suspend: usecase.NewSetMcpPatSuspension(repo, susAuditSink{}, susFixedClock{})}}
	ctx := tenant.WithTenantID(context.Background(), "t-1")
	if _, err := s.SetMcpPatSuspension(ctx, &authv1.SetMcpPatSuspensionRequest{Suspended: true, Reason: "incident"}); err != nil {
		t.Fatal(err)
	}
	if !repo.suspended["t-1"] {
		t.Fatal("tenant from metadata was not suspended")
	}
	if _, err := s.SetMcpPatSuspension(context.Background(), &authv1.SetMcpPatSuspensionRequest{Suspended: true}); err == nil {
		t.Fatal("a call without a tenant must fail")
	}
}

func guardedCall(t *testing.T, g grpc.UnaryServerInterceptor, method string, md metadata.MD) error {
	t.Helper()
	ctx := metadata.NewIncomingContext(context.Background(), md)
	_, err := g(ctx, nil, &grpc.UnaryServerInfo{FullMethod: method}, func(context.Context, any) (any, error) { return nil, nil })
	return err
}

func TestMcpInternalMethodsAreGuardedWithSeparateSecrets(t *testing.T) {
	resolve := internalcaller.Guard("gw-secret", McpPrincipalMethods...)
	suspend := internalcaller.Guard("mcp-secret", append(append([]string{}, OAuthInternalMethods...), McpSuspensionMethods...)...)
	resolveM, suspendM := authv1.AuthService_ResolveMcpPrincipal_FullMethodName, authv1.AuthService_SetMcpPatSuspension_FullMethodName

	if status.Code(guardedCall(t, resolve, resolveM, nil)) != codes.PermissionDenied {
		t.Fatal("ResolveMcpPrincipal without a token must be denied")
	}
	if err := guardedCall(t, resolve, resolveM, metadata.Pairs(internalcaller.MetadataKey, "gw-secret")); err != nil {
		t.Fatalf("gateway token: %v", err)
	}
	if status.Code(guardedCall(t, suspend, suspendM, metadata.Pairs(internalcaller.MetadataKey, "gw-secret"))) != codes.PermissionDenied {
		t.Fatal("the gateway secret must not open SetMcpPatSuspension")
	}
	if err := guardedCall(t, suspend, suspendM, metadata.Pairs(internalcaller.MetadataKey, "mcp-secret")); err != nil {
		t.Fatalf("mcp-service token: %v", err)
	}
	// Other RPCs stay open through both guards.
	if err := guardedCall(t, resolve, authv1.AuthService_RevokeMcpToken_FullMethodName, nil); err != nil {
		t.Fatalf("unrelated rpc blocked: %v", err)
	}
}
