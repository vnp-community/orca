package usecase

import (
	"context"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/common/tenant"
)

// RemoveTeamMemberInput mirrors RemoveTeamMemberRequest 1:1.
type RemoveTeamMemberInput struct {
	TeamID string
	UserID string
}

// RemoveTeamMember deletes one team-membership row — the documented gap
// from services/tenant-service/README.md:101.
type RemoveTeamMember struct {
	teams        TeamRepository
	cache        ProfileCache
	invalidation CacheInvalidationPublisher
}

// NewRemoveTeamMember wires cache invalidation. invalidation may be nil
// (NATS unreachable at startup), same convention as NewAddTeamMember.
func NewRemoveTeamMember(teams TeamRepository, cache ProfileCache, invalidation CacheInvalidationPublisher) *RemoveTeamMember {
	return &RemoveTeamMember{teams: teams, cache: cache, invalidation: invalidation}
}

func (uc *RemoveTeamMember) Execute(ctx context.Context, in RemoveTeamMemberInput) error {
	companyID, err := tenant.RequireTenantID(ctx)
	if err != nil {
		return apperrors.New(apperrors.KindUnauthenticated, "TENANT_NO_TENANT", "no tenant in request context", err)
	}
	if _, found, err := uc.teams.Get(ctx, companyID, in.TeamID); err != nil {
		return apperrors.New(apperrors.KindInternal, "TENANT_TEAM_LOOKUP_FAILED", "failed to look up team", err)
	} else if !found {
		return apperrors.New(apperrors.KindNotFound, "TENANT_TEAM_NOT_FOUND", "team does not exist", nil)
	}
	if _, err := uc.teams.RemoveMember(ctx, in.TeamID, in.UserID); err != nil {
		return apperrors.New(apperrors.KindInternal, "TENANT_REMOVE_TEAM_MEMBER_FAILED", "failed to remove team member", err)
	}
	// Same invalidation obligation as AddTeamMember.Execute (§8) — the
	// removed member's team-layer contribution to ResolveProfile changes.
	if uc.cache != nil {
		uc.cache.Invalidate(ctx, in.UserID)
	}
	if uc.invalidation != nil {
		_ = uc.invalidation.PublishProfileInvalidated(ctx, companyID, in.UserID)
	}
	return nil
}
