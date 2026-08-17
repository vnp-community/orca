// Package credentialbroker is scm-integration-service's client to
// credential-broker-service — the source of truth for per-tenant OAuth
// tokens per scm-integration-service.md §7/§9. Real as of Epic B
// (docs/execution-plan.md §8).
//
// Lookup key: this service is only ever handed (tenantID, provider) — it
// never receives an opaque credential_id, because nothing in this
// scaffold's RPC surface has a "connect this SCM account" write flow yet
// (see this package's README "Known gaps"). credentialbrokerv1's
// ResolveCredentialByOwner exists specifically for this shape of caller:
// resolve by (tenant_id, category, owner_id) instead of by id. owner_id is
// the provider name itself (e.g. "github", "gitlab") — see
// ownerIDForProvider below — since CREDENTIAL_CATEGORY_SCM_OAUTH is one
// category shared by every SCM provider (a tenant with both a GitHub and a
// GitLab connection has two SCM_OAUTH-category rows, distinguished only by
// owner_id).
package credentialbroker

import (
	"context"
	"fmt"

	"google.golang.org/grpc"

	"github.com/stablyai/orca-go/services/scm-integration-service/internal/domain"
	"github.com/stablyai/orca-go/services/scm-integration-service/internal/usecase"

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

// Resolve fetches this tenant's OAuth token for provider via
// ResolveCredentialByOwner. The returned Credential.Token holds the
// resolved plaintext bytes as-is — this service is on
// credentialbroker.proto's documented authorized-caller list for plaintext
// resolution (unlike ai-provider-service, see that service's
// grpcclient package doc comment), since holding the token for the
// duration of one outbound provider API call is exactly this service's
// job (usecase package doc comment: "the only place a token value lives
// outside an adapter's own HTTP auth-header construction").
func (r *Resolver) Resolve(ctx context.Context, tenantID string, provider domain.ScmProvider) (usecase.Credential, error) {
	resp, err := r.client.ResolveCredentialByOwner(ctx, &credentialbrokerv1.ResolveCredentialByOwnerRequest{
		TenantId: tenantID,
		Category: credentialbrokerv1.CredentialCategory_CREDENTIAL_CATEGORY_SCM_OAUTH,
		OwnerId:  string(provider),
	})
	if err != nil {
		return usecase.Credential{}, fmt.Errorf("credentialbroker: resolving %s credential: %w", provider, err)
	}
	return usecase.Credential{Token: string(resp.GetValue())}, nil
}

var _ usecase.CredentialWriter = (*Resolver)(nil)

// Write implements usecase.CredentialWriter — the one write path into
// credential-broker-service, exercised for real as of Phase 3
// (docs/execution-plan.md §3)'s OAuth flow (CompleteOAuthFlow is this
// method's only caller). owner_id is the provider name, mirroring
// Resolve's own by-owner convention above.
//
// encrypted_envelope: WriteCredentialRequest's proto doc comment describes
// a client-side-encrypted envelope, but no service in this codebase
// implements that half of the design yet — credential-broker-service's own
// WriteCredential usecase treats the field as OPAQUE BYTES end to end and
// forwards it straight into Vault Transit (see that service's
// write_credential.go doc comment and README "Known gaps"); ai-provider-
// service's only non-nil caller is a pure passthrough of externally-
// supplied bytes, never something it constructs itself. There is no
// established client-side sealing step anywhere to reuse. Given that, the
// token bytes are sent here as-is: Vault Transit at the broker is what
// protects them at rest, matching this codebase's actual (not aspirational)
// contract today — see this service's README "Known gaps" for the same
// honesty this repo already applies to ai-provider-service's equivalent gap.
func (r *Resolver) Write(ctx context.Context, tenantID string, provider domain.ScmProvider, token usecase.OAuthToken) error {
	_, err := r.client.WriteCredential(ctx, &credentialbrokerv1.WriteCredentialRequest{
		TenantId:          tenantID,
		OwnerId:           string(provider),
		Category:          credentialbrokerv1.CredentialCategory_CREDENTIAL_CATEGORY_SCM_OAUTH,
		EncryptedEnvelope: []byte(token.AccessToken),
	})
	if err != nil {
		return fmt.Errorf("credentialbroker: writing %s credential: %w", provider, err)
	}
	return nil
}

var _ usecase.CredentialRevoker = (*Resolver)(nil)

// RevokeByOwner implements usecase.CredentialRevoker via
// RevokeCredentialByOwner — RevokeAuth's only call site, closing the gap
// that usecase's prior doc comment flagged (this service never receives an
// opaque credential_id, and RevokeCredential's by-id RPC was unreachable
// from here). Same (tenant_id, category, owner_id) triple Resolve/Write
// already use.
func (r *Resolver) RevokeByOwner(ctx context.Context, tenantID string, provider domain.ScmProvider) error {
	_, err := r.client.RevokeCredentialByOwner(ctx, &credentialbrokerv1.RevokeCredentialByOwnerRequest{
		TenantId: tenantID,
		Category: credentialbrokerv1.CredentialCategory_CREDENTIAL_CATEGORY_SCM_OAUTH,
		OwnerId:  string(provider),
	})
	if err != nil {
		return fmt.Errorf("credentialbroker: revoking %s credential: %w", provider, err)
	}
	return nil
}
