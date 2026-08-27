package usecase

import (
	"context"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/common/tenant"
)

type RevokeGrantInput struct {
	TaskID  string
	GrantID string
}

// RevokeGrant requires 'manage' on TaskID before deleting a grant, same
// pre-check TASK-TG-03-01 added to Grant — RevokeGrant is new code, built
// with the check from the start.
type RevokeGrant struct {
	grants            GrantRepository
	resolvePermission *ResolvePermission
	events            EventPublisher
}

func NewRevokeGrant(grants GrantRepository, resolvePermission *ResolvePermission, events EventPublisher) *RevokeGrant {
	return &RevokeGrant{grants: grants, resolvePermission: resolvePermission, events: events}
}

func (uc *RevokeGrant) Execute(ctx context.Context, in RevokeGrantInput) error {
	tenantID, err := tenant.RequireTenantID(ctx)
	if err != nil {
		return apperrors.New(apperrors.KindUnauthenticated, "TASK_NO_TENANT", "no tenant in request context", err)
	}
	callerID, _ := tenant.UserID(ctx)
	if _, err := uc.resolvePermission.Execute(ctx, ResolvePermissionInput{TaskID: in.TaskID, UserID: callerID, Action: "manage"}); err != nil {
		return err
	}
	if err := uc.grants.Revoke(ctx, tenantID, in.GrantID); err != nil {
		return apperrors.New(apperrors.KindNotFound, "TASK_GRANT_NOT_FOUND", "grant not found", err)
	}
	uc.events.Publish(ctx, tenantID, "task.grant_revoked", map[string]any{"task_id": in.TaskID, "grant_id": in.GrantID, "revoked_by": callerID})
	return nil
}
