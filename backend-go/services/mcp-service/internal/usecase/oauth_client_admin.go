package usecase

import (
	"context"
	"errors"
	"log/slog"

	"github.com/google/uuid"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/services/mcp-service/internal/domain"
)

type OAuthClientItem struct {
	OAuthClientView
	ActiveGrants int
}

// ListOAuthClients is the admin client list: auth-service's per-tenant client
// status joined with this service's grant counts, so the gateway never has to
// stitch data from two services.
type ListOAuthClients struct {
	repo AuthorizationRepository
	as   AuthorizationServer
}

func NewListOAuthClients(repo AuthorizationRepository, as AuthorizationServer) *ListOAuthClients {
	return &ListOAuthClients{repo: repo, as: as}
}

func (uc *ListOAuthClients) Execute(ctx context.Context) ([]OAuthClientItem, error) {
	who, err := userCaller(ctx)
	if err != nil {
		return nil, err
	}
	if err := who.requireAdmin(); err != nil {
		return nil, err
	}
	clients, err := uc.as.ListClientsForTenant(ctx)
	if err != nil {
		return nil, err
	}
	counts, err := uc.repo.CountActiveGrantsByClient(ctx, who.TenantID)
	if err != nil {
		return nil, domain.ErrInternal("failed to count grants", err)
	}
	out := make([]OAuthClientItem, 0, len(clients))
	for _, c := range clients {
		out = append(out, OAuthClientItem{OAuthClientView: c, ActiveGrants: counts[c.ClientID]})
	}
	return out, nil
}

// SetOAuthClientStatus lets a tenant admin allow or block a client. Blocking
// is enforced in auth-service (it revokes the client's live tokens); here we
// only publish the change.
type SetOAuthClientStatus struct {
	repo   AuthorizationRepository
	as     AuthorizationServer
	outbox OutboxWriter
	clock  Clock
}

func NewSetOAuthClientStatus(repo AuthorizationRepository, as AuthorizationServer, outbox OutboxWriter, clock Clock) *SetOAuthClientStatus {
	return &SetOAuthClientStatus{repo: repo, as: as, outbox: outbox, clock: clock}
}

func (uc *SetOAuthClientStatus) Execute(ctx context.Context, clientID, status string) (OAuthClientItem, error) {
	who, err := userCaller(ctx)
	if err != nil {
		return OAuthClientItem{}, err
	}
	if err := who.requireAdmin(); err != nil {
		return OAuthClientItem{}, err
	}
	if status != "allowed" && status != "blocked" {
		return OAuthClientItem{}, domain.ErrInvalidArgument("status must be allowed or blocked")
	}
	view, err := uc.as.SetClientStatus(ctx, clientID, status)
	if err != nil {
		return OAuthClientItem{}, mapUnknownClient(err)
	}
	if ev, err := domain.NewOutboxEvent(uuid.NewString(), domain.SubjectClientStatusChange, who.TenantID, uc.clock.Now(),
		map[string]any{"client_id": view.ClientID, "status": view.Status, "by": who.UserID}); err == nil {
		if err := uc.outbox.EnqueueOutbox(ctx, who.TenantID, ev); err != nil {
			slog.WarnContext(ctx, "failed to enqueue client status event", slog.String("client_id", view.ClientID), slog.Any("error", err))
		}
	}
	counts, err := uc.repo.CountActiveGrantsByClient(ctx, who.TenantID)
	if err != nil {
		return OAuthClientItem{}, domain.ErrInternal("failed to count grants", err)
	}
	return OAuthClientItem{OAuthClientView: view, ActiveGrants: counts[view.ClientID]}, nil
}

// mapUnknownClient hides auth-service's client-lookup codes behind MCP_NOT_FOUND.
func mapUnknownClient(err error) error {
	var ae *apperrors.AppError
	if errors.As(err, &ae) && (ae.Code == "OAUTH_CLIENT_NOT_FOUND" || ae.Code == "OAUTH_INVALID_CLIENT") {
		return domain.ErrNotFound()
	}
	return err
}
