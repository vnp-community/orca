package domain

import "github.com/stablyai/orca-go/common/apperrors"

// Error codes of the consent/grant surface (CONTRACT-mcp-ui-api.md section 2.3).
// MCP_CLIENT_NOT_ALLOWED is gateway-internal: the authorize endpoint turns it
// into a redirect with error=access_denied and never sends it over WS.
const (
	CodeConsentNotFound  = "MCP_CONSENT_NOT_FOUND"
	CodeConsentExpired   = "MCP_CONSENT_EXPIRED"
	CodeScopeInvalid     = "MCP_SCOPE_INVALID"
	CodeScopeNotAllowed  = "MCP_SCOPE_NOT_ALLOWED"
	CodeNotAdmin         = "MCP_NOT_ADMIN"
	CodeClientNotAllowed = "MCP_CLIENT_NOT_ALLOWED"
	CodeDisabled         = "MCP_DISABLED"
	CodeUnavailable      = "MCP_UNAVAILABLE"
)

// ErrConsentNotFound deliberately covers "does not exist", "already decided",
// "belongs to another user" and "belongs to another tenant": callers cannot
// probe for other people's consent requests.
func ErrConsentNotFound() error {
	return apperrors.New(apperrors.KindNotFound, CodeConsentNotFound, "consent request not found", nil)
}

func ErrConsentExpired() error {
	return apperrors.New(apperrors.KindFailedPrecondition, CodeConsentExpired, "consent request has expired", nil)
}

func ErrScopeInvalid(msg string) error {
	return apperrors.New(apperrors.KindInvalidArgument, CodeScopeInvalid, msg, nil)
}

func ErrScopeNotAllowed(msg string) error {
	return apperrors.New(apperrors.KindPermissionDenied, CodeScopeNotAllowed, msg, nil)
}

func ErrNotAdmin() error {
	return apperrors.New(apperrors.KindPermissionDenied, CodeNotAdmin, "administrator role required", nil)
}

// ErrNotFound never distinguishes "missing" from "not yours".
func ErrNotFound() error {
	return apperrors.New(apperrors.KindNotFound, CodeNotFound, "not found", nil)
}

func ErrClientNotAllowed() error {
	return apperrors.New(apperrors.KindPermissionDenied, CodeClientNotAllowed, "client is not allowed in this tenant", nil)
}

func ErrDisabled() error {
	return apperrors.New(apperrors.KindFailedPrecondition, CodeDisabled, "MCP is disabled for this tenant", nil)
}

func ErrInvalidArgument(msg string) error {
	return apperrors.New(apperrors.KindInvalidArgument, CodeInvalidArgument, msg, nil)
}
