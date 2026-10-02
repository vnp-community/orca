package domain

import "github.com/stablyai/orca-go/common/apperrors"

// Codes of the external server registry (CONTRACT-mcp-ui-api.md section 2.3).
const (
	CodeServerSSRFBlocked     = "MCP_SERVER_SSRF_BLOCKED"
	CodeServerInvalid         = "MCP_SERVER_INVALID"
	CodeServerStdioNotAllowed = "MCP_SERVER_STDIO_NOT_ALLOWED"
	CodeServerDigestMismatch  = "MCP_SERVER_DIGEST_MISMATCH"
	CodeServerNameConflict    = "MCP_SERVER_NAME_CONFLICT"
	CodeServerNotApproved     = "MCP_SERVER_NOT_APPROVED"
)

// ErrSSRFBlocked messages are static or derived from the URL shape only; they
// never include a resolved address.
func ErrSSRFBlocked(msg string) error {
	return apperrors.New(apperrors.KindInvalidArgument, CodeServerSSRFBlocked, msg, nil)
}

func ErrServerInvalid(msg string) error {
	return apperrors.New(apperrors.KindInvalidArgument, CodeServerInvalid, msg, nil)
}

func ErrStdioNotAllowed(msg string) error {
	return apperrors.New(apperrors.KindPermissionDenied, CodeServerStdioNotAllowed, msg, nil)
}

func ErrDigestMismatch() error {
	return apperrors.New(apperrors.KindFailedPrecondition, CodeServerDigestMismatch, "tools changed since the last probe; probe again before reviewing", nil)
}

func ErrNameConflict() error {
	return apperrors.New(apperrors.KindAlreadyExists, CodeServerNameConflict, "a server with this name already exists in this scope", nil)
}

func ErrServerNotApproved() error {
	return apperrors.New(apperrors.KindFailedPrecondition, CodeServerNotApproved, "server is not approved", nil)
}

// ErrUnavailable reports a missing or failing dependency (broker, tenant
// service). The cause is kept for logs only; the message stays generic.
func ErrUnavailable(msg string, cause error) error {
	return apperrors.New(apperrors.KindInternal, CodeUnavailable, msg, cause)
}

// ErrProbeFailed classifies a probe failure that is not an SSRF block. The
// text is a short fixed reason (unreachable, timeout, bad_response, ...).
type ErrProbeFailed struct{ Reason string }

func (e ErrProbeFailed) Error() string { return "probe failed: " + e.Reason }
