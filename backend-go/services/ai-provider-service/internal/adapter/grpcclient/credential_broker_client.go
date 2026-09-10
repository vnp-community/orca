// Package grpcclient implements ai-provider-service's CredentialBrokerClient
// port (internal/usecase/ports.go) against a real credential-broker-service
// gRPC connection — Epic B (docs/execution-plan.md §8).
//
// SECURITY-CRITICAL — read before touching this file:
// credential-broker-service's ResolveCredential RPC returns
// ResolveCredentialResponse{bytes value} — PLAINTEXT, by that proto's own
// doc comment ("caller must not persist or log"). ai-provider-service must
// NEVER call that RPC and must NEVER let that field reach this service's
// process memory, a log line, or any response this service returns to its
// own callers. Per architecture/06-secrets-vault-architecture.md §9,
// plaintext resolution happens ONLY on the execution plane (the Dev Server
// Agent, via its own narrowly-scoped Vault Transit-decrypt identity) —
// never in a backend service. ResolveCredential below calls
// GetCredentialMetadata instead, a dedicated metadata-only RPC added
// specifically so this constraint holds for real rather than by
// convention — see credentialbroker.proto's doc comment on that RPC. Do
// not "simplify" this by switching it to call credentialbrokerv1's
// ResolveCredential — there is no plaintext field in usecase.CredentialRef,
// and there must never be one.
package grpcclient

import (
	"context"
	"fmt"

	"google.golang.org/grpc"

	"github.com/stablyai/orca-go/services/ai-provider-service/internal/usecase"

	credentialbrokerv1 "github.com/stablyai/orca-go/proto/gen/go/orca/credentialbroker/v1"
)

// CredentialBrokerClient implements usecase.CredentialBrokerClient against
// a real credential-broker-service connection.
type CredentialBrokerClient struct {
	client credentialbrokerv1.CredentialBrokerServiceClient
}

// New wraps an already-dialed connection to credential-broker-service (see
// cmd/server/main.go's composition root for the dial + insecure-transport-
// credentials rationale shared by every peer-service client in this
// workspace).
func New(conn grpc.ClientConnInterface) *CredentialBrokerClient {
	return &CredentialBrokerClient{client: credentialbrokerv1.NewCredentialBrokerServiceClient(conn)}
}

var _ usecase.CredentialBrokerClient = (*CredentialBrokerClient)(nil)

// credentialCategory is this service's one fixed category — every
// credential ai-provider-service ever writes/rotates/resolves is an AI
// provider API key, never any other kind.
const credentialCategory = credentialbrokerv1.CredentialCategory_CREDENTIAL_CATEGORY_AI_PROVIDER_KEY

// WriteCredential forwards in.EncryptedBlob unopened to
// credentialbrokerv1.WriteCredential — this adapter never inspects it, per
// the port's own doc comment.
func (c *CredentialBrokerClient) WriteCredential(ctx context.Context, in usecase.WriteCredentialInput) (usecase.CredentialRef, error) {
	resp, err := c.client.WriteCredential(ctx, &credentialbrokerv1.WriteCredentialRequest{
		TenantId:          in.TenantID,
		OwnerId:           in.OwnerID,
		Category:          credentialCategory,
		EncryptedEnvelope: in.EncryptedBlob,
	})
	if err != nil {
		return usecase.CredentialRef{}, fmt.Errorf("grpcclient: credential-broker-service WriteCredential: %w", err)
	}
	return toCredentialRef(resp.GetMetadata()), nil
}

// RotateCredential asks credential-broker-service to rotate credentialRef's
// underlying material. This scaffold has no PushCiphertext/client-side-crypto
// integration yet (see this service's README "Known gaps"), so, exactly as
// WriteCredential already documents as an accepted scaffold simplification,
// NewEncryptedEnvelope goes over the wire empty — the plumbing (ref
// changes, rotating-status transition) is real and exercised end to end;
// only the "new material actually differs from the old" step is not, until
// a real client-side crypto integration exists to produce one.
func (c *CredentialBrokerClient) RotateCredential(ctx context.Context, credentialRef string) (usecase.CredentialRef, error) {
	resp, err := c.client.RotateCredential(ctx, &credentialbrokerv1.RotateCredentialRequest{
		CredentialId: credentialRef,
	})
	if err != nil {
		return usecase.CredentialRef{}, fmt.Errorf("grpcclient: credential-broker-service RotateCredential: %w", err)
	}
	return toCredentialRef(resp.GetMetadata()), nil
}

// ResolveCredential returns opaque status metadata for a ref — see the
// package-level SECURITY-CRITICAL note: this calls GetCredentialMetadata,
// never ResolveCredential, which returns plaintext.
func (c *CredentialBrokerClient) ResolveCredential(ctx context.Context, credentialRef string) (usecase.CredentialRef, error) {
	resp, err := c.client.GetCredentialMetadata(ctx, &credentialbrokerv1.GetCredentialMetadataRequest{
		CredentialId: credentialRef,
	})
	if err != nil {
		return usecase.CredentialRef{}, fmt.Errorf("grpcclient: credential-broker-service GetCredentialMetadata: %w", err)
	}
	return toCredentialRef(resp.GetMetadata()), nil
}

// RevokeCredential asks credential-broker-service to revoke credentialRef —
// used by CreateAccount's test-before-save rollback path (TASK-AIP-01-06)
// when a just-written credential fails its live connection test. Never
// touches a secret value; the broker handles Vault revocation on its own
// side of this call.
func (c *CredentialBrokerClient) RevokeCredential(ctx context.Context, credentialRef string) error {
	if _, err := c.client.RevokeCredential(ctx, &credentialbrokerv1.RevokeCredentialRequest{
		CredentialId: credentialRef,
	}); err != nil {
		return fmt.Errorf("grpcclient: credential-broker-service RevokeCredential: %w", err)
	}
	return nil
}

func toCredentialRef(m *credentialbrokerv1.CredentialMetadata) usecase.CredentialRef {
	return usecase.CredentialRef{ID: m.GetId(), Status: m.GetStatus()}
}
