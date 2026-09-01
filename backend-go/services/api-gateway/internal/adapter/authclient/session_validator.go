// Package authclient implements real (non-placeholder) session validation
// against auth-service, for the two callers that need actual identity
// resolution rather than usecase.AuthValidator's unverified-JWT placeholder:
// the /auth/local login endpoint (httpgateway.AuthRoutes) and the wscompat
// WS transport, both of which see the browser's real orca_session cookie
// value (a raw, high-entropy token — never a JWT), not a bearer token.
package authclient

import (
	"context"
	"fmt"
	"net/http"

	authv1 "github.com/stablyai/orca-go/proto/gen/go/orca/auth/v1"
	"github.com/stablyai/orca-go/services/api-gateway/internal/adapter/wscompat"
)

// SessionCookieName matches usecase.AuthValidator.SessionCookieName — kept
// as its own constant here rather than importing that package's meaning
// into this one, since the two will diverge once AuthValidator's own
// cookie path is upgraded to call this same validator (see
// docs/execution-plan.md Epic D).
const SessionCookieName = "orca_session"

// SessionValidator validates a raw session token against auth-service's
// real ValidateSession RPC — unlike usecase.AuthValidator, this performs an
// actual server-side lookup (auth-service hashes the token and checks
// expiry/revocation), not just unverified claim extraction.
type SessionValidator struct {
	client authv1.AuthServiceClient
}

func New(client authv1.AuthServiceClient) *SessionValidator {
	return &SessionValidator{client: client}
}

// ValidateCookie reads the orca_session cookie from r and validates it
// against auth-service. Returns wscompat.Identity so callers compose with
// the same AttachIdentity helper every REST route already uses.
func (v *SessionValidator) ValidateCookie(ctx context.Context, r *http.Request) (wscompat.Identity, error) {
	cookie, err := r.Cookie(SessionCookieName)
	if err != nil || cookie.Value == "" {
		return wscompat.Identity{}, fmt.Errorf("authclient: no %s cookie present", SessionCookieName)
	}
	return v.ValidateToken(ctx, cookie.Value)
}

// ValidateToken validates a raw session token directly — used by
// ValidateCookie and reusable anywhere else a raw token (not an HTTP
// request) needs validating.
func (v *SessionValidator) ValidateToken(ctx context.Context, token string) (wscompat.Identity, error) {
	resp, err := v.client.ValidateSession(ctx, &authv1.ValidateSessionRequest{SessionToken: token})
	if err != nil {
		return wscompat.Identity{}, fmt.Errorf("authclient: validating session: %w", err)
	}
	if !resp.GetValid() || resp.GetUser() == nil {
		return wscompat.Identity{}, fmt.Errorf("authclient: session invalid or expired")
	}
	user := resp.GetUser()
	return wscompat.Identity{TenantID: user.GetTenantId(), UserID: user.GetId(), Role: roleString(user.GetRole())}, nil
}

// roleString maps auth-service's proto Role enum to the lowercase strings
// domain.Role/common/tenant.Role use — CR-DS-006 Phase 2's first real
// consumer of auth-service's Role field past this point (previously
// discarded here entirely, see this function's call site's history).
func roleString(r authv1.Role) string {
	switch r {
	case authv1.Role_ROLE_ADMIN:
		return "admin"
	case authv1.Role_ROLE_USER:
		return "user"
	default:
		return ""
	}
}
