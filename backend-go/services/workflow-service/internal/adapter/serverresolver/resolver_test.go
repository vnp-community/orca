package serverresolver

import (
	"context"
	"errors"
	"sync"
	"testing"

	"google.golang.org/grpc"

	"github.com/stablyai/orca-go/common/tenant"
	infrafleetv1 "github.com/stablyai/orca-go/proto/gen/go/orca/infrafleet/v1"
	projectv1 "github.com/stablyai/orca-go/proto/gen/go/orca/project/v1"
)

// fakeProjectClient implements projectv1.ProjectServiceClient directly —
// embedding the (nil) interface means any RPC this package doesn't call
// panics loudly rather than silently succeeding. "Fake the port, not the
// transport" (specs/backend-go/standards/testing-strategy.md).
type fakeProjectClient struct {
	projectv1.ProjectServiceClient
	getProjectFunc func(ctx context.Context, in *projectv1.GetProjectRequest, opts ...grpc.CallOption) (*projectv1.GetProjectResponse, error)
}

func (f *fakeProjectClient) GetProject(ctx context.Context, in *projectv1.GetProjectRequest, opts ...grpc.CallOption) (*projectv1.GetProjectResponse, error) {
	return f.getProjectFunc(ctx, in, opts...)
}

// fakeInfraFleetClient implements infrafleetv1.InfraFleetServiceClient
// directly — same "fake the port" convention as fakeProjectClient.
type fakeInfraFleetClient struct {
	infrafleetv1.InfraFleetServiceClient
	resolveConnectionFunc   func(ctx context.Context, in *infrafleetv1.ResolveConnectionRequest, opts ...grpc.CallOption) (*infrafleetv1.ResolveConnectionResponse, error)
	listDevServersByTagFunc func(ctx context.Context, in *infrafleetv1.ListDevServersByTagRequest, opts ...grpc.CallOption) (*infrafleetv1.ListDevServersByTagResponse, error)
}

func (f *fakeInfraFleetClient) ResolveConnection(ctx context.Context, in *infrafleetv1.ResolveConnectionRequest, opts ...grpc.CallOption) (*infrafleetv1.ResolveConnectionResponse, error) {
	return f.resolveConnectionFunc(ctx, in, opts...)
}

func (f *fakeInfraFleetClient) ListDevServersByTag(ctx context.Context, in *infrafleetv1.ListDevServersByTagRequest, opts ...grpc.CallOption) (*infrafleetv1.ListDevServersByTagResponse, error) {
	return f.listDevServersByTagFunc(ctx, in, opts...)
}

func withTenantContext(ctx context.Context, tenantID string) context.Context {
	return tenant.WithTenantID(ctx, tenantID)
}

