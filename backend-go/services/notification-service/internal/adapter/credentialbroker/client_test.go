package credentialbroker

import (
	"context"
	"testing"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	credentialbrokerv1 "github.com/stablyai/orca-go/proto/gen/go/orca/credentialbroker/v1"
)

// fakeCredentialBrokerServiceClient implements
// credentialbrokerv1.CredentialBrokerServiceClient minimally — only
// ResolveCredentialByOwner (what this package's Resolver actually calls)
// is configurable; every other RPC is unimplemented, mirroring the
// "stub what you don't use" pattern already established for generated
// client fakes elsewhere in this codebase (e.g. notification-service's own
// fakeNotificationServiceClient).
type fakeCredentialBrokerServiceClient struct {
	req  *credentialbrokerv1.ResolveCredentialByOwnerRequest
	resp *credentialbrokerv1.ResolveCredentialByOwnerResponse
	err  error
}

func (f *fakeCredentialBrokerServiceClient) ResolveCredentialByOwner(_ context.Context, in *credentialbrokerv1.ResolveCredentialByOwnerRequest, _ ...grpc.CallOption) (*credentialbrokerv1.ResolveCredentialByOwnerResponse, error) {
	f.req = in
	if f.err != nil {
		return nil, f.err
	}
	return f.resp, nil
}

func (f *fakeCredentialBrokerServiceClient) WriteCredential(context.Context, *credentialbrokerv1.WriteCredentialRequest, ...grpc.CallOption) (*credentialbrokerv1.WriteCredentialResponse, error) {
	return nil, status.Error(codes.Unimplemented, "not used by this test")
}
func (f *fakeCredentialBrokerServiceClient) ResolveCredential(context.Context, *credentialbrokerv1.ResolveCredentialRequest, ...grpc.CallOption) (*credentialbrokerv1.ResolveCredentialResponse, error) {
	return nil, status.Error(codes.Unimplemented, "not used by this test")
}
func (f *fakeCredentialBrokerServiceClient) RotateCredential(context.Context, *credentialbrokerv1.RotateCredentialRequest, ...grpc.CallOption) (*credentialbrokerv1.RotateCredentialResponse, error) {
	return nil, status.Error(codes.Unimplemented, "not used by this test")
}
func (f *fakeCredentialBrokerServiceClient) RevokeCredential(context.Context, *credentialbrokerv1.RevokeCredentialRequest, ...grpc.CallOption) (*credentialbrokerv1.RevokeCredentialResponse, error) {
	return nil, status.Error(codes.Unimplemented, "not used by this test")
}
func (f *fakeCredentialBrokerServiceClient) GetCredentialMetadata(context.Context, *credentialbrokerv1.GetCredentialMetadataRequest, ...grpc.CallOption) (*credentialbrokerv1.GetCredentialMetadataResponse, error) {
	return nil, status.Error(codes.Unimplemented, "not used by this test")
}
func (f *fakeCredentialBrokerServiceClient) RevokeCredentialByOwner(context.Context, *credentialbrokerv1.RevokeCredentialByOwnerRequest, ...grpc.CallOption) (*credentialbrokerv1.RevokeCredentialByOwnerResponse, error) {
	return nil, status.Error(codes.Unimplemented, "not used by this test")
}
func (f *fakeCredentialBrokerServiceClient) SignVapidPayload(context.Context, *credentialbrokerv1.SignVapidPayloadRequest, ...grpc.CallOption) (*credentialbrokerv1.SignVapidPayloadResponse, error) {
	return nil, status.Error(codes.Unimplemented, "not used by this test")
}
func (f *fakeCredentialBrokerServiceClient) GetCredentialMetadataByOwner(context.Context, *credentialbrokerv1.GetCredentialMetadataByOwnerRequest, ...grpc.CallOption) (*credentialbrokerv1.GetCredentialMetadataByOwnerResponse, error) {
	return nil, status.Error(codes.Unimplemented, "not used by this test")
}
func (f *fakeCredentialBrokerServiceClient) ListCredentialsByCategory(context.Context, *credentialbrokerv1.ListCredentialsByCategoryRequest, ...grpc.CallOption) (*credentialbrokerv1.ListCredentialsByCategoryResponse, error) {
	return nil, status.Error(codes.Unimplemented, "not used by this test")
}

// newTestResolver builds a *Resolver directly against a fake client,
// bypassing New's grpc.ClientConnInterface dial step (this package has no
// exported way to inject a client directly, so the test constructs the
// struct literal — acceptable since Resolver has no unexported invariant
// New enforces beyond wrapping the generated client constructor).
func newTestResolver(fake credentialbrokerv1.CredentialBrokerServiceClient) *Resolver {
	return &Resolver{client: fake}
}

func TestResolver_Resolve_CallsResolveCredentialByOwnerWithServiceSecretCategory(t *testing.T) {
	fake := &fakeCredentialBrokerServiceClient{resp: &credentialbrokerv1.ResolveCredentialByOwnerResponse{Value: []byte("secret")}}
	r := newTestResolver(fake)

	if _, err := r.Resolve(context.Background(), "tenant-1", "apns"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if fake.req.GetTenantId() != "tenant-1" {
		t.Errorf("TenantId = %q, want %q", fake.req.GetTenantId(), "tenant-1")
	}
	if fake.req.GetOwnerId() != "apns" {
		t.Errorf("OwnerId = %q, want %q", fake.req.GetOwnerId(), "apns")
	}
	if fake.req.GetCategory() != credentialbrokerv1.CredentialCategory_CREDENTIAL_CATEGORY_SERVICE_SECRET {
		t.Errorf("Category = %v, want CREDENTIAL_CATEGORY_SERVICE_SECRET", fake.req.GetCategory())
	}
}

func TestResolver_Resolve_PropagatesTransportError(t *testing.T) {
	fake := &fakeCredentialBrokerServiceClient{err: status.Error(codes.Unavailable, "broker down")}
	r := newTestResolver(fake)

	if _, err := r.Resolve(context.Background(), "tenant-1", "fcm"); err == nil {
		t.Fatal("expected the transport error to propagate")
	}
}

func TestResolver_Resolve_ReturnsValueBytesUnmodified(t *testing.T) {
	want := []byte(`{"team_id":"T123","key_id":"K456","private_key_pem":"..."}`)
	fake := &fakeCredentialBrokerServiceClient{resp: &credentialbrokerv1.ResolveCredentialByOwnerResponse{Value: want}}
	r := newTestResolver(fake)

	got, err := r.Resolve(context.Background(), "tenant-1", "apns")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(got) != string(want) {
		t.Errorf("got %q, want %q", got, want)
	}
}
