package usecase

import (
	"context"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/task-service/internal/domain"
)

type RevokeGrantInput struct {
	TaskID    string
	SubjectID string
	Level     domain.GrantLevel
}

// RevokeGrant requires 'manage' on TaskID before deleting a grant, same
// pre-check TASK-TG-03-01 added to Grant — RevokeGrant is new code, built
// with the check from the start. Deletes by the (task_id, subject_id,
// level) composite key (no surrogate grant_id on the wire, see
// GrantRepository.Revoke's doc comment) — idempotent: revoking a grant
// that doesn't (or no longer) exist is not an error.
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
	// The fix: require 'manage' on TaskID before revoking ANY grant on it —
	// same "every mutating RPC calls ResolvePermission internally first"
	// rule task-service.md §3 already states for every other mutation.
	if _, err := uc.resolvePermission.Execute(ctx, ResolvePermissionInput{TaskID: in.TaskID, UserID: callerID, Action: "manage"}); err != nil {
		return err
	}
	if err := uc.grants.Revoke(ctx, tenantID, in.TaskID, in.SubjectID, in.Level); err != nil {
		return apperrors.New(apperrors.KindInternal, "TASK_REVOKE_GRANT_FAILED", "failed to revoke grant", err)
	}
	uc.events.Publish(ctx, tenantID, "task.grant_revoked", map[string]any{
		"task_id": in.TaskID, "subject_id": in.SubjectID, "level": in.Level, "revoked_by": callerID,
	})
	return nil
}
