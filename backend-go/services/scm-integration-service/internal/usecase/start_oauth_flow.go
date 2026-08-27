package usecase

import (
	"context"
	"time"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/services/scm-integration-service/internal/domain"
)

// oauthStateTTL bounds how long a StartOAuthFlow state token stays valid —
// long enough for a human to complete a provider's consent screen, short
// enough that a leaked/logged callback URL isn't a standing replay risk.
const oauthStateTTL = 15 * time.Minute

// StartOAuthFlowInput mirrors StartOAuthFlowRequest 1:1.
type StartOAuthFlowInput struct {
	TenantID    string
	UserID      string
	Provider    domain.ScmProvider
	RedirectURI string
}

// StartOAuthFlowResult mirrors StartOAuthFlowResponse 1:1.
type StartOAuthFlowResult struct {
	AuthorizationURL string
	State            string
}

// StartOAuthFlow begins the §9.1 authorization-code web flow: resolves the
// provider's OAuthExchanger, mints a signed state token binding this
// request's tenant/user/provider/redirect_uri, and returns the URL to
// redirect the browser to. No provider HTTP call happens here —
// AuthorizationURL is pure string construction; the provider is only
// contacted once CompleteOAuthFlow exchanges the resulting code.
type StartOAuthFlow struct {
	exchangers OAuthExchangerRegistry
	states     OAuthStateCodec
	now        func() time.Time
}

func NewStartOAuthFlow(exchangers OAuthExchangerRegistry, states OAuthStateCodec, now func() time.Time) *StartOAuthFlow {
	if now == nil {
		now = time.Now
	}
	return &StartOAuthFlow{exchangers: exchangers, states: states, now: now}
}

func (uc *StartOAuthFlow) Execute(ctx context.Context, in StartOAuthFlowInput) (StartOAuthFlowResult, error) {
	if in.TenantID == "" {
		return StartOAuthFlowResult{}, apperrors.New(apperrors.KindInvalidArgument, "SCM_NO_TENANT", "tenant_id is required", nil)
	}
	if in.RedirectURI == "" {
		return StartOAuthFlowResult{}, apperrors.New(apperrors.KindInvalidArgument, "SCM_NO_REDIRECT_URI", "redirect_uri is required", nil)
	}

	exchanger, err := uc.exchangers.Resolve(in.Provider)
	if err != nil {
		return StartOAuthFlowResult{}, apperrors.New(apperrors.KindInvalidArgument, "SCM_PROVIDER_UNSUPPORTED", "no OAuth configuration registered for this provider", err)
	}

	state, err := uc.states.Encode(OAuthState{
		TenantID:    in.TenantID,
		UserID:      in.UserID,
		Provider:    in.Provider,
		RedirectURI: in.RedirectURI,
		ExpiresAt:   uc.now().Add(oauthStateTTL),
	})
	if err != nil {
		return StartOAuthFlowResult{}, apperrors.New(apperrors.KindInternal, "SCM_OAUTH_STATE_ENCODE_FAILED", "failed to mint oauth state token", err)
	}

	return StartOAuthFlowResult{
		AuthorizationURL: exchanger.AuthorizationURL(state, in.RedirectURI),
		State:            state,
	}, nil
}
