package providerresolver

import (
	"context"
	"errors"
	"testing"

	"google.golang.org/grpc"

	aiproviderv1 "github.com/stablyai/orca-go/proto/gen/go/orca/aiprovider/v1"
	"github.com/stablyai/orca-go/services/workflow-service/internal/domain"
)

// fakeAIProviderClient implements aiproviderv1.AiProviderServiceClient
// directly — embedding the (nil) interface means any RPC this package
// doesn't call panics loudly rather than silently succeeding. "Fake the
// port, not the transport" (specs/backend-go/standards/testing-strategy.md).
type fakeAIProviderClient struct {
	aiproviderv1.AiProviderServiceClient
	resolveProviderFunc func(ctx context.Context, in *aiproviderv1.ResolveProviderRequest, opts ...grpc.CallOption) (*aiproviderv1.ResolveProviderResponse, error)
	listAccountsFunc    func(ctx context.Context, in *aiproviderv1.ListAccountsRequest, opts ...grpc.CallOption) (*aiproviderv1.ListAccountsResponse, error)
}

func (f *fakeAIProviderClient) ResolveProvider(ctx context.Context, in *aiproviderv1.ResolveProviderRequest, opts ...grpc.CallOption) (*aiproviderv1.ResolveProviderResponse, error) {
	return f.resolveProviderFunc(ctx, in, opts...)
}

func (f *fakeAIProviderClient) ListAccounts(ctx context.Context, in *aiproviderv1.ListAccountsRequest, opts ...grpc.CallOption) (*aiproviderv1.ListAccountsResponse, error) {
	return f.listAccountsFunc(ctx, in, opts...)
}

