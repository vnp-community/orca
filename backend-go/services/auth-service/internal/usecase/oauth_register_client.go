package usecase

import (
	"context"
	"errors"
	"slices"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/services/auth-service/internal/domain"
)

const (
	oauthGrantAuthorizationCode = "authorization_code"
	oauthGrantRefreshToken      = "refresh_token"
	oauthClientIDBytes          = 32
)

// OAuthRegisterClientInput mirrors RFC 7591 client metadata (public clients only).
type OAuthRegisterClientInput struct {
	ClientName              string
	ClientURI               string
	RedirectURIs            []string
	TokenEndpointAuthMethod string
	GrantTypes              []string
	ResponseTypes           []string
}

type OAuthRegisterClientOutput struct {
	Client        domain.OAuthClient
	GrantTypes    []string
	ResponseTypes []string
}

// OAuthRegisterClient implements anonymous Dynamic Client Registration. It
// issues no secret: every client is public and must use PKCE.
type OAuthRegisterClient struct {
	repo  OAuthRepository
	audit oauthAuditor
	clock Clock
	cfg   OAuthConfig
}

func NewOAuthRegisterClient(repo OAuthRepository, audit AuditRepository, clock Clock, cfg OAuthConfig) *OAuthRegisterClient {
	return &OAuthRegisterClient{repo: repo, audit: oauthAuditor{audit: audit, clock: clock}, clock: clock, cfg: cfg.WithDefaults()}
}

func invalidMetadata(msg string) error {
	return oauthErr(apperrors.KindInvalidArgument, CodeOAuthInvalidClientMetadata, msg, nil)
}

func (uc *OAuthRegisterClient) Execute(ctx context.Context, in OAuthRegisterClientInput) (OAuthRegisterClientOutput, error) {
	if !uc.cfg.DCREnabled {
		return OAuthRegisterClientOutput{}, oauthErr(apperrors.KindFailedPrecondition, CodeOAuthDCRDisabled, "dynamic client registration is disabled", nil)
	}
	if m := in.TokenEndpointAuthMethod; m != "" && m != "none" {
		return OAuthRegisterClientOutput{}, invalidMetadata("only token_endpoint_auth_method \"none\" is supported")
	}
	grants := in.GrantTypes
	if len(grants) == 0 {
		grants = []string{oauthGrantAuthorizationCode, oauthGrantRefreshToken}
	}
	for _, g := range grants {
		if g != oauthGrantAuthorizationCode && g != oauthGrantRefreshToken {
			return OAuthRegisterClientOutput{}, invalidMetadata("unsupported grant_types")
		}
	}
	if !slices.Contains(grants, oauthGrantAuthorizationCode) {
		return OAuthRegisterClientOutput{}, invalidMetadata("grant_types must include authorization_code")
	}
	responses := in.ResponseTypes
	if len(responses) == 0 {
		responses = []string{"code"}
	}
	if len(responses) != 1 || responses[0] != "code" {
		return OAuthRegisterClientOutput{}, invalidMetadata("response_types must be [\"code\"]")
	}
	name, err := domain.SanitizeClientName(in.ClientName)
	if err != nil {
		return OAuthRegisterClientOutput{}, invalidMetadata("client_name must be 1-100 characters and must not contain a URL")
	}
	if err := domain.ValidateClientURI(in.ClientURI); err != nil {
		return OAuthRegisterClientOutput{}, invalidMetadata("client_uri must be an https URL")
	}
	if err := domain.ValidateRedirectURIs(in.RedirectURIs); err != nil {
		code := CodeOAuthInvalidClientMetadata
		if !errors.Is(err, domain.ErrOAuthTooManyRedirectURIs) {
			code = CodeOAuthInvalidRedirectURI
		}
		return OAuthRegisterClientOutput{}, oauthErr(apperrors.KindInvalidArgument, code, "redirect_uris must be 1-5 distinct https URLs (http only for loopback), without wildcards or fragments", nil)
	}

	n, err := uc.repo.CountOAuthClients(ctx)
	if err != nil {
		return OAuthRegisterClientOutput{}, errOAuthInternal("failed to count clients", err)
	}
	if n >= uc.cfg.DCRMaxClients {
		return OAuthRegisterClientOutput{}, oauthErr(apperrors.KindFailedPrecondition, CodeOAuthDCRLimitReached, "client registration limit reached", nil)
	}

	clientID, err := generateRandomToken(oauthClientIDBytes)
	if err != nil {
		return OAuthRegisterClientOutput{}, errOAuthInternal("failed to generate client id", err)
	}
	client := domain.OAuthClient{
		ClientID:      clientID,
		ClientName:    name,
		ClientURI:     in.ClientURI,
		RedirectURIs:  slices.Clone(in.RedirectURIs),
		RegisteredVia: domain.OAuthRegisteredViaDCR,
		CreatedAt:     uc.clock.Now(),
	}
	if err := uc.repo.CreateOAuthClient(ctx, client); err != nil {
		return OAuthRegisterClientOutput{}, errOAuthInternal("failed to store client", err)
	}
	uc.audit.record(ctx, "", "", "oauth.client_registered", "oauth_client", clientID,
		map[string]any{"client_name": name, "redirect_uri_count": len(client.RedirectURIs)}, domain.OutcomeAllowed)
	return OAuthRegisterClientOutput{Client: client, GrantTypes: grants, ResponseTypes: responses}, nil
}
