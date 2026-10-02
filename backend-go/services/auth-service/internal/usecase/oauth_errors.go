package usecase

import "github.com/stablyai/orca-go/common/apperrors"

// RFC 6749 / 7591 / 8707 error identifiers carried in apperrors.Code. The
// gateway turns them into {"error","error_description"}; messages here are
// generic on purpose and never include a code, token or verifier.
const (
	CodeOAuthInvalidRequest        = "OAUTH_INVALID_REQUEST"
	CodeOAuthInvalidClient         = "OAUTH_INVALID_CLIENT"
	CodeOAuthInvalidRedirectURI    = "OAUTH_INVALID_REDIRECT_URI"
	CodeOAuthInvalidGrant          = "OAUTH_INVALID_GRANT"
	CodeOAuthInvalidScope          = "OAUTH_INVALID_SCOPE"
	CodeOAuthInvalidTarget         = "OAUTH_INVALID_TARGET"
	CodeOAuthUnauthorizedClient    = "OAUTH_UNAUTHORIZED_CLIENT"
	CodeOAuthUnsupportedGrantType  = "OAUTH_UNSUPPORTED_GRANT_TYPE"
	CodeOAuthUnsupportedResponse   = "OAUTH_UNSUPPORTED_RESPONSE_TYPE"
	CodeOAuthInvalidClientMetadata = "OAUTH_INVALID_CLIENT_METADATA"
	CodeOAuthDCRDisabled           = "OAUTH_DCR_DISABLED"
	CodeOAuthDCRLimitReached       = "OAUTH_DCR_LIMIT_REACHED"
	CodeOAuthClientNotFound        = "OAUTH_CLIENT_NOT_FOUND"
	CodeOAuthAccessDenied          = "OAUTH_ACCESS_DENIED"
	CodeOAuthInternal              = "OAUTH_INTERNAL"
)

func oauthErr(kind apperrors.Kind, code, msg string, cause error) error {
	return apperrors.New(kind, code, msg, cause)
}

func errOAuthInvalidRequest(msg string) error {
	return oauthErr(apperrors.KindInvalidArgument, CodeOAuthInvalidRequest, msg, nil)
}

func errOAuthInvalidGrant(msg string) error {
	return oauthErr(apperrors.KindInvalidArgument, CodeOAuthInvalidGrant, msg, nil)
}

func errOAuthInternal(msg string, cause error) error {
	return oauthErr(apperrors.KindInternal, CodeOAuthInternal, msg, cause)
}
