package grpcmw

import (
	"context"
	"testing"

	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"

	"github.com/stablyai/orca-go/common/tenant"
)

func TestTenantExtractionInterceptor_AttachesRoleWhenPresent(t *testing.T) {
	md := metadata.Pairs(MetadataTenantID, "t1", MetadataUserID, "u1", MetadataRole, "admin")
	ctx := metadata.NewIncomingContext(context.Background(), md)

	interceptor := TenantExtractionInterceptor()
	var gotCtx context.Context
	handler := func(ctx context.Context, req any) (any, error) {
		gotCtx = ctx
		return nil, nil
	}
	if _, err := interceptor(ctx, nil, &grpc.UnaryServerInfo{}, handler); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if v, ok := tenant.Role(gotCtx); !ok || v != "admin" {
		t.Errorf("want role=\"admin\", got (%q, %v)", v, ok)
	}
	if v, ok := tenant.TenantID(gotCtx); !ok || v != "t1" {
		t.Errorf("want tenant_id=\"t1\", got (%q, %v)", v, ok)
	}
}

// TestTenantExtractionInterceptor_MissingRoleLeavesItAbsent guards backward
// compatibility: a caller that never sends x-orca-role (every existing
// caller, before CR-DS-006 Phase 2) must not have a role fabricated for it.
func TestTenantExtractionInterceptor_MissingRoleLeavesItAbsent(t *testing.T) {
	md := metadata.Pairs(MetadataTenantID, "t1", MetadataUserID, "u1")
	ctx := metadata.NewIncomingContext(context.Background(), md)

	interceptor := TenantExtractionInterceptor()
	var gotCtx context.Context
	handler := func(ctx context.Context, req any) (any, error) {
		gotCtx = ctx
		return nil, nil
	}
	if _, err := interceptor(ctx, nil, &grpc.UnaryServerInfo{}, handler); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if v, ok := tenant.Role(gotCtx); ok || v != "" {
		t.Errorf("want role absent, got (%q, %v)", v, ok)
	}
}
