package usecase

import (
	"context"
	"errors"

	"github.com/google/uuid"

	"github.com/stablyai/orca-go/services/mcp-service/internal/domain"
)

type DecideConsentInput struct {
	RequestID string
	Decision  string   // approve | deny
	Scopes    []string // approve: the scopes the user chose
}

type DecideConsentOutput struct{ RedirectURL string }

// DecideConsent records the user's decision. The decision is a single
// conditional update (pending, unexpired, owned by the caller), so two
// concurrent decisions on one request can never both succeed. Approving
// stores exactly the scopes the user chose as their grant, then asks
// auth-service for the code; denying creates nothing.
type DecideConsent struct {
	repo  AuthorizationRepository
	as    AuthorizationServer
	clock Clock
	cfg   ConsentConfig
}

func NewDecideConsent(repo AuthorizationRepository, as AuthorizationServer, clock Clock, cfg ConsentConfig) *DecideConsent {
	return &DecideConsent{repo: repo, as: as, clock: clock, cfg: cfg}
}

func (uc *DecideConsent) Execute(ctx context.Context, in DecideConsentInput) (DecideConsentOutput, error) {
	who, err := userCaller(ctx)
	if err != nil {
		return DecideConsentOutput{}, err
	}
	if in.Decision != domain.DecisionApprove && in.Decision != domain.DecisionDeny {
		return DecideConsentOutput{}, domain.ErrInvalidArgument("decision must be approve or deny")
	}
	req, err := loadPendingConsent(ctx, uc.repo, uc.clock, who, in.RequestID)
	if err != nil {
		return DecideConsentOutput{}, err
	}
	if in.Decision == domain.DecisionDeny {
		return uc.deny(ctx, who, req)
	}
	return uc.approve(ctx, who, req, in.Scopes)
}

func (uc *DecideConsent) deny(ctx context.Context, who callerIdentity, req domain.ConsentRequest) (DecideConsentOutput, error) {
	if _, err := uc.repo.DenyConsent(ctx, who.TenantID, who.UserID, req.ID, uc.clock.Now()); err != nil {
		return DecideConsentOutput{}, uc.mapNotDecidable(ctx, who, req.ID, err)
	}
	u, err := domain.BuildAuthorizeRedirect(req.RedirectURI, map[string]string{"error": "access_denied"}, req.State, uc.cfg.Issuer)
	if err != nil {
		return DecideConsentOutput{}, domain.ErrInternal("failed to build redirect", err)
	}
	return DecideConsentOutput{RedirectURL: u}, nil
}

func (uc *DecideConsent) approve(ctx context.Context, who callerIdentity, req domain.ConsentRequest, chosen []string) (DecideConsentOutput, error) {
	scopes, err := domain.NormalizeScopes(chosen)
	if err != nil || len(scopes) == 0 {
		return DecideConsentOutput{}, domain.ErrScopeInvalid("choose at least one of the requested scopes")
	}
	if !domain.ScopesSubset(scopes, req.Scopes) {
		return DecideConsentOutput{}, domain.ErrScopeInvalid("scopes must be a subset of what the app requested")
	}
	if !domain.ScopesSubset(scopes, domain.ScopeCeilingForRole(who.Role)) {
		return DecideConsentOutput{}, domain.ErrScopeNotAllowed("a selected scope exceeds what your role allows")
	}

	res, err := uc.repo.ApproveConsent(ctx, ApproveConsentInput{
		TenantID: who.TenantID, UserID: who.UserID, RequestID: req.ID, Scopes: scopes, Now: uc.clock.Now(),
		GrantID: uuid.NewString(), EventID: uuid.NewString(),
	})
	if err != nil {
		return DecideConsentOutput{}, uc.mapNotDecidable(ctx, who, req.ID, err)
	}
	code, err := uc.as.IssueAuthCode(ctx, IssueCodeInput{
		ClientID: req.ClientID, RedirectURI: req.RedirectURI, CodeChallenge: req.CodeChallenge,
		Resource: req.Resource, GrantID: res.Grant.ID, Scopes: scopes,
	})
	if err != nil {
		return DecideConsentOutput{}, err
	}
	u, err := domain.BuildAuthorizeRedirect(req.RedirectURI, map[string]string{"code": code}, req.State, uc.cfg.Issuer)
	if err != nil {
		return DecideConsentOutput{}, domain.ErrInternal("failed to build redirect", err)
	}
	return DecideConsentOutput{RedirectURL: u}, nil
}

// mapNotDecidable translates a lost race. Only a still-pending request that
// has timed out reports EXPIRED; every other cause is the opaque NOT_FOUND.
func (uc *DecideConsent) mapNotDecidable(ctx context.Context, who callerIdentity, requestID string, err error) error {
	if !errors.Is(err, ErrConsentNotDecidable) {
		return domain.ErrInternal("failed to record decision", err)
	}
	_, lookupErr := loadPendingConsent(ctx, uc.repo, uc.clock, who, requestID)
	if lookupErr != nil {
		return lookupErr
	}
	return domain.ErrConsentNotFound()
}
