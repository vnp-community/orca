package usecase

import "github.com/stablyai/orca-go/common/apperrors"

// Error codes of the MCP PAT usecases; api-gateway maps AUTH_MCP_* to the
// CONTRACT MCP_* codes.
const (
	CodeMcpTokenTooLong      = "AUTH_MCP_TOKEN_TOO_LONG"
	CodeMcpScopeNotAllowed   = "AUTH_MCP_SCOPE_NOT_ALLOWED"
	CodeMcpScopeInvalid      = "AUTH_MCP_SCOPE_INVALID"
	CodeMcpTokenNotFound     = "AUTH_MCP_TOKEN_NOT_FOUND"
	CodeMcpTokenInvalidName  = "AUTH_MCP_TOKEN_INVALID_NAME"
	CodeMcpNotConfigured     = "AUTH_MCP_NOT_CONFIGURED"
	mcpTokenSecretPrefix     = "omp_"
	mcpTokenUsePAT           = "mcp_pat"
	mcpTokenUseOAuth         = oauthTokenUseOAuth
	mcpTokenJTIBytes         = 32
	mcpLastUsedRefreshWindow = 5 // minutes
)

func errMcp(kind apperrors.Kind, code, msg string) error { return apperrors.New(kind, code, msg, nil) }
