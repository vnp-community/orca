package usecase

import (
	"context"
	"log/slog"

	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/request-service/internal/domain"
)

// ListPendingApprovalsForUser is the approver inbox: pending approvals whose snapshot names the caller
// (directly, by team, by role, or as reporter), minus those blocked by separation of duties.
type ListPendingApprovalsForUser struct {
	Repo  ApprovalRepository
	Teams TeamMembershipResolver
	Log   *slog.Logger
}

func (uc *ListPendingApprovalsForUser) Execute(ctx context.Context, st domain.SubjectType, pageSize int, pageToken string) ([]PendingApproval, string, error) {
	tenantID, err := tenant.RequireTenantID(ctx)
	if err != nil {
		return nil, "", err
	}
	userID, _ := tenant.UserID(ctx)
	if userID == "" {
		return nil, "", domain.ErrNoUser
	}
	role, _ := tenant.Role(ctx)
	f := PendingForUserFilter{UserID: userID, Role: role, SubjectType: st, PageSize: clampApprovalPageSize(pageSize), PageToken: pageToken}
	if role != "admin" && uc.Teams != nil {
		teams, err := uc.Teams.TeamsForUser(ctx, userID)
		if err != nil {
			// Degrade to user/role/reporter matches rather than an empty inbox when tenant-service is down.
			log := uc.Log
			if log == nil {
				log = slog.Default()
			}
			log.Warn("team lookup failed; inbox omits team approvals", slog.Any("error", err))
		}
		f.TeamIDs = teams
	}
	return uc.Repo.ListPendingForUser(ctx, tenantID, f)
}
