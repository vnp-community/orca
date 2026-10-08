package usecase

import (
	"context"
	"log/slog"

	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/request-service/internal/domain"
)

// DefaultNotifyMaxRecipients caps one approval's recipients (REQUEST_APPROVAL_NOTIFY_MAX_RECIPIENTS).
const DefaultNotifyMaxRecipients = 50

// ExpandApprovalRecipients turns the approver snapshot into user ids: users stay, the reporter is the Request's
// reporter, teams and role:admin go through the directories. Lookup errors are returned so the outbox relay retries;
// they never touch the Approval.
type ExpandApprovalRecipients struct {
	ApproverRepo  ApprovalApproverRepository
	TeamResolver  TeamMembershipResolver
	AdminResolver AdminDirectoryResolver
	MaxRecipients int
	Log           *slog.Logger
}

func (uc *ExpandApprovalRecipients) Execute(ctx context.Context, approvalID string, selfApprovalAllowed bool, reporterID, requestedBy string) ([]string, error) {
	tenantID, err := tenant.RequireTenantID(ctx)
	if err != nil {
		return nil, err
	}
	log := uc.Log
	if log == nil {
		log = slog.Default()
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
			if uc.TeamResolver == nil {
				return nil, domain.ErrApprovalDirectoryUnavailable
			}
			members, err := uc.TeamResolver.MembersOfTeam(ctx, p.ID)
			if err != nil {
				return nil, err
			}
			users = append(users, members...)
		case domain.PrincipalKindRole:
			if p.ID != "admin" {
				log.Warn("no directory for role principal; skipping recipients", slog.String("role", p.ID))
				continue
			}
			if uc.AdminResolver == nil {
				return nil, domain.ErrApprovalDirectoryUnavailable
			}
			admins, err := uc.AdminResolver.ListAdmins(ctx, tenantID)
			if err != nil {
				return nil, err
			}
			users = append(users, admins...)
		}
	}

	seen := make(map[string]bool, len(users))
	var eligible []string
	for _, u := range users {
		if u == "" || seen[u] {
			continue
		}
		seen[u] = true
		if !selfApprovalAllowed && (u == reporterID || (u == requestedBy && requestedBy != systemActor)) {
			continue
		}
		eligible = append(eligible, u)
	}

	max := uc.MaxRecipients
	if max <= 0 {
		max = DefaultNotifyMaxRecipients
	}
	if len(eligible) > max {
		log.Warn("approval recipients truncated", slog.String("approval_id", approvalID), slog.Int("dropped", len(eligible)-max))
		eligible = eligible[:max]
	}
	return eligible, nil
}
