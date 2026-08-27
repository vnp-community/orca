package usecase

import (
	"context"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/services/scm-integration-service/internal/domain"
)

// CompleteOAuthFlowInput mirrors CompleteOAuthFlowRequest 1:1.
type CompleteOAuthFlowInput struct {
	TenantID    string
	UserID      string
	Provider    domain.ScmProvider
	Code        string
	State       string
	RedirectURI string
}

// CompleteOAuthFlow finishes the §9.1 authorization-code web flow: verifies
// the state token minted by StartOAuthFlow, exchanges the callback code for
// an access token against the real provider, and writes the result to
// credential-broker-service. This is the one place in this service that
// ever calls WriteCredential.
type CompleteOAuthFlow struct {
	exchangers OAuthExchangerRegistry
	states     OAuthStateCodec
	writer     CredentialWriter
}

func NewCompleteOAuthFlow(exchangers OAuthExchangerRegistry, states OAuthStateCodec, writer CredentialWriter) *CompleteOAuthFlow {
	return &CompleteOAuthFlow{exchangers: exchangers, states: states, writer: writer}
}

func (uc *CompleteOAuthFlow) Execute(ctx context.Context, in CompleteOAuthFlowInput) (bool, error) {
	if in.TenantID == "" {
		return false, apperrors.New(apperrors.KindInvalidArgument, "SCM_NO_TENANT", "tenant_id is required", nil)
	}
	if in.Code == "" {
		return false, apperrors.New(apperrors.KindInvalidArgument, "SCM_NO_CODE", "code is required", nil)
	}
	if in.State == "" {
		return false, apperrors.New(apperrors.KindInvalidArgument, "SCM_NO_STATE", "state is required", nil)
	}

	state, err := uc.states.Decode(in.State)
	if err != nil {
		return false, apperrors.New(apperrors.KindPermissionDenied, "SCM_OAUTH_STATE_INVALID", "oauth state token is invalid, expired, or tampered with", err)
	}
	// The state token is the source of truth for which tenant/user/provider
	// this callback belongs to (§9.1: it's signed at StartOAuthFlow time,
	// before the browser ever leaves this service's control) — the request's
	// own fields must agree with it, never override it, so a caller can't
	// complete tenant A's flow against tenant B's callback by re-submitting
	// a different tenant_id alongside someone else's valid state token.
	if state.TenantID != in.TenantID || state.Provider != in.Provider {
		return false, apperrors.New(apperrors.KindPermissionDenied, "SCM_OAUTH_STATE_MISMATCH", "oauth state token does not match this request's tenant/provider", nil)
	}
	redirectURI := in.RedirectURI
	if redirectURI == "" {
		redirectURI = state.RedirectURI
	}

	exchanger, err := uc.exchangers.Resolve(in.Provider)
	if err != nil {
		return false, apperrors.New(apperrors.KindInvalidArgument, "SCM_PROVIDER_UNSUPPORTED", "no OAuth configuration registered for this provider", err)
	}

	token, err := exchanger.ExchangeCode(ctx, in.Code, redirectURI)
	if err != nil {
		return false, apperrors.New(apperrors.KindInternal, "SCM_OAUTH_EXCHANGE_FAILED", "failed to exchange authorization code for an access token", err)
	}

	if err := uc.writer.Write(ctx, in.TenantID, in.Provider, token); err != nil {
		return false, apperrors.New(apperrors.KindInternal, "SCM_CREDENTIAL_WRITE_FAILED", "failed to write oauth credential via credential-broker-service", err)
	}

	return true, nil
}
