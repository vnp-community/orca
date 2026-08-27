package wscompat

import (
	"context"
	"testing"

	"google.golang.org/grpc"
	"google.golang.org/protobuf/types/known/emptypb"

	infrafleetv1 "github.com/stablyai/orca-go/proto/gen/go/orca/infrafleet/v1"
)

// fakeBrowserProfileClient is scoped to the 5 RPCs browser.profile*
// channels call: the 3 Postgres-backed CRUD RPCs plus ResolveConnection +
// Relay for the 3 live-agent profile ops.
type fakeBrowserProfileClient struct {
	infrafleetv1.InfraFleetServiceClient

	listFunc              func(ctx context.Context, in *infrafleetv1.ListBrowserProfilesRequest) (*infrafleetv1.ListBrowserProfilesResponse, error)
	createFunc            func(ctx context.Context, in *infrafleetv1.CreateBrowserProfileRequest) (*infrafleetv1.CreateBrowserProfileResponse, error)
	deleteFunc            func(ctx context.Context, in *infrafleetv1.DeleteBrowserProfileRequest) (*emptypb.Empty, error)
	resolveConnectionFunc func(ctx context.Context, in *infrafleetv1.ResolveConnectionRequest) (*infrafleetv1.ResolveConnectionResponse, error)
	relayFunc             func(ctx context.Context, in *infrafleetv1.RelayRequest) (*infrafleetv1.RelayResponse, error)
}

func (f *fakeBrowserProfileClient) ListBrowserProfiles(ctx context.Context, in *infrafleetv1.ListBrowserProfilesRequest, _ ...grpc.CallOption) (*infrafleetv1.ListBrowserProfilesResponse, error) {
	return f.listFunc(ctx, in)
}

func (f *fakeBrowserProfileClient) CreateBrowserProfile(ctx context.Context, in *infrafleetv1.CreateBrowserProfileRequest, _ ...grpc.CallOption) (*infrafleetv1.CreateBrowserProfileResponse, error) {
	return f.createFunc(ctx, in)
}

func (f *fakeBrowserProfileClient) DeleteBrowserProfile(ctx context.Context, in *infrafleetv1.DeleteBrowserProfileRequest, _ ...grpc.CallOption) (*emptypb.Empty, error) {
	return f.deleteFunc(ctx, in)
}

func (f *fakeBrowserProfileClient) ResolveConnection(ctx context.Context, in *infrafleetv1.ResolveConnectionRequest, _ ...grpc.CallOption) (*infrafleetv1.ResolveConnectionResponse, error) {
	return f.resolveConnectionFunc(ctx, in)
}

func (f *fakeBrowserProfileClient) Relay(ctx context.Context, in *infrafleetv1.RelayRequest, _ ...grpc.CallOption) (*infrafleetv1.RelayResponse, error) {
	return f.relayFunc(ctx, in)
}

func TestBrowserProfileList_Success(t *testing.T) {
	var gotReq *infrafleetv1.ListBrowserProfilesRequest
	fake := &fakeBrowserProfileClient{
		listFunc: func(_ context.Context, in *infrafleetv1.ListBrowserProfilesRequest) (*infrafleetv1.ListBrowserProfilesResponse, error) {
			gotReq = in
			return &infrafleetv1.ListBrowserProfilesResponse{Profiles: []*infrafleetv1.BrowserProfile{
				{Id: "bp-1", DevServerId: "ds-1", Name: "Work", SourceBrowser: "chrome", IsDefault: true},
			}}, nil
		},
	}
	r := NewRegistry()
	registerBrowserProfileChannels(r, fake)

	args := argsJSON(t, map[string]any{"devServerId": "ds-1"})
	result, err := r.Dispatch(context.Background(), Identity{TenantID: "t1"}, "browser.profileList", args)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotReq.GetDevServerId() != "ds-1" {
		t.Errorf("DevServerId = %q, want ds-1", gotReq.GetDevServerId())
	}
	views, ok := result.([]map[string]any)
	if !ok || len(views) != 1 {
		t.Fatalf("unexpected result: %#v", result)
	}
	if views[0]["id"] != "bp-1" || views[0]["isDefault"] != true {
		t.Errorf("unexpected view: %#v", views[0])
	}
}

func TestBrowserProfileCreate_Success(t *testing.T) {
	var gotReq *infrafleetv1.CreateBrowserProfileRequest
	fake := &fakeBrowserProfileClient{
		createFunc: func(_ context.Context, in *infrafleetv1.CreateBrowserProfileRequest) (*infrafleetv1.CreateBrowserProfileResponse, error) {
			gotReq = in
			return &infrafleetv1.CreateBrowserProfileResponse{Profile: &infrafleetv1.BrowserProfile{
				Id: "bp-new", DevServerId: in.GetDevServerId(), Name: in.GetName(),
			}}, nil
		},
	}
	r := NewRegistry()
	registerBrowserProfileChannels(r, fake)

	args := argsJSON(t, map[string]any{"devServerId": "ds-1", "name": "Work", "sourceBrowser": "chrome", "isDefault": false})
	result, err := r.Dispatch(context.Background(), Identity{TenantID: "t1"}, "browser.profileCreate", args)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotReq.GetName() != "Work" || gotReq.GetDevServerId() != "ds-1" {
		t.Errorf("unexpected request: %#v", gotReq)
	}
	view, ok := result.(map[string]any)
	if !ok || view["id"] != "bp-new" {
		t.Fatalf("unexpected result: %#v", result)
	}
}

