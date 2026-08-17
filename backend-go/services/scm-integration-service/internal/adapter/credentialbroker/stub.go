// Package credentialbroker is scm-integration-service's client to
// credential-broker-service — the source of truth for per-tenant OAuth
// tokens per scm-integration-service.md §7/§9.
//
// STUB — credential-broker-service doesn't exist as a running service in
// this scaffold. StubResolver returns a fake, clearly-marked token string so
// the usecase layer's provider-dispatch logic (internal/usecase) is
// exercisable end-to-end without a live dependency. Replace this file with
// a real gRPC client call to credential-broker-service — scoped to
// (tenant_id, provider, user_id) per §9 — before this service is deployed
// anywhere real tenant credentials matter. See scm-integration-service.md §7.
package credentialbroker

import (
	"context"
	"fmt"

	"github.com/stablyai/orca-go/services/scm-integration-service/internal/domain"
	"github.com/stablyai/orca-go/services/scm-integration-service/internal/usecase"
)

// StubResolver implements usecase.CredentialResolver with a fake token.
type StubResolver struct{}

// NewStubResolver returns a StubResolver. See package doc — TODO: replace
// with a real credential-broker-service gRPC client.
func NewStubResolver() *StubResolver {
	return &StubResolver{}
}

// Resolve returns a fake, obviously-not-a-real-OAuth-token string. It never
// contacts any external system — this is what makes it a stub.
func (r *StubResolver) Resolve(_ context.Context, tenantID string, provider domain.ScmProvider) (usecase.Credential, error) {
	// STUB token — never a real OAuth access token, and never persisted or
	// logged past this in-memory struct. See package doc comment.
	return usecase.Credential{Token: fmt.Sprintf("stub-credential-broker-token:%s:%s", tenantID, provider)}, nil
}
