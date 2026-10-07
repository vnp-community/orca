package usecase

import (
	"context"

	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/request-service/internal/domain"
)

type AdminDirectoryResolver interface {
	ListAdmins(ctx context.Context, tenantID string) ([]string, error)
}

type TeamMembershipResolver interface {
	MembersOfTeam(ctx context.Context, teamID string) ([]string, error)
}

type ExpandApprovalRecipients struct {
	ApproverRepo  ApprovalApproverRepository
	TeamResolver  TeamMembershipResolver
	AdminResolver AdminDirectoryResolver
}

func (uc *ExpandApprovalRecipients) Execute(ctx context.Context, approvalID string, selfApprovalAllowed bool, reporterID string) ([]string, error) {
	tenantID, err := tenant.RequireTenantID(ctx)
	if err != nil {
		return nil, err
	}

	approvers, err := uc.ApproverRepo.ListForApproval(ctx, tenantID, approvalID)
	if err != nil {
		return nil, err
	}

	var users []string
	for _, p := range approvers {
		switch p.Kind {
		case domain.PrincipalKindUser:
			users = append(users, p.ID)
		case domain.PrincipalKindReporter:
			users = append(users, reporterID)
		case domain.PrincipalKindTeam:
			members, err := uc.TeamResolver.MembersOfTeam(ctx, p.ID)
			if err != nil {
				return nil, err // Let relay retry
			}
			users = append(users, members...)
		case domain.PrincipalKindRole:
			if p.ID == "admin" {
				admins, err := uc.AdminResolver.ListAdmins(ctx, tenantID)
				if err != nil {
					return nil, err // Let relay retry
				}
				users = append(users, admins...)
			}
		}
	}

	// Deduplicate
	seen := make(map[string]bool)
	var unique []string
	for _, u := range users {
		if !seen[u] {
			seen[u] = true
			unique = append(unique, u)
		}
	}

	// Filter
	eligible := domain.EligibleApprovers(nil, reporterID, selfApprovalAllowed, unique)

	// Truncate at 50
	if len(eligible) > 50 {
		eligible = eligible[:50]
	}

	return eligible, nil
}
