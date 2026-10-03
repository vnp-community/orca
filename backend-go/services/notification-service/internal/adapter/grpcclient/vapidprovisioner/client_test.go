package vapidprovisioner

import (
	"context"
	"errors"
	"testing"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	"github.com/stablyai/orca-go/services/notification-service/internal/usecase"

	credentialbrokerv1 "github.com/stablyai/orca-go/proto/gen/go/orca/credentialbroker/v1"
)

type stubBroker struct {
	credentialbrokerv1.CredentialBrokerServiceClient
	err error
	md  metadata.MD
}

func (s *stubBroker) EnsureVapidSigningKey(ctx context.Context, _ *credentialbrokerv1.EnsureVapidSigningKeyRequest, _ ...grpc.CallOption) (*credentialbrokerv1.EnsureVapidSigningKeyResponse, error) {
	s.md, _ = metadata.FromOutgoingContext(ctx)
	if s.err != nil {
		return nil, s.err
	}
	return &credentialbrokerv1.EnsureVapidSigningKeyResponse{PublicKey: "PUB"}, nil
}

func TestClient_SendsIdentityAndMapsForbidden(t *testing.T) {
	b := &stubBroker{}
	pub, err := NewWithClient(b).EnsureVapidSigningKey(context.Background(), "tenant-1")
	if err != nil || pub != "PUB" {
		t.Fatalf("%q %v", pub, err)
	}
	if b.md.Get("x-orca-service-id")[0] != "notification-service" || b.md.Get("x-orca-tenant-id")[0] != "tenant-1" {
		t.Fatalf("metadata: %v", b.md)
	}

	b.err = status.Error(codes.PermissionDenied, "CREDBROKER_VAULT_FORBIDDEN: apply policy")
	if _, err := NewWithClient(b).EnsureVapidSigningKey(context.Background(), "t"); !errors.Is(err, usecase.ErrVapidProvisionForbidden) {
		t.Fatalf("want forbidden, got %v", err)
	}

	b.err = status.Error(codes.PermissionDenied, "CREDENTIAL_CALLER_NOT_ALLOWED: nope")
	if _, err := NewWithClient(b).EnsureVapidSigningKey(context.Background(), "t"); err == nil || errors.Is(err, usecase.ErrVapidProvisionForbidden) {
		t.Fatalf("caller-not-allowed must not look like a Vault policy issue: %v", err)
	}
}
