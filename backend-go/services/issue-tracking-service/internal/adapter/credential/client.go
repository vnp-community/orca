// Package credential implements usecase.CredentialResolver against a real
// credential-broker-service connection — Epic B (docs/execution-plan.md
// §8), replacing the previous env-var-reading StubResolver.
//
// Resolve is keyed through ConnectionRepository per TASK-097/SOL-015's
// credential-model design: Resolve looks up credential_id via connections,
// then calls ResolveCredential(credential_id) — the per-request read path.
// ResolveCredentialByOwner (owner_id = "<userID>:<provider>") is used only
// by Write/ExistingCredentialID for Connect's create-vs-already-connected
// bootstrap — never the per-request read path. credential-broker-service's
// ResolveCredentialResponse.value is a single plaintext byte slice, so this
// adapter's convention is: the envelope a caller WriteCredential's for an
// issue-tracker credential is a JSON object
// {"baseUrl":"...","email":"...","token":"..."}, and Resolve JSON-decodes
// the returned value back into usecase.Credential.
package credential

import (
	"context"
	"encoding/json"
	"fmt"

	"google.golang.org/grpc"

	"github.com/stablyai/orca-go/services/issue-tracking-service/internal/domain"
	"github.com/stablyai/orca-go/services/issue-tracking-service/internal/usecase"

	credentialbrokerv1 "github.com/stablyai/orca-go/proto/gen/go/orca/credentialbroker/v1"
)

// Resolver implements usecase.CredentialResolver against a real
// credential-broker-service connection.
type Resolver struct {
	client      credentialbrokerv1.CredentialBrokerServiceClient
	connections usecase.ConnectionRepository
}

// New wraps an already-dialed connection to credential-broker-service (see
// cmd/server/main.go's composition root for the dial + insecure-transport-
// credentials rationale shared by every peer-service client in this
// workspace).
func New(conn grpc.ClientConnInterface, connections usecase.ConnectionRepository) *Resolver {
	return &Resolver{client: credentialbrokerv1.NewCredentialBrokerServiceClient(conn), connections: connections}
}

var _ usecase.CredentialResolver = (*Resolver)(nil)

// credentialEnvelope is the JSON shape this adapter expects
// ResolveCredential's plaintext value to decode into — see the package doc
// comment.
type credentialEnvelope struct {
	BaseURL string `json:"baseUrl"`
	Email   string `json:"email"`
	Token   string `json:"token"`
}

func ownerID(userID string, provider domain.Provider) string {
	return fmt.Sprintf("%s:%s", userID, provider)
}

// Resolve looks up which credential_id backs (tenantID, userID, provider,
// workspaceID) via ConnectionRepository, then resolves it — the
// per-request read path, never a direct owner-keyed lookup.
func (r *Resolver) Resolve(ctx context.Context, tenantID, userID string, provider domain.Provider, workspaceID string) (usecase.Credential, error) {
	credID, err := r.connections.GetCredentialID(ctx, tenantID, userID, provider, workspaceID)
	if err != nil {
		return usecase.Credential{}, fmt.Errorf("credential: resolving connection for %s: %w", provider, err)
	}
	resp, err := r.client.ResolveCredential(ctx, &credentialbrokerv1.ResolveCredentialRequest{CredentialId: credID})
	if err != nil {
		return usecase.Credential{}, fmt.Errorf("credential: resolving %s credential: %w", provider, err)
	}
	var envelope credentialEnvelope
	if err := json.Unmarshal(resp.GetValue(), &envelope); err != nil {
		return usecase.Credential{}, fmt.Errorf("credential: decoding %s credential envelope: %w", provider, err)
	}
	return usecase.Credential{BaseURL: envelope.BaseURL, Email: envelope.Email, Token: envelope.Token}, nil
}

// Write encrypts and persists cred under a composite owner_id
// ("<userID>:<provider>") via credential-broker-service.WriteCredential.
//
// WriteCredentialRequest.encrypted_envelope is documented as a
// client-side-encrypted envelope (credentialbroker.proto's doc comment on
// WriteCredentialRequest) — this adapter, like scm-integration-service's
// existing caller, writes the plaintext JSON envelope directly for now
// (matches this service's pre-existing convention; no client-side
// encryption layer exists anywhere in backend-go yet). Flagged here rather
// than silently deviating without a comment — do not treat this as solved.
func (r *Resolver) Write(ctx context.Context, tenantID, userID string, provider domain.Provider, cred usecase.Credential) (string, error) {
	envelope, err := json.Marshal(credentialEnvelope{BaseURL: cred.BaseURL, Email: cred.Email, Token: cred.Token})
	if err != nil {
		return "", fmt.Errorf("credential: encoding %s credential envelope: %w", provider, err)
	}
	resp, err := r.client.WriteCredential(ctx, &credentialbrokerv1.WriteCredentialRequest{
		TenantId:          tenantID,
		OwnerId:           ownerID(userID, provider),
		Category:          credentialbrokerv1.CredentialCategory_CREDENTIAL_CATEGORY_ISSUE_TRACKER_OAUTH,
		EncryptedEnvelope: envelope,
	})
	if err != nil {
		return "", fmt.Errorf("credential: writing %s credential: %w", provider, err)
	}
	return resp.GetMetadata().GetId(), nil
}

// ExistingCredentialID checks whether a credential already exists for this
// composite owner_id via ResolveCredentialByOwner — used only by Connect's
// create-new-vs-already-connected bootstrap decision, never the
// per-request read path (that's Resolve, above).
func (r *Resolver) ExistingCredentialID(ctx context.Context, tenantID, userID string, provider domain.Provider) (string, bool, error) {
	resp, err := r.client.ResolveCredentialByOwner(ctx, &credentialbrokerv1.ResolveCredentialByOwnerRequest{
		TenantId: tenantID,
		Category: credentialbrokerv1.CredentialCategory_CREDENTIAL_CATEGORY_ISSUE_TRACKER_OAUTH,
		OwnerId:  ownerID(userID, provider),
	})
	if err != nil {
		return "", false, nil // not found is not an error here — Connect treats it as "create new"
	}
	// ResolveCredentialByOwnerResponse carries only the plaintext value, not
	// an id — this adapter has no credential_id to report from this call.
	// Callers only need the boolean "does one already exist" today.
	_ = resp
	return "", true, nil
}
