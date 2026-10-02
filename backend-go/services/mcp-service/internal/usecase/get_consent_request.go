package usecase

import (
	"context"
	"errors"

	"github.com/google/uuid"

	"github.com/stablyai/orca-go/services/mcp-service/internal/domain"
)

type ConsentView struct {
	Request        domain.ConsentRequest
	Scopes         []domain.ScopeDescriptor // what the client asked for, described for the user
	AlreadyGranted []string                 // scopes the user already gave this client
	RedirectHost   string
}

// GetConsentRequest shows a consent request to the user it belongs to.
type GetConsentRequest struct {
	repo  AuthorizationRepository
	clock Clock
}

func NewGetConsentRequest(repo AuthorizationRepository, clock Clock) *GetConsentRequest {
	return &GetConsentRequest{repo: repo, clock: clock}
}

func (uc *GetConsentRequest) Execute(ctx context.Context, requestID string) (ConsentView, error) {
	who, err := userCaller(ctx)
	if err != nil {
		return ConsentView{}, err
	}
	req, err := loadPendingConsent(ctx, uc.repo, uc.clock, who, requestID)
	if err != nil {
		return ConsentView{}, err
	}
	view := ConsentView{Request: req, RedirectHost: domain.RedirectHost(req.RedirectURI)}
	for _, id := range req.Scopes {
		for _, d := range domain.ScopeCatalog() {
			if d.ID == id {
				view.Scopes = append(view.Scopes, d)
			}
		}
	}
	if g, err := uc.repo.GetActiveGrant(ctx, who.TenantID, who.UserID, req.ClientID); err == nil {
		view.AlreadyGranted = g.Scopes
	} else if !errors.Is(err, ErrGrantNotFound) {
		return ConsentView{}, domain.ErrInternal("failed to load grant", err)
	}
	return view, nil
}

// loadPendingConsent returns the caller's own, undecided, unexpired request.
// Missing, foreign, cross-tenant and already-decided all look the same
// (MCP_CONSENT_NOT_FOUND); only the owner can learn that one expired.
func loadPendingConsent(ctx context.Context, repo AuthorizationRepository, clock Clock, who callerIdentity, requestID string) (domain.ConsentRequest, error) {
	if _, err := uuid.Parse(requestID); err != nil {
		return domain.ConsentRequest{}, domain.ErrConsentNotFound()
	}
	req, err := repo.GetConsentRequest(ctx, who.TenantID, who.UserID, requestID)
	if errors.Is(err, ErrConsentRequestNotFound) {
		return domain.ConsentRequest{}, domain.ErrConsentNotFound()
	}
	if err != nil {
		return domain.ConsentRequest{}, domain.ErrInternal("failed to load consent request", err)
	}
	if req.DecidedAt != nil {
		return domain.ConsentRequest{}, domain.ErrConsentNotFound()
	}
	if !clock.Now().Before(req.ExpiresAt) {
		return domain.ConsentRequest{}, domain.ErrConsentExpired()
	}
	return req, nil
}
