package mcpserver

import (
	"context"
	"errors"
	"net/http"

	"github.com/stablyai/orca-go/services/api-gateway/internal/usecase"
)

// MCPValidator is the slice of usecase.AuthValidator the verifier needs.
type MCPValidator interface {
	ValidateMCP(r *http.Request, resourceURL string) (usecase.McpPrincipal, error)
}

// BearerTokenVerifier is the real TokenVerifier: it accepts only OAuth access
// tokens and PATs whose aud is exactly the MCP resource, still active in
// auth-service, and never looks at cookies.
type BearerTokenVerifier struct {
	validator   MCPValidator
	resourceURL string
}

func NewBearerTokenVerifier(v MCPValidator, resourceURL string) *BearerTokenVerifier {
	return &BearerTokenVerifier{validator: v, resourceURL: resourceURL}
}

func (b *BearerTokenVerifier) Verify(_ context.Context, r *http.Request) (Principal, error) {
	p, err := b.validator.ValidateMCP(r, b.resourceURL)
	switch {
	case err == nil:
		return Principal{
			TenantID: p.TenantID, UserID: p.UserID, Role: p.Role, ClientID: p.ClientID,
			TokenID: p.JTI, GrantID: p.GrantID, Scopes: p.Scopes, ExpiresAt: p.ExpiresAt,
			Depth: p.McpDepth, Root: p.McpRoot,
		}, nil
	case errors.Is(err, usecase.ErrNoCredential):
		return Principal{}, ErrNoCredentials
	case errors.Is(err, usecase.ErrPrincipalLookupFailed):
		return Principal{}, ErrVerifierUnavailable
	default:
		return Principal{}, ErrInvalidToken
	}
}
