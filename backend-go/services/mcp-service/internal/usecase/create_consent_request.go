package usecase

import (
	"context"
	"errors"

	"github.com/google/uuid"

	"github.com/stablyai/orca-go/services/mcp-service/internal/domain"
)

type CreateConsentRequestInput struct {
	AuthorizeParams
	State string
}

// CreateConsentRequestOutput has exactly one of RequestID (show the consent
// screen) or RedirectURL (auto-approved).
type CreateConsentRequestOutput struct {
	RequestID   string
	RedirectURL string
}

// CreateConsentRequest handles an authenticated user arriving at
// /oauth/authorize. It re-validates the request with auth-service itself (the
// gateway's earlier validation is a UX nicety, not a trust boundary), makes
// sure the client has a status in this tenant, and either auto-approves from
// an existing grant or records a consent request.
type CreateConsentRequest struct {
	repo     AuthorizationRepository
	settings TenantSettingsRepository
	as       AuthorizationServer
	clock    Clock
	defaults Defaults
	cfg      ConsentConfig
}

func NewCreateConsentRequest(repo AuthorizationRepository, settings TenantSettingsRepository, as AuthorizationServer, clock Clock, defaults Defaults, cfg ConsentConfig) *CreateConsentRequest {
	if cfg.ConsentTTL <= 0 {
		cfg.ConsentTTL = DefaultConsentTTL
	}
	return &CreateConsentRequest{repo: repo, settings: settings, as: as, clock: clock, defaults: defaults, cfg: cfg}
}

func (uc *CreateConsentRequest) Execute(ctx context.Context, in CreateConsentRequestInput) (CreateConsentRequestOutput, error) {
	who, err := userCaller(ctx)
	if err != nil {
		return CreateConsentRequestOutput{}, err
	}
	if len(in.State) > domain.MaxOAuthStateLength {
		return CreateConsentRequestOutput{}, domain.ErrInvalidArgument("state is too long")
	}
	st, err := uc.settings.GetOrCreateTenantSettings(ctx, uc.defaults.For(who.TenantID))
	if err != nil {
		return CreateConsentRequestOutput{}, domain.ErrInternal("failed to load tenant settings", err)
	}
	if !st.Enabled {
		return CreateConsentRequestOutput{}, domain.ErrDisabled()
	}

	info, err := uc.as.ValidateAuthorizeRequest(ctx, in.AuthorizeParams)
	if err != nil {
		return CreateConsentRequestOutput{}, err
	}
	view, err := uc.as.EnsureClientForTenant(ctx, info.ClientID, st.DCREnabled)
	if err != nil {
		return CreateConsentRequestOutput{}, err
	}
	if view.Status != "allowed" {
		return CreateConsentRequestOutput{}, domain.ErrClientNotAllowed()
	}

	// Never ask the user to approve more than their role could ever grant.
	scopes := domain.IntersectScopes(info.Scopes, domain.ScopeCeilingForRole(who.Role))
	if len(scopes) == 0 {
		return CreateConsentRequestOutput{}, domain.ErrScopeNotAllowed("none of the requested scopes is permitted for your role")
	}

	grant, err := uc.repo.GetActiveGrant(ctx, who.TenantID, who.UserID, info.ClientID)
	switch {
	case err == nil && domain.ScopesSubset(scopes, grant.Scopes):
		// Existing consent already covers everything asked for.
		return uc.autoApprove(ctx, info, in, scopes, grant)
	case err != nil && !errors.Is(err, ErrGrantNotFound):
		return CreateConsentRequestOutput{}, domain.ErrInternal("failed to load grant", err)
	}

	seen, err := uc.repo.CountGrantsForClient(ctx, who.TenantID, info.ClientID)
	if err != nil {
		return CreateConsentRequestOutput{}, domain.ErrInternal("failed to load client history", err)
	}
	now := uc.clock.Now()
	req := domain.ConsentRequest{
		ID: uuid.NewString(), TenantID: who.TenantID, UserID: who.UserID, ClientID: info.ClientID, ClientName: info.ClientName,
		ClientURI: info.ClientURI, RedirectURI: info.RedirectURI, Scopes: scopes, State: in.State,
		CodeChallenge: in.CodeChallenge, Resource: info.Resource, IsNewClient: seen == 0,
		RegisteredViaDCR: info.RegisteredVia == "dcr", CreatedAt: now, ExpiresAt: now.Add(uc.cfg.ConsentTTL),
	}
	if err := uc.repo.CreateConsentRequest(ctx, req); err != nil {
		return CreateConsentRequestOutput{}, domain.ErrInternal("failed to store consent request", err)
	}
	return CreateConsentRequestOutput{RequestID: req.ID}, nil
}

func (uc *CreateConsentRequest) autoApprove(ctx context.Context, info AuthorizeInfo, in CreateConsentRequestInput, scopes []string, g domain.Grant) (CreateConsentRequestOutput, error) {
	code, err := uc.as.IssueAuthCode(ctx, IssueCodeInput{
		ClientID: info.ClientID, RedirectURI: info.RedirectURI, CodeChallenge: in.CodeChallenge,
		Resource: info.Resource, GrantID: g.ID, Scopes: scopes,
	})
	if err != nil {
		return CreateConsentRequestOutput{}, err
	}
	u, err := domain.BuildAuthorizeRedirect(info.RedirectURI, map[string]string{"code": code}, in.State, uc.cfg.Issuer)
	if err != nil {
		return CreateConsentRequestOutput{}, domain.ErrInternal("failed to build redirect", err)
	}
	return CreateConsentRequestOutput{RedirectURL: u}, nil
}
