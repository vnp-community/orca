// Package credential implements usecase.CredentialResolver against a real
// credential-broker-service connection — Epic B (docs/execution-plan.md
// §8), replacing the previous env-var-reading StubResolver.
//
// Lookup key and wire shape: like scm-integration-service (see that
// service's internal/adapter/credentialbroker package doc comment for the
// full rationale), this service is only ever handed (tenantID, provider) —
// never an opaque credential_id — so it resolves via
// credentialbrokerv1.ResolveCredentialByOwner, owner_id = the provider
// name ("jira"/"linear"). Unlike scm-integration-service's Credential
// (a single opaque token), usecase.Credential here has three fields
// (BaseURL, Email, Token — Jira needs all three; Linear needs only Token).
// credential-broker-service's ResolveCredentialByOwnerResponse.value is a
// single plaintext byte slice, so this adapter's convention is: the
// envelope a caller WriteCredential's for an issue-tracker credential is a
// JSON object {"baseUrl":"...","email":"...","token":"..."}, and Resolve
// JSON-decodes the returned value back into usecase.Credential. There is no
// WriteCredential caller for this category anywhere in this scaffold yet
// (no "connect Jira/Linear" flow exists — see this service's README "Known
// gaps"), so this convention is documented here for whenever that flow is
// built, not yet exercised end to end against a real written credential.
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
	client credentialbrokerv1.CredentialBrokerServiceClient
}

// New wraps an already-dialed connection to credential-broker-service (see
// cmd/server/main.go's composition root for the dial + insecure-transport-
// credentials rationale shared by every peer-service client in this
// workspace).
func New(conn grpc.ClientConnInterface) *Resolver {
	return &Resolver{client: credentialbrokerv1.NewCredentialBrokerServiceClient(conn)}
}

var _ usecase.CredentialResolver = (*Resolver)(nil)

// credentialEnvelope is the JSON shape this adapter expects
// ResolveCredentialByOwner's plaintext value to decode into — see the
// package doc comment.
type credentialEnvelope struct {
	BaseURL string `json:"baseUrl"`
	Email   string `json:"email"`
	Token   string `json:"token"`
}

// Resolve fetches this tenant's Jira/Linear credential. Like
// scm-integration-service, this service is on credentialbroker.proto's
// documented authorized-caller list for plaintext resolution — holding the
// credential for the duration of one outbound Jira/Linear API call is this
// service's job.
func (r *Resolver) Resolve(ctx context.Context, tenantID string, provider domain.Provider) (usecase.Credential, error) {
	resp, err := r.client.ResolveCredentialByOwner(ctx, &credentialbrokerv1.ResolveCredentialByOwnerRequest{
		TenantId: tenantID,
		Category: credentialbrokerv1.CredentialCategory_CREDENTIAL_CATEGORY_ISSUE_TRACKER_OAUTH,
		OwnerId:  string(provider),
	})
	if err != nil {
		return usecase.Credential{}, fmt.Errorf("credential: resolving %s credential: %w", provider, err)
	}

	var envelope credentialEnvelope
	if err := json.Unmarshal(resp.GetValue(), &envelope); err != nil {
		return usecase.Credential{}, fmt.Errorf("credential: decoding %s credential envelope: %w", provider, err)
	}
	return usecase.Credential{BaseURL: envelope.BaseURL, Email: envelope.Email, Token: envelope.Token}, nil
}
