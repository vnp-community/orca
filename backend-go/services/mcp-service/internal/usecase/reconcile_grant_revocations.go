package usecase

import (
	"context"
	"log/slog"

	"github.com/stablyai/orca-go/common/tenant"
)

// ReconcileGrantRevocations retries telling auth-service about grants that
// were revoked here but whose propagation failed. It is what turns "revoked
// in the UI" into "tokens stop working" even across auth-service outages.
type ReconcileGrantRevocations struct {
	repo  AuthorizationRepository
	as    AuthorizationServer
	clock Clock
}

func NewReconcileGrantRevocations(repo AuthorizationRepository, as AuthorizationServer, clock Clock) *ReconcileGrantRevocations {
	return &ReconcileGrantRevocations{repo: repo, as: as, clock: clock}
}

// Execute processes up to limit pending revocations and returns how many
// were propagated. A failure on one grant does not stop the others.
func (uc *ReconcileGrantRevocations) Execute(ctx context.Context, limit int) (int, error) {
	pending, err := uc.repo.ListUnpropagatedRevocations(ctx, limit)
	if err != nil {
		return 0, err
	}
	done := 0
	for _, g := range pending {
		gctx := tenant.WithTenantID(ctx, g.TenantID)
		if g.RevokedBy != "" {
			gctx = tenant.WithUserID(gctx, g.RevokedBy)
		}
		if err := uc.as.RevokeGrant(gctx, g.ID, revokeReason(g.UserID, g.RevokedBy)); err != nil {
			slog.WarnContext(ctx, "grant revocation propagation retry failed", slog.String("grant_id", g.ID), slog.Any("error", err))
			continue
		}
		if err := uc.repo.MarkRevocationPropagated(ctx, g.TenantID, g.ID, uc.clock.Now()); err != nil {
			slog.WarnContext(ctx, "failed to mark grant revocation propagated", slog.String("grant_id", g.ID), slog.Any("error", err))
			continue
		}
		done++
	}
	return done, nil
}
