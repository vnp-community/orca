package domain

import "github.com/stablyai/orca-go/common/apperrors"

// Codes follow CONTRACT-mcp-ui-api.md §2.3. Not-found never distinguishes
// "missing" from "not yours" (cross-tenant reads are indistinguishable).
const (
	CodeInvalidArgument = "MCP_INVALID_ARGUMENT"
	CodeNotFound        = "MCP_NOT_FOUND"
	CodeInternal        = "MCP_INTERNAL"
	CodeNoTenant        = "MCP_NO_TENANT"
)

func ErrTenantSettingsInvalid(msg string) error {
	return apperrors.New(apperrors.KindInvalidArgument, CodeInvalidArgument, msg, nil)
}

func ErrNoTenant(cause error) error {
	return apperrors.New(apperrors.KindUnauthenticated, CodeNoTenant, "no tenant in request context", cause)
}

func ErrInternal(msg string, cause error) error {
	return apperrors.New(apperrors.KindInternal, CodeInternal, msg, cause)
}
