// Package grpcclient implements ai-provider-service's CredentialBrokerClient
// port (internal/usecase/ports.go).
//
// STUB NOTICE: credential-broker-service is not deployed in this workspace
// yet — ai-provider-service and credential-broker-service are Phase 2, built
// and cut over together (specs/backend-go/services/ai-provider-service.md
// §10). This adapter does not dial anything; it synthesizes locally-generated,
// opaque reference IDs so the rest of this service's create/rotate paths
// (and their tests) can be exercised end-to-end before that dependency
// exists. Replace New's body with a real grpc.NewClient dial + generated
// credentialbrokerv1.NewCredentialBrokerServiceClient(conn) once that
// service ships, mapping its CredentialMetadata{Id, Status} onto
// usecase.CredentialRef the same way the stub methods below do.
//
// SECURITY-CRITICAL — read before wiring the real client:
// credential-broker-service's proto already exists at
// proto/orca/credentialbroker/v1/credentialbroker.proto, and its
// ResolveCredentialResponse carries `bytes value = 1` — PLAINTEXT, by that
// proto's own doc comment ("caller must not persist or log").
// ai-provider-service must NEVER call that RPC and must NEVER let that
// field reach this service's process memory, a log line, or any response
// this service returns to its own callers. Per the design doc §9, plaintext
// resolution happens ONLY on the execution plane (the Dev Server Agent, via
// its own narrowly-scoped Vault Transit-decrypt identity) — never in a
// backend service. ResolveCredential below deliberately does NOT call
// credentialbrokerv1's RPC of the same name for exactly this reason: it
// returns opaque status metadata only, mirroring what
// usecase.CredentialRef can express. Do not "complete" this stub by making
// it return a value/plaintext field — there is no such field in
// usecase.CredentialRef, and there must never be one.
package grpcclient

import (
	"context"
	"fmt"

	"github.com/google/uuid"

	"github.com/stablyai/orca-go/services/ai-provider-service/internal/usecase"
)

// CredentialBrokerClient is a stub implementation of
// usecase.CredentialBrokerClient — see package doc.
type CredentialBrokerClient struct {
	// target is retained for the future real dial (e.g.
	// "credential-broker-service:9090"); unused by this stub.
	target string
}

// New returns a stub CredentialBrokerClient. target is accepted (and will
// be used once real dialing is wired) but not connected to here.
func New(target string) *CredentialBrokerClient {
	return &CredentialBrokerClient{target: target}
}

var _ usecase.CredentialBrokerClient = (*CredentialBrokerClient)(nil)

// WriteCredential does not inspect in.EncryptedBlob — a real implementation
// forwards it unopened via credentialbrokerv1.WriteCredential; this stub
// never even needs to look at it to demonstrate that.
func (c *CredentialBrokerClient) WriteCredential(_ context.Context, _ usecase.WriteCredentialInput) (usecase.CredentialRef, error) {
	return usecase.CredentialRef{ID: "stub-cred-" + uuid.NewString(), Status: "pending_push"}, nil
}

// RotateCredential returns a new synthesized opaque ref derived from the
// old one, so callers/tests can observe that a rotation actually changed
// the ref without any real secret material existing anywhere in this call.
func (c *CredentialBrokerClient) RotateCredential(_ context.Context, credentialRef string) (usecase.CredentialRef, error) {
	return usecase.CredentialRef{ID: fmt.Sprintf("%s-rotated-%s", credentialRef, uuid.NewString()), Status: "pending_push"}, nil
}

// ResolveCredential returns opaque status metadata for a ref — see the
// package-level SECURITY-CRITICAL note: this must never be wired to
// credentialbrokerv1's ResolveCredential RPC, which returns plaintext.
func (c *CredentialBrokerClient) ResolveCredential(_ context.Context, credentialRef string) (usecase.CredentialRef, error) {
	return usecase.CredentialRef{ID: credentialRef, Status: "active"}, nil
}
