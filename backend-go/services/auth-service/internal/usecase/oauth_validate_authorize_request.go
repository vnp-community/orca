package usecase

import (
	"context"
	"errors"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/services/auth-service/internal/domain"
)

// OAuthAuthorizeParams are the raw /authorize query parameters.
type OAuthAuthorizeParams struct {
	ResponseType        string
	ClientID            string
	RedirectURI         string
	Scope               string
	CodeChallenge       string
	CodeChallengeMethod string
	Resource            string
}

type OAuthAuthorizeRequestInfo struct {
	Client      domain.OAuthClient
	Scopes      []string
	RedirectURI string
	Resource    string
}

// OAuthValidateAuthorizeRequest checks an authorization request without
// needing a user. client_id and redirect_uri are checked first and fail with
// OAUTH_INVALID_CLIENT / OAUTH_INVALID_REDIRECT_URI, the only two errors that
// must never be redirected back (open-redirect guard). PKCE S256 and the
// RFC 8707 resource indicator are mandatory.
type OAuthValidateAuthorizeRequest struct {
	repo OAuthRepository
	cfg  OAuthConfig
}

func NewOAuthValidateAuthorizeRequest(repo OAuthRepository, cfg OAuthConfig) *OAuthValidateAuthorizeRequest {
	return &OAuthValidateAuthorizeRequest{repo: repo, cfg: cfg.WithDefaults()}
}

func (uc *OAuthValidateAuthorizeRequest) Execute(ctx context.Context, p OAuthAuthorizeParams) (OAuthAuthorizeRequestInfo, error) {
	client, err := uc.lookupClient(ctx, p.ClientID)
	if err != nil {
		return OAuthAuthorizeRequestInfo{}, err
	}
	if !domain.MatchRedirectURI(client.RedirectURIs, p.RedirectURI) {
		return OAuthAuthorizeRequestInfo{}, oauthErr(apperrors.KindInvalidArgument, CodeOAuthInvalidRedirectURI, "redirect_uri does not match a registered URI", nil)
	}
	scopes, err := validateAuthorizeCommon(uc.cfg, p.ResponseType, p.CodeChallenge, p.CodeChallengeMethod, p.Resource, p.Scope)
	if err != nil {
		return OAuthAuthorizeRequestInfo{}, err
	}
	return OAuthAuthorizeRequestInfo{Client: client, Scopes: scopes, RedirectURI: p.RedirectURI, Resource: p.Resource}, nil
}

func (uc *OAuthValidateAuthorizeRequest) lookupClient(ctx context.Context, clientID string) (domain.OAuthClient, error) {
	return lookupOAuthClient(ctx, uc.repo, clientID)
}

func lookupOAuthClient(ctx context.Context, repo OAuthRepository, clientID string) (domain.OAuthClient, error) {
	if clientID == "" {
		return domain.OAuthClient{}, oauthErr(apperrors.KindUnauthenticated, CodeOAuthInvalidClient, "client_id is required", nil)
	}
	c, err := repo.GetOAuthClient(ctx, clientID)
	if errors.Is(err, ErrOAuthClientNotFound) {
		return domain.OAuthClient{}, oauthErr(apperrors.KindUnauthenticated, CodeOAuthInvalidClient, "unknown client_id", nil)
	}
	if err != nil {
		return domain.OAuthClient{}, errOAuthInternal("failed to look up client", err)
	}
	return c, nil
}

// validateAuthorizeCommon holds the checks shared by validation and code issuance.
func validateAuthorizeCommon(cfg OAuthConfig, responseType, challenge, method, resource, scope string) ([]string, error) {
	if responseType != "code" {
		return nil, oauthErr(apperrors.KindInvalidArgument, CodeOAuthUnsupportedResponse, "response_type must be \"code\"", nil)
	}
	if err := domain.ValidatePKCEChallenge(challenge, method); err != nil {
		return nil, errOAuthInvalidRequest("PKCE with code_challenge_method=S256 is required")
	}
	if resource == "" || resource != cfg.ResourceURL {
		return nil, oauthErr(apperrors.KindInvalidArgument, CodeOAuthInvalidTarget, "resource must be the MCP resource URL", nil)
	}
	scopes, err := domain.ParseOAuthScopes(scope)
	if err != nil {
		return nil, oauthErr(apperrors.KindInvalidArgument, CodeOAuthInvalidScope, "unknown scope requested", nil)
	}
	if len(scopes) == 0 {
		scopes = []string{domain.OAuthScopeRead}
	}
	return scopes, nil
}
