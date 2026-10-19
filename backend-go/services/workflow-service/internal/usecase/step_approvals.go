package usecase

import (
	"context"
	"errors"
	"time"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/workflow-service/internal/domain"
)

// ProjectRoleReader answers a user's role in a project ("" = not a member).
type ProjectRoleReader interface {
	MemberRole(ctx context.Context, projectID, userID string) (string, error)
}

// StepApprovals serves the person's side of domain.StepApproval: see what is
// waiting, read a decision, approve or reject it. The waiting step itself
// (tdd_agent.approval) delivers the decision to the external system.
type StepApprovals struct {
	repo         StepApprovalRepository
	roles        ProjectRoleReader
	allowedRoles map[string]bool
	now          func() time.Time
}

// NewStepApprovals: approverRoles are the project roles allowed to decide (default: owner only).
func NewStepApprovals(repo StepApprovalRepository, roles ProjectRoleReader, approverRoles []string) *StepApprovals {
	allowed := map[string]bool{}
	for _, r := range approverRoles {
		allowed[r] = true
	}
	if len(allowed) == 0 {
		allowed["owner"] = true
	}
	return &StepApprovals{repo: repo, roles: roles, allowedRoles: allowed, now: func() time.Time { return time.Now().UTC() }}
}

func requireCaller(ctx context.Context) (string, string, error) {
	tenantID, err := tenant.RequireTenantID(ctx)
	if err != nil {
		return "", "", apperrors.New(apperrors.KindUnauthenticated, "WORKFLOW_NO_TENANT", "no tenant in request context", err)
	}
	userID, _ := tenant.UserID(ctx)
	if userID == "" {
		return "", "", apperrors.New(apperrors.KindUnauthenticated, "WORKFLOW_NO_USER", "no user in request context", nil)
	}
	return tenantID, userID, nil
}

func mapStepApprovalErr(err error) error {
	if errors.Is(err, domain.ErrStepApprovalNotFound) {
		return apperrors.New(apperrors.KindNotFound, "STEP_APPROVAL_NOT_FOUND", "step approval not found", err)
	}
	return apperrors.New(apperrors.KindInternal, "STEP_APPROVAL_STORE_FAILED", "step approval store failed", err)
}

// canSee: any member of the approval's project; canDecide: an allowed project role.
func (uc *StepApprovals) role(ctx context.Context, a domain.StepApproval, userID string) (string, error) {
	if a.ProjectID == "" {
		return "", nil
	}
	role, err := uc.roles.MemberRole(ctx, a.ProjectID, userID)
	if err != nil {
		return "", apperrors.New(apperrors.KindUnavailable, "STEP_APPROVAL_ROLE_UNAVAILABLE", "could not read the project membership", err)
	}
	return role, nil
}

// List returns pending approvals of a project (or an execution) the caller can see.
func (uc *StepApprovals) List(ctx context.Context, projectID, executionID string) ([]domain.StepApproval, error) {
	tenantID, userID, err := requireCaller(ctx)
	if err != nil {
		return nil, err
	}
	rows, err := uc.repo.ListPendingStepApprovals(ctx, tenantID, projectID, executionID, 200)
	if err != nil {
		return nil, mapStepApprovalErr(err)
	}
	visible := make([]domain.StepApproval, 0, len(rows))
	roleOf := map[string]string{}
	for _, a := range rows {
		role, seen := roleOf[a.ProjectID]
		if !seen {
			if role, err = uc.role(ctx, a, userID); err != nil {
				return nil, err
			}
			roleOf[a.ProjectID] = role
		}
		if role != "" {
			visible = append(visible, a)
		}
	}
	return visible, nil
}

// Get returns one approval with its decision, and whether the caller may decide it.
func (uc *StepApprovals) Get(ctx context.Context, id string) (domain.StepApproval, bool, error) {
	tenantID, userID, err := requireCaller(ctx)
	if err != nil {
		return domain.StepApproval{}, false, err
	}
	a, err := uc.repo.GetStepApproval(ctx, tenantID, id)
	if err != nil {
		return domain.StepApproval{}, false, mapStepApprovalErr(err)
	}
	role, err := uc.role(ctx, a, userID)
	if err != nil {
		return domain.StepApproval{}, false, err
	}
	if role == "" {
		return domain.StepApproval{}, false, apperrors.New(apperrors.KindNotFound, "STEP_APPROVAL_NOT_FOUND", "step approval not found", nil)
	}
	return a, uc.allowedRoles[role], nil
}

// Resolve approves or rejects; decisionHash must be the hash of the decision the caller was shown.
func (uc *StepApprovals) Resolve(ctx context.Context, id string, approve bool, decisionHash, rationale string) (domain.StepApproval, error) {
	a, canDecide, err := uc.Get(ctx, id)
	if err != nil {
		return domain.StepApproval{}, err
	}
	if !canDecide {
		return domain.StepApproval{}, apperrors.New(apperrors.KindPermissionDenied, "STEP_APPROVAL_FORBIDDEN", "your project role cannot decide this approval", nil)
	}
	_, userID, _ := requireCaller(ctx)
	if err := a.Resolve(approve, userID, decisionHash, rationale, uc.now()); err != nil {
		switch {
		case errors.Is(err, domain.ErrStepApprovalStale):
			return domain.StepApproval{}, apperrors.New(apperrors.KindFailedPrecondition, "STEP_APPROVAL_STALE", err.Error(), err)
		case errors.Is(err, domain.ErrStepApprovalNotPending):
			return domain.StepApproval{}, apperrors.New(apperrors.KindFailedPrecondition, "STEP_APPROVAL_NOT_PENDING", err.Error(), err)
		default:
			return domain.StepApproval{}, apperrors.New(apperrors.KindInvalidArgument, "STEP_APPROVAL_INVALID", err.Error(), err)
		}
	}
	ok, err := uc.repo.ResolveStepApproval(ctx, a)
	if err != nil {
		return domain.StepApproval{}, mapStepApprovalErr(err)
	}
	if !ok {
		return domain.StepApproval{}, apperrors.New(apperrors.KindFailedPrecondition, "STEP_APPROVAL_NOT_PENDING", "someone resolved this approval first", nil)
	}
	return a, nil
}
