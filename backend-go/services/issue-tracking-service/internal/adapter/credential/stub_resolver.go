// Package credential implements usecase.CredentialResolver.
//
// STUB PACKAGE: this is not the production implementation.
// issue-tracking-service.md §7/§9 requires per-tenant Jira/Linear
// credentials to come from credential-broker-service's ResolveCredential
// RPC, backed by Vault KV v2 (one path per (tenant, service, user)) — never
// resolved or cached locally beyond the request, never read directly from
// environment variables in production. This scaffold's StubResolver reads
// from environment variables purely so ListIssues/CreateIssue are
// exercisable in local dev without credential-broker-service running.
// Replace this package with a real credential-broker-service gRPC client
// before this service is deployed anywhere real tenant secrets exist —
// same stub-then-wire pattern scm-integration-service's credential
// resolution follows.
package credential

import (
	"context"
	"fmt"
	"os"

	"github.com/stablyai/orca-go/services/issue-tracking-service/internal/domain"
	"github.com/stablyai/orca-go/services/issue-tracking-service/internal/usecase"
)

// StubResolver is the local-dev CredentialResolver — see package doc.
type StubResolver struct{}

func NewStubResolver() *StubResolver {
	return &StubResolver{}
}

var _ usecase.CredentialResolver = (*StubResolver)(nil)

// Resolve ignores tenantID beyond the error message — a real resolver would
// scope the lookup to (tenantID, provider, actingUserID); this stub has no
// per-tenant storage at all, only process-wide env vars.
func (r *StubResolver) Resolve(ctx context.Context, tenantID string, provider domain.Provider) (usecase.Credential, error) {
	switch provider {
	case domain.ProviderJira:
		baseURL := os.Getenv("JIRA_BASE_URL")
		email := os.Getenv("JIRA_EMAIL")
		token := os.Getenv("JIRA_API_TOKEN")
		if baseURL == "" || email == "" || token == "" {
			return usecase.Credential{}, fmt.Errorf(
				"credential: stub resolver has no Jira credential configured for tenant %q "+
					"(set JIRA_BASE_URL/JIRA_EMAIL/JIRA_API_TOKEN, or wire credential-broker-service)", tenantID)
		}
		return usecase.Credential{BaseURL: baseURL, Email: email, Token: token}, nil

	case domain.ProviderLinear:
		token := os.Getenv("LINEAR_API_TOKEN")
		if token == "" {
			return usecase.Credential{}, fmt.Errorf(
				"credential: stub resolver has no Linear credential configured for tenant %q "+
					"(set LINEAR_API_TOKEN, or wire credential-broker-service)", tenantID)
		}
		return usecase.Credential{Token: token}, nil

	default:
		return usecase.Credential{}, fmt.Errorf("credential: unsupported provider %q", provider)
	}
}
