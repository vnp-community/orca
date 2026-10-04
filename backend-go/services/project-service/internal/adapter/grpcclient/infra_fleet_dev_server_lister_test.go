package grpcclient

import (
	"context"
	"testing"

	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"

	"github.com/stablyai/orca-go/common/grpcmw"
	"github.com/stablyai/orca-go/common/tenant"
	infrafleetv1 "github.com/stablyai/orca-go/proto/gen/go/orca/infrafleet/v1"
)

// fakeInfraFleetServiceClientForDevServerLister implements
// infrafleetv1.InfraFleetServiceClient, recording the ctx ListDevServers was
// called with — same fake-the-generated-client-port convention as
// profile_resolver_test.go's fakeTenantServiceClient.
type fakeInfraFleetServiceClientForDevServerLister struct {
	infrafleetv1.InfraFleetServiceClient

	devServers []*infrafleetv1.DevServer
	err        error

	gotCtx   context.Context
	callSeen bool
}

func (f *fakeInfraFleetServiceClientForDevServerLister) ListDevServers(ctx context.Context, _ *infrafleetv1.ListDevServersRequest, _ ...grpc.CallOption) (*infrafleetv1.ListDevServersResponse, error) {
	f.callSeen = true
	f.gotCtx = ctx
	if f.err != nil {
		return nil, f.err
	}
	return &infrafleetv1.ListDevServersResponse{DevServers: f.devServers}, nil
}

func outboundTenantID(t *testing.T, ctx context.Context) string {
	t.Helper()
	md, ok := metadata.FromOutgoingContext(ctx)
	if !ok {
		t.Fatal("expected outbound call to carry gRPC metadata")
	}
	got := md.Get(grpcmw.MetadataTenantID)
	if len(got) != 1 {
		t.Fatalf("expected exactly one outgoing %q value, got %v", grpcmw.MetadataTenantID, got)
	}
	return got[0]
}

// TestInfraFleetDevServerLister_Exists_ForwardsTenantMetadata is the
// regression test for BUG-010: without this, infra-fleet-service's own
// tenant.RequireTenantID fails closed with INFRA_NO_TENANT — confirmed live
// on b15.openledger.vn (see BUG-010's log evidence).
func TestInfraFleetDevServerLister_Exists_ForwardsTenantMetadata(t *testing.T) {
	fake := &fakeInfraFleetServiceClientForDevServerLister{
		devServers: []*infrafleetv1.DevServer{{Id: "ds-1"}},
	}
	lister := &InfraFleetDevServerLister{client: fake}

	ctx := tenant.WithTenantID(context.Background(), "tenant-1")
	exists, err := lister.Exists(ctx, "tenant-1", "ds-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !exists {
		t.Error("expected ds-1 to be found")
	}
	if !fake.callSeen {
		t.Fatal("expected ListDevServers to be called")
	}
	if got := outboundTenantID(t, fake.gotCtx); got != "tenant-1" {
		t.Errorf("expected outgoing metadata %q=tenant-1, got %q", grpcmw.MetadataTenantID, got)
	}
}

// TestInfraFleetDevServerLister_Exists_NoTenantInContext is the fail-closed
// counterpart — withTenantMetadata must error before the outbound RPC is
// attempted at all, same as profile_resolver_test.go's sibling case.
func TestInfraFleetDevServerLister_Exists_NoTenantInContext(t *testing.T) {
	fake := &fakeInfraFleetServiceClientForDevServerLister{}
	lister := &InfraFleetDevServerLister{client: fake}

	_, err := lister.Exists(context.Background(), "", "ds-1")
	if err == nil {
		t.Fatal("expected an error when ctx carries no tenant")
	}
	if fake.callSeen {
		t.Error("expected ListDevServers NOT to be called when tenant is missing")
	}
}

// TestInfraFleetHostnameResolver_Hostname_ForwardsTenantMetadata mirrors the
// Exists test above — same bug, same fix, second call site (BUG-010).
func TestInfraFleetHostnameResolver_Hostname_ForwardsTenantMetadata(t *testing.T) {
	fake := &fakeInfraFleetServiceClientForDevServerLister{
		devServers: []*infrafleetv1.DevServer{{Id: "ds-1", Host: "test-01"}},
	}
	resolver := &InfraFleetHostnameResolver{client: fake}

	ctx := tenant.WithTenantID(context.Background(), "tenant-1")
	host, err := resolver.Hostname(ctx, "tenant-1", "ds-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if host != "test-01" {
		t.Errorf("expected host %q, got %q", "test-01", host)
	}
	if got := outboundTenantID(t, fake.gotCtx); got != "tenant-1" {
		t.Errorf("expected outgoing metadata %q=tenant-1, got %q", grpcmw.MetadataTenantID, got)
	}
}

// TestInfraFleetHostnameResolver_Hostname_NoTenantInContext is the
// fail-closed counterpart for the second call site.
func TestInfraFleetHostnameResolver_Hostname_NoTenantInContext(t *testing.T) {
	fake := &fakeInfraFleetServiceClientForDevServerLister{}
	resolver := &InfraFleetHostnameResolver{client: fake}

	_, err := resolver.Hostname(context.Background(), "", "ds-1")
	if err == nil {
		t.Fatal("expected an error when ctx carries no tenant")
	}
	if fake.callSeen {
		t.Error("expected ListDevServers NOT to be called when tenant is missing")
	}
}
