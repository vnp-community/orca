package usecase

import (
	"context"
	"errors"
	"log/slog"

	"github.com/google/uuid"

	"github.com/stablyai/orca-go/services/mcp-service/internal/domain"
)

type RevokeGrantInput struct {
	GrantID string
	Admin   bool // revoke someone else's grant; requires role admin
}

// RevokeGrant revokes a grant. The durable state change and its outbox event
// commit first; propagation to auth-service (which kills the tokens) is
// attempted right away and retried by ReconcileGrantRevocations if it fails,
// so the user always gets OK once the revocation is recorded.
type RevokeGrant struct {
	repo  AuthorizationRepository
	as    AuthorizationServer
	clock Clock
}

func NewRevokeGrant(repo AuthorizationRepository, as AuthorizationServer, clock Clock) *RevokeGrant {
	return &RevokeGrant{repo: repo, as: as, clock: clock}
}

func (uc *RevokeGrant) Execute(ctx context.Context, in RevokeGrantInput) error {
	who, err := userCaller(ctx)
	if err != nil {
		return err
	}
	owner := who.UserID
	if in.Admin {
		if err := who.requireAdmin(); err != nil {
			return err
		}
		owner = ""
	}
	if _, err := uuid.Parse(in.GrantID); err != nil {
		return domain.ErrNotFound()
	}
	now := uc.clock.Now()
	g, _, err := uc.repo.RevokeGrant(ctx, who.TenantID, in.GrantID, owner, who.UserID, now, uuid.NewString())
	if errors.Is(err, ErrGrantNotFound) {
		return domain.ErrNotFound()
	}
	if err != nil {
		return domain.ErrInternal("failed to revoke grant", err)
	}
	if g.RevocationPropagatedAt != nil {
		return nil
	}
	if err := uc.as.RevokeGrant(ctx, g.ID, revokeReason(g.UserID, who.UserID)); err != nil {
		slog.WarnContext(ctx, "grant revocation not yet propagated to auth-service; reconcile job will retry",
			slog.String("grant_id", g.ID), slog.Any("error", err))
		return nil
	}
	if err := uc.repo.MarkRevocationPropagated(ctx, who.TenantID, g.ID, uc.clock.Now()); err != nil {
		slog.WarnContext(ctx, "failed to mark grant revocation propagated", slog.String("grant_id", g.ID), slog.Any("error", err))
	}
	return nil
}

func revokeReason(grantOwner, actor string) string {
	if actor != "" && actor != grantOwner {
		return domain.RevokeReasonAdmin
	}
	return domain.RevokeReasonUser
}
