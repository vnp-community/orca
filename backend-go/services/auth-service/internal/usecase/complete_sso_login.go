package usecase

import (
	"context"

	"github.com/stablyai/orca-go/common/apperrors"
)

// CompleteSsoLoginInput mirrors CompleteSsoLoginRequest 1:1.
type CompleteSsoLoginInput struct {
	Code  string
	State string
}

// CompleteSsoLogin finishes the CR-LOGIN-001 flow: verifies the state token
// minted by StartSsoLogin, exchanges the callback code (+ PKCE verifier)
// for a verified identity against the real provider, then delegates account
// linking/provisioning and session creation to LoginOrProvisionSsoUser.
// Mirrors scm-integration-service's CompleteOAuthFlow shape.
type CompleteSsoLogin struct {
	exchangers SsoExchangerRegistry
	states     SsoStateCodec
	provision  *LoginOrProvisionSsoUser
}

func NewCompleteSsoLogin(exchangers SsoExchangerRegistry, states SsoStateCodec, provision *LoginOrProvisionSsoUser) *CompleteSsoLogin {
	return &CompleteSsoLogin{exchangers: exchangers, states: states, provision: provision}
}

func (uc *CompleteSsoLogin) Execute(ctx context.Context, in CompleteSsoLoginInput) (LoginOrProvisionSsoUserOutput, error) {
	if in.Code == "" {
		return LoginOrProvisionSsoUserOutput{}, apperrors.New(apperrors.KindInvalidArgument, "AUTH_SSO_NO_CODE", "code is required", nil)
	}
	if in.State == "" {
		return LoginOrProvisionSsoUserOutput{}, apperrors.New(apperrors.KindInvalidArgument, "AUTH_SSO_NO_STATE", "state is required", nil)
	}

	state, err := uc.states.Decode(in.State)
	if err != nil {
		return LoginOrProvisionSsoUserOutput{}, apperrors.New(apperrors.KindPermissionDenied, "AUTH_SSO_STATE_INVALID", "sso state token is invalid, expired, or tampered with", err)
	}

	exchanger, err := uc.exchangers.Resolve(state.Provider)
	if err != nil {
		return LoginOrProvisionSsoUserOutput{}, apperrors.New(apperrors.KindInvalidArgument, "AUTH_SSO_PROVIDER_UNSUPPORTED", "no sso configuration registered for this provider", err)
	}

	identity, err := exchanger.ExchangeAndVerify(ctx, in.Code, state.RedirectURI, state.CodeVerifier)
	if err != nil {
		return LoginOrProvisionSsoUserOutput{}, apperrors.New(apperrors.KindInternal, "AUTH_SSO_EXCHANGE_FAILED", "failed to exchange authorization code for a verified identity", err)
	}
	// The state token is the source of truth for which provider this
	// callback belongs to (signed at StartSsoLogin time) — the exchanger
	// was already resolved from it above, so identity.Provider is
	// guaranteed to match; set it explicitly regardless in case a future
	// exchanger implementation forgets to.
	identity.Provider = state.Provider

	return uc.provision.Execute(ctx, identity)
}
