package grpcclient

import (
	"context"
	"errors"
	"testing"

	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"

	"github.com/stablyai/orca-go/common/grpcmw"
	"github.com/stablyai/orca-go/common/tenant"
	infrafleetv1 "github.com/stablyai/orca-go/proto/gen/go/orca/infrafleet/v1"
	tenantv1 "github.com/stablyai/orca-go/proto/gen/go/orca/tenant/v1"
)

// fakeTenantServiceClient implements tenantv1.TenantServiceClient directly —
// same fake-the-generated-client-port convention task-service/other
// grpcclient packages already use (embed: panics on any unimplemented
// method, intentional for these tests).
type fakeTenantServiceClient struct {
	tenantv1.TenantServiceClient

	resolvedSettingsJSON string
	err                  error

	gotCtx context.Context
	gotReq *tenantv1.GetResolvedProfileRequest
}

func (f *fakeTenantServiceClient) GetResolvedProfile(ctx context.Context, in *tenantv1.GetResolvedProfileRequest, _ ...grpc.CallOption) (*tenantv1.GetResolvedProfileResponse, error) {
	f.gotCtx = ctx
	f.gotReq = in
	if f.err != nil {
		return nil, f.err
	}
	return &tenantv1.GetResolvedProfileResponse{ResolvedSettingsJson: f.resolvedSettingsJSON}, nil
}

// fakeInfraFleetServiceClientForProfile implements
// infrafleetv1.InfraFleetServiceClient — unused by these tests
// (GetResolvedProfile never calls it) but required to construct
// TenantProfileResolver.
type fakeInfraFleetServiceClientForProfile struct {
	infrafleetv1.InfraFleetServiceClient
}

// TestTenantProfileResolver_GetResolvedProfile_ForwardsTenantMetadata is the
// regression test for BUG-008: the outbound call to tenant-service must
// carry the caller's tenant ID as outgoing gRPC metadata (the same key
// grpcmw.TenantExtractionInterceptor reads inbound), or tenant-service's own
// tenant.RequireTenantID fails closed.
func TestTenantProfileResolver_GetResolvedProfile_ForwardsTenantMetadata(t *testing.T) {
	fake := &fakeTenantServiceClient{resolvedSettingsJSON: `{"fleet":{"allowedServerTags":["gpu"]}}`}
	r := NewTenantProfileResolver(fake, &fakeInfraFleetServiceClientForProfile{})

	ctx := tenant.WithTenantID(context.Background(), "tenant-1")
	if _, err := r.GetResolvedProfile(ctx, "tenant-1", "user-1"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if fake.gotCtx == nil {
		t.Fatal("expected tenant-service's GetResolvedProfile to be called")
	}
	md, ok := metadata.FromOutgoingContext(fake.gotCtx)
	if !ok {
		t.Fatal("expected outbound call to carry gRPC metadata")
	}
	got := md.Get(grpcmw.MetadataTenantID)
	if len(got) != 1 || got[0] != "tenant-1" {
		t.Errorf("expected outgoing metadata %q=[tenant-1], got %v", grpcmw.MetadataTenantID, got)
	}
	if fake.gotReq.GetUserId() != "user-1" {
		t.Errorf("expected user_id to pass through verbatim, got %q", fake.gotReq.GetUserId())
	}
}

// TestTenantProfileResolver_GetResolvedProfile_NoTenantInContext is the
// fail-closed counterpart: without a tenant in ctx, withTenantMetadata must
// error before the outbound RPC is attempted at all.
func TestTenantProfileResolver_GetResolvedProfile_NoTenantInContext(t *testing.T) {
	fake := &fakeTenantServiceClient{}
	r := NewTenantProfileResolver(fake, &fakeInfraFleetServiceClientForProfile{})

	if _, err := r.GetResolvedProfile(context.Background(), "", "user-1"); !errors.Is(err, tenant.ErrNoTenant) {
		t.Errorf("expected tenant.ErrNoTenant, got %v", err)
	}
	if fake.gotCtx != nil {
		t.Error("expected tenant-service's GetResolvedProfile not to be called without a tenant in context")
	}
}

// TestTenantProfileResolver_GetResolvedProfile_NoAllowedServerTags covers
// the existing (unchanged-by-this-fix) branch where fleet.allowedServerTags
// is absent from the resolved profile.
func TestTenantProfileResolver_GetResolvedProfile_NoAllowedServerTags(t *testing.T) {
	fake := &fakeTenantServiceClient{resolvedSettingsJSON: `{}`}
	r := NewTenantProfileResolver(fake, &fakeInfraFleetServiceClientForProfile{})

	ctx := tenant.WithTenantID(context.Background(), "tenant-1")
	view, err := r.GetResolvedProfile(ctx, "tenant-1", "user-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, hasRestriction := view.AllowedServerTags(); hasRestriction {
		t.Error("expected no restriction when fleet.allowedServerTags is absent")
	}
}