func TestResolve_NoPin_DelegatesToCascadeWithRightScope(t *testing.T) {
	var gotReq *aiproviderv1.ResolveProviderRequest
	client := &fakeAIProviderClient{
		resolveProviderFunc: func(_ context.Context, in *aiproviderv1.ResolveProviderRequest, _ ...grpc.CallOption) (*aiproviderv1.ResolveProviderResponse, error) {
			gotReq = in
			return &aiproviderv1.ResolveProviderResponse{Account: &aiproviderv1.ProviderAccount{Id: "acct-cascade"}}, nil
		},
	}
	r := New(client)
	got, err := r.Resolve(context.Background(), "tenant-1", "user-1", "proj-1", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "acct-cascade" {
		t.Errorf("expected acct-cascade, got %q", got)
	}
	if gotReq.GetTenantId() != "tenant-1" || gotReq.GetUserId() != "user-1" || gotReq.GetProjectId() != "proj-1" {
		t.Errorf("expected tenant/user/project forwarded, got %+v", gotReq)
	}
}

func TestResolve_NoPin_CascadeReturnsNoAccountErrors(t *testing.T) {
	client := &fakeAIProviderClient{
		resolveProviderFunc: func(_ context.Context, _ *aiproviderv1.ResolveProviderRequest, _ ...grpc.CallOption) (*aiproviderv1.ResolveProviderResponse, error) {
			return &aiproviderv1.ResolveProviderResponse{}, nil
		},
	}
	r := New(client)
	_, err := r.Resolve(context.Background(), "tenant-1", "user-1", "proj-1", nil)
	if err == nil {
		t.Fatal("expected an error when the cascade resolves no account")
	}
}

func TestResolve_PinnedActiveAccount_WinsOverCascade(t *testing.T) {
	client := &fakeAIProviderClient{
		resolveProviderFunc: func(_ context.Context, _ *aiproviderv1.ResolveProviderRequest, _ ...grpc.CallOption) (*aiproviderv1.ResolveProviderResponse, error) {
			t.Fatal("cascade should not be called when a pin is set")
			return nil, nil
		},
		listAccountsFunc: func(_ context.Context, in *aiproviderv1.ListAccountsRequest, _ ...grpc.CallOption) (*aiproviderv1.ListAccountsResponse, error) {
			if in.GetTenantId() != "tenant-1" {
				t.Errorf("expected tenant_id=tenant-1, got %q", in.GetTenantId())
			}
			return &aiproviderv1.ListAccountsResponse{Accounts: []*aiproviderv1.ProviderAccount{
				{Id: "acct-other", Status: "active"},
				{Id: "acct-pinned", Status: "active"},
			}}, nil
		},
	}
	r := New(client)
	got, err := r.Resolve(context.Background(), "tenant-1", "user-1", "proj-1", &domain.ProviderPin{AccountID: "acct-pinned"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "acct-pinned" {
		t.Errorf("expected acct-pinned, got %q", got)
	}
}

func TestResolve_PinnedInactiveAccount_ErrorsWithoutFallback(t *testing.T) {
	client := &fakeAIProviderClient{
		resolveProviderFunc: func(_ context.Context, _ *aiproviderv1.ResolveProviderRequest, _ ...grpc.CallOption) (*aiproviderv1.ResolveProviderResponse, error) {
			t.Fatal("cascade should not be called as a fallback for an inactive pin")
			return nil, nil
		},
		listAccountsFunc: func(_ context.Context, _ *aiproviderv1.ListAccountsRequest, _ ...grpc.CallOption) (*aiproviderv1.ListAccountsResponse, error) {
			return &aiproviderv1.ListAccountsResponse{Accounts: []*aiproviderv1.ProviderAccount{
				{Id: "acct-pinned", Status: "revoked"},
			}}, nil
		},
	}
	r := New(client)
	_, err := r.Resolve(context.Background(), "tenant-1", "user-1", "proj-1", &domain.ProviderPin{AccountID: "acct-pinned"})
	if err == nil {
		t.Fatal("expected an error for a pinned but inactive account")
	}
}

func TestResolve_PinnedUnknownAccount_ErrorsWithoutFallback(t *testing.T) {
	client := &fakeAIProviderClient{
		resolveProviderFunc: func(_ context.Context, _ *aiproviderv1.ResolveProviderRequest, _ ...grpc.CallOption) (*aiproviderv1.ResolveProviderResponse, error) {
			t.Fatal("cascade should not be called as a fallback for an unknown pin")
			return nil, nil
		},
		listAccountsFunc: func(_ context.Context, _ *aiproviderv1.ListAccountsRequest, _ ...grpc.CallOption) (*aiproviderv1.ListAccountsResponse, error) {
			return &aiproviderv1.ListAccountsResponse{Accounts: []*aiproviderv1.ProviderAccount{
				{Id: "acct-other", Status: "active"},
			}}, nil
		},
	}
	r := New(client)
	_, err := r.Resolve(context.Background(), "tenant-1", "user-1", "proj-1", &domain.ProviderPin{AccountID: "does-not-exist"})
	if err == nil {
		t.Fatal("expected an error for a pinned account that doesn't exist")
	}
}

func TestResolve_PinnedAccount_ListAccountsFailurePropagates(t *testing.T) {
	client := &fakeAIProviderClient{
		listAccountsFunc: func(_ context.Context, _ *aiproviderv1.ListAccountsRequest, _ ...grpc.CallOption) (*aiproviderv1.ListAccountsResponse, error) {
			return nil, errors.New("ai-provider-service unavailable")
		},
	}
	r := New(client)
	_, err := r.Resolve(context.Background(), "tenant-1", "user-1", "proj-1", &domain.ProviderPin{AccountID: "acct-1"})
	if err == nil {
		t.Fatal("expected the ListAccounts failure to propagate")
	}
}

func TestResolve_EmptyPinAccountID_TreatedAsNoPin(t *testing.T) {
	client := &fakeAIProviderClient{
		resolveProviderFunc: func(_ context.Context, _ *aiproviderv1.ResolveProviderRequest, _ ...grpc.CallOption) (*aiproviderv1.ResolveProviderResponse, error) {
			return &aiproviderv1.ResolveProviderResponse{Account: &aiproviderv1.ProviderAccount{Id: "acct-cascade"}}, nil
		},
	}
	r := New(client)
	got, err := r.Resolve(context.Background(), "tenant-1", "user-1", "proj-1", &domain.ProviderPin{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "acct-cascade" {
		t.Errorf("expected the cascade result when pin.AccountID is empty, got %q", got)
	}
}
