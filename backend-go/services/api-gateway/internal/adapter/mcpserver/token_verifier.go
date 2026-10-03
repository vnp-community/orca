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
		switch reason := usecase.McpRejectionReason(err); reason {
		case "":
			return Principal{}, ErrInvalidToken
		case usecase.McpReasonKillSwitch:
			// PATs suspended by the tenant kill switch answer like the kill guard.
			return Principal{}, ErrKillSwitchActive
		default:
			return Principal{}, invalidTokenError{reason: reason}
		}
	}
}

// invalidTokenError is ErrInvalidToken (same 401 answer) plus a bounded reason
// for orca_mcp_auth_failures_total.
type invalidTokenError struct{ reason string }

func (invalidTokenError) Error() string               { return ErrInvalidToken.Error() }
func (invalidTokenError) Is(target error) bool        { return target == ErrInvalidToken }
func (e invalidTokenError) AuthFailureReason() string { return e.reason }

// invalidTokenReason is the metric reason of a verifier error that is answered
// as invalid_token.
func invalidTokenReason(err error) string {
	var r interface{ AuthFailureReason() string }
	if errors.As(err, &r) {
		return r.AuthFailureReason()
	}
	return "invalid_token"
}