func TestBrowserProfileDelete_Success(t *testing.T) {
	var gotReq *infrafleetv1.DeleteBrowserProfileRequest
	fake := &fakeBrowserProfileClient{
		deleteFunc: func(_ context.Context, in *infrafleetv1.DeleteBrowserProfileRequest) (*emptypb.Empty, error) {
			gotReq = in
			return &emptypb.Empty{}, nil
		},
	}
	r := NewRegistry()
	registerBrowserProfileChannels(r, fake)

	args := argsJSON(t, map[string]any{"id": "bp-1"})
	result, err := r.Dispatch(context.Background(), Identity{TenantID: "t1"}, "browser.profileDelete", args)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotReq.GetId() != "bp-1" {
		t.Errorf("Id = %q, want bp-1", gotReq.GetId())
	}
	if got, ok := result.(map[string]bool); !ok || !got["ok"] {
		t.Errorf("unexpected result: %#v", result)
	}
}

func TestBrowserProfileRelayOps_RequiresDevServerID_FailsFastWithoutResolving(t *testing.T) {
	cases := []string{"browser.profileClearDefaultCookies", "browser.profileDetectBrowsers", "browser.profileImportFromBrowser"}
	for _, channel := range cases {
		t.Run(channel, func(t *testing.T) {
			called := false
			fake := &fakeBrowserProfileClient{
				resolveConnectionFunc: func(context.Context, *infrafleetv1.ResolveConnectionRequest) (*infrafleetv1.ResolveConnectionResponse, error) {
					called = true
					return nil, nil
				},
			}
			r := NewRegistry()
			registerBrowserProfileChannels(r, fake)

			args := argsJSON(t, map[string]any{}) // no devServerId
			_, err := r.Dispatch(context.Background(), Identity{TenantID: "t1"}, channel, args)
			if err == nil {
				t.Fatal("expected BROWSER_NO_DEV_SERVER error")
			}
			if called {
				t.Error("ResolveConnection must not be called when devServerId is missing")
			}
		})
	}
}

func TestBrowserProfileRelayOps_ResolvesDevServerThenRelays(t *testing.T) {
	cases := []string{"browser.profileClearDefaultCookies", "browser.profileDetectBrowsers", "browser.profileImportFromBrowser"}
	for _, channel := range cases {
		t.Run(channel, func(t *testing.T) {
			var gotResolve *infrafleetv1.ResolveConnectionRequest
			var gotRelay *infrafleetv1.RelayRequest
			fake := &fakeBrowserProfileClient{
				resolveConnectionFunc: func(_ context.Context, in *infrafleetv1.ResolveConnectionRequest) (*infrafleetv1.ResolveConnectionResponse, error) {
					gotResolve = in
					return &infrafleetv1.ResolveConnectionResponse{Connected: true, ConnectionId: "conn-1"}, nil
				},
				relayFunc: func(_ context.Context, in *infrafleetv1.RelayRequest) (*infrafleetv1.RelayResponse, error) {
					gotRelay = in
					return &infrafleetv1.RelayResponse{ResultJson: `{"ok":true}`}, nil
				},
			}
			r := NewRegistry()
			registerBrowserProfileChannels(r, fake)

			args := argsJSON(t, map[string]any{"devServerId": "ds-1"})
			result, err := r.Dispatch(context.Background(), Identity{TenantID: "t1"}, channel, args)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if gotResolve.GetDevServerId() != "ds-1" {
				t.Errorf("DevServerId = %q, want ds-1", gotResolve.GetDevServerId())
			}
			if gotRelay.GetConnectionId() != "conn-1" {
				t.Errorf("ConnectionId = %q, want conn-1", gotRelay.GetConnectionId())
			}
			if gotRelay.GetMethod() != channel {
				t.Errorf("Method = %q, want %q", gotRelay.GetMethod(), channel)
			}
			if result.(map[string]any)["ok"] != true {
				t.Errorf("unexpected result: %#v", result)
			}
		})
	}
}

func TestBrowserProfileRelayOps_NotConnected_Errors(t *testing.T) {
	fake := &fakeBrowserProfileClient{
		resolveConnectionFunc: func(context.Context, *infrafleetv1.ResolveConnectionRequest) (*infrafleetv1.ResolveConnectionResponse, error) {
			return &infrafleetv1.ResolveConnectionResponse{Connected: false}, nil
		},
	}
	r := NewRegistry()
	registerBrowserProfileChannels(r, fake)

	args := argsJSON(t, map[string]any{"devServerId": "ds-1"})
	_, err := r.Dispatch(context.Background(), Identity{TenantID: "t1"}, "browser.profileDetectBrowsers", args)
	if err == nil {
		t.Fatal("expected BROWSER_NO_CONNECTION error")
	}
}