func TestResolve_Empty_ExecutesLocally(t *testing.T) {
	r := New(nil, nil)
	got, err := r.Resolve(withTenantContext(context.Background(), "tenant-1"), "tenant-1", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "" {
		t.Errorf("expected empty connectionId for empty target, got %q", got)
	}
}

func TestResolve_ConnectionPrefix_DirectPassthrough(t *testing.T) {
	r := New(nil, nil)
	got, err := r.Resolve(withTenantContext(context.Background(), "tenant-1"), "tenant-1", "connection:conn-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "conn-1" {
		t.Errorf("expected conn-1, got %q", got)
	}
}

func TestResolve_LegacyBareConnectionID(t *testing.T) {
	r := New(nil, nil)
	got, err := r.Resolve(withTenantContext(context.Background(), "tenant-1"), "tenant-1", "conn-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "conn-1" {
		t.Errorf("expected the bare string passed through unchanged, got %q", got)
	}
}

func TestResolve_ServerPrefix_ResolvesViaInfraFleet(t *testing.T) {
	var gotReq *infrafleetv1.ResolveConnectionRequest
	infra := &fakeInfraFleetClient{
		resolveConnectionFunc: func(_ context.Context, in *infrafleetv1.ResolveConnectionRequest, _ ...grpc.CallOption) (*infrafleetv1.ResolveConnectionResponse, error) {
			gotReq = in
			return &infrafleetv1.ResolveConnectionResponse{Connected: true, ConnectionId: "resolved-conn"}, nil
		},
	}
	r := New(nil, infra)
	got, err := r.Resolve(withTenantContext(context.Background(), "tenant-1"), "tenant-1", "server:ds-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "resolved-conn" {
		t.Errorf("expected resolved-conn, got %q", got)
	}
	if gotReq.GetDevServerId() != "ds-1" {
		t.Errorf("expected dev_server_id=ds-1, got %q", gotReq.GetDevServerId())
	}
}

func TestResolve_ServerPrefix_NotConnectedReturnsError(t *testing.T) {
	infra := &fakeInfraFleetClient{
		resolveConnectionFunc: func(_ context.Context, _ *infrafleetv1.ResolveConnectionRequest, _ ...grpc.CallOption) (*infrafleetv1.ResolveConnectionResponse, error) {
			return &infrafleetv1.ResolveConnectionResponse{Connected: false}, nil
		},
	}
	r := New(nil, infra)
	_, err := r.Resolve(withTenantContext(context.Background(), "tenant-1"), "tenant-1", "server:ds-1")
	if err == nil {
		t.Fatal("expected an error when the dev server has no active connection")
	}
}

func TestResolve_ProjectPrefix_ResolvesViaProjectThenInfraFleet(t *testing.T) {
	projects := &fakeProjectClient{
		getProjectFunc: func(_ context.Context, in *projectv1.GetProjectRequest, _ ...grpc.CallOption) (*projectv1.GetProjectResponse, error) {
			if in.GetId() != "proj-1" {
				t.Errorf("expected project id proj-1, got %q", in.GetId())
			}
			return &projectv1.GetProjectResponse{Project: &projectv1.Project{Id: "proj-1", DevServerId: "ds-1"}}, nil
		},
	}
	var gotReq *infrafleetv1.ResolveConnectionRequest
	infra := &fakeInfraFleetClient{
		resolveConnectionFunc: func(_ context.Context, in *infrafleetv1.ResolveConnectionRequest, _ ...grpc.CallOption) (*infrafleetv1.ResolveConnectionResponse, error) {
			gotReq = in
			return &infrafleetv1.ResolveConnectionResponse{Connected: true, ConnectionId: "resolved-conn"}, nil
		},
	}
	r := New(projects, infra)
	got, err := r.Resolve(withTenantContext(context.Background(), "tenant-1"), "tenant-1", "project:proj-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "resolved-conn" {
		t.Errorf("expected resolved-conn, got %q", got)
	}
	if gotReq.GetDevServerId() != "ds-1" {
		t.Errorf("expected dev_server_id=ds-1 (from the project's dev_server_id), got %q", gotReq.GetDevServerId())
	}
}

func TestResolve_ProjectPrefix_NoDevServerBoundReturnsError(t *testing.T) {
	projects := &fakeProjectClient{
		getProjectFunc: func(_ context.Context, _ *projectv1.GetProjectRequest, _ ...grpc.CallOption) (*projectv1.GetProjectResponse, error) {
			return &projectv1.GetProjectResponse{Project: &projectv1.Project{Id: "proj-1"}}, nil
		},
	}
	r := New(projects, nil)
	_, err := r.Resolve(withTenantContext(context.Background(), "tenant-1"), "tenant-1", "project:proj-1")
	if err == nil {
		t.Fatal("expected an error when the project has no dev_server_id bound")
	}
}

func TestResolve_FleetTag_ZeroHealthyServersReturnsClearError(t *testing.T) {
	infra := &fakeInfraFleetClient{
		listDevServersByTagFunc: func(_ context.Context, in *infrafleetv1.ListDevServersByTagRequest, _ ...grpc.CallOption) (*infrafleetv1.ListDevServersByTagResponse, error) {
			if !in.GetHealthyOnly() {
				t.Error("expected healthy_only=true")
			}
			return &infrafleetv1.ListDevServersByTagResponse{}, nil
		},
	}
	r := New(nil, infra)
	got, err := r.Resolve(withTenantContext(context.Background(), "tenant-1"), "tenant-1", "fleet:tag:gpu")
	if err == nil {
		t.Fatal("expected an error for zero healthy servers")
	}
	if got != "" {
		t.Errorf("expected an empty connectionId on error, got %q", got)
	}
}

func TestResolve_FleetTag_RoundRobinsAcrossHealthyServers(t *testing.T) {
	servers := []*infrafleetv1.DevServer{{Id: "ds-a"}, {Id: "ds-b"}}
	infra := &fakeInfraFleetClient{
		listDevServersByTagFunc: func(_ context.Context, _ *infrafleetv1.ListDevServersByTagRequest, _ ...grpc.CallOption) (*infrafleetv1.ListDevServersByTagResponse, error) {
			return &infrafleetv1.ListDevServersByTagResponse{DevServers: servers}, nil
		},
		resolveConnectionFunc: func(_ context.Context, in *infrafleetv1.ResolveConnectionRequest, _ ...grpc.CallOption) (*infrafleetv1.ResolveConnectionResponse, error) {
			return &infrafleetv1.ResolveConnectionResponse{Connected: true, ConnectionId: "conn-" + in.GetDevServerId()}, nil
		},
	}
	r := New(nil, infra)
	ctx := withTenantContext(context.Background(), "tenant-1")

	seen := map[string]bool{}
	var mu sync.Mutex
	var wg sync.WaitGroup
	for range 20 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			got, err := r.Resolve(ctx, "tenant-1", "fleet:tag:gpu")
			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}
			mu.Lock()
			seen[got] = true
			mu.Unlock()
		}()
	}
	wg.Wait()

	if !seen["conn-ds-a"] || !seen["conn-ds-b"] {
		t.Errorf("expected both healthy servers to be selected over repeated calls, got %v", seen)
	}
}

func TestResolve_FleetTag_ListFailurePropagates(t *testing.T) {
	infra := &fakeInfraFleetClient{
		listDevServersByTagFunc: func(_ context.Context, _ *infrafleetv1.ListDevServersByTagRequest, _ ...grpc.CallOption) (*infrafleetv1.ListDevServersByTagResponse, error) {
			return nil, errors.New("infra-fleet-service unavailable")
		},
	}
	r := New(nil, infra)
	_, err := r.Resolve(withTenantContext(context.Background(), "tenant-1"), "tenant-1", "fleet:tag:gpu")
	if err == nil {
		t.Fatal("expected the list failure to propagate")
	}
}
