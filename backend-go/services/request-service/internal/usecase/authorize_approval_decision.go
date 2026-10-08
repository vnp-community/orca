package usecase

import (
	"context"

	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/request-service/internal/domain"
)

// AuthorizeApprovalDecision loads the caller's identity (user, role, teams) and checks it against the
// approver snapshot taken when the approval opened, so later policy edits never change who may decide.
type AuthorizeApprovalDecision struct {
	ApproverRepo ApprovalApproverRepository
	Auth         domain.ApprovalAuthorization
	Teams        TeamMembershipResolver
}

var _ ApprovalAuthorizer = (*AuthorizeApprovalDecision)(nil)

type prefetchedTeamsKey struct{}

// Prefetch looks the caller's teams up before the decision transaction starts, so CanDecide makes no
// outbound call inside it. Callers that skip it still work: CanDecide then asks the directory itself.
func (a *AuthorizeApprovalDecision) Prefetch(ctx context.Context, ap domain.Approval) (context.Context, error) {
	tenantID, err := tenant.RequireTenantID(ctx)
	if err != nil {
		return ctx, err
	}
	userID, _ := tenant.UserID(ctx)
	role, _ := tenant.Role(ctx)
	if userID == "" || role == "admin" || tenant.ActorType(ctx) == tenant.ActorAgent {
		return ctx, nil
	}
	approvers, err := a.ApproverRepo.ListForApproval(ctx, tenantID, ap.ID)
	if err != nil || !needsTeams(approvers, userID) {
		return ctx, err
	}
	if a.Teams == nil {
		return ctx, domain.ErrApprovalDirectoryUnavailable
	}
	teams, err := a.Teams.TeamsForUser(ctx, userID)
	if err != nil {
		return ctx, err
	}
	return context.WithValue(ctx, prefetchedTeamsKey{}, teams), nil
}

func (a *AuthorizeApprovalDecision) CanDecide(ctx context.Context, req domain.Request, ap domain.Approval) error {
	tenantID, err := tenant.RequireTenantID(ctx)
	if err != nil {
		return err
	}
	userID, _ := tenant.UserID(ctx)
	role, _ := tenant.Role(ctx)
	actor := domain.DecisionActor{UserID: userID, Role: role, IsMachine: tenant.ActorType(ctx) == tenant.ActorAgent}

	approvers, err := a.ApproverRepo.ListForApproval(ctx, tenantID, ap.ID)
	if err != nil {
		return err
	}
	// Teams are only fetched when a team principal could grant access, so a directory outage does not block
	// admins or directly named approvers.
	if teams, ok := ctx.Value(prefetchedTeamsKey{}).([]string); ok {
		actor.TeamIDs = teams
	} else if actor.UserID != "" && !actor.IsMachine && role != "admin" && needsTeams(approvers, actor.UserID) {
		if a.Teams == nil {
			return domain.ErrApprovalDirectoryUnavailable
		}
		if actor.TeamIDs, err = a.Teams.TeamsForUser(ctx, actor.UserID); err != nil {
			return err
		}
	}
	return a.Auth.Decide(actor, ap, approvers, req.ReporterID)
}

func needsTeams(approvers []domain.Principal, userID string) bool {
	hasTeam := false
	for _, p := range approvers {
		if p.Kind == domain.PrincipalKindUser && p.ID == userID {
			return false
		}
		if p.Kind == domain.PrincipalKindTeam {
			hasTeam = true
		}
	}
	return hasTeam
}
