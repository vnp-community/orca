package usecase

import (
	"context"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/services/auth-service/internal/domain"
)

// GetAdminStats is an admin-console, read-only operation with no
// TDD-specified RPC — GET /admin/api/stats has no documented backing RPC,
// so this is a scope addition (SOL-001) rather than skip a documented
// route entirely. Cheapest useful implementation: 3 counts.
type GetAdminStats struct {
	users    UserRepository
	sessions SessionRepository
	policies AccessPolicyRepository
	clock    Clock
	opa      OPAClient
}

func NewGetAdminStats(users UserRepository, sessions SessionRepository, policies AccessPolicyRepository, clock Clock, opa OPAClient) *GetAdminStats {
	return &GetAdminStats{users: users, sessions: sessions, policies: policies, clock: clock, opa: opa}
}

func (uc *GetAdminStats) Execute(ctx context.Context) (domain.AdminStats, error) {
	if _, err := requireAdminActor(ctx, uc.users, uc.opa); err != nil {
		return domain.AdminStats{}, err
	}

	totalUsers, err := uc.users.Count(ctx)
	if err != nil {
		return domain.AdminStats{}, apperrors.New(apperrors.KindInternal, "AUTH_ADMIN_STATS_FAILED", "failed to count users", err)
	}
	activeSessions, err := uc.sessions.CountActive(ctx, uc.clock.Now())
	if err != nil {
		return domain.AdminStats{}, apperrors.New(apperrors.KindInternal, "AUTH_ADMIN_STATS_FAILED", "failed to count active sessions", err)
	}
	totalPolicies, err := uc.policies.CountDistinctIDs(ctx)
	if err != nil {
		return domain.AdminStats{}, apperrors.New(apperrors.KindInternal, "AUTH_ADMIN_STATS_FAILED", "failed to count access policies", err)
	}

	return domain.AdminStats{
		TotalUsers:     totalUsers,
		ActiveSessions: activeSessions,
		TotalPolicies:  totalPolicies,
	}, nil
}
