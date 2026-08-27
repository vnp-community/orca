package usecase

import (
	"context"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/task-service/internal/domain"
)

type GrantInput struct {
	TaskID    string
	SubjectID string
	Level     domain.GrantLevel
	ApplyTree bool
}

// Grant is task-service's grant-mutation usecase. Requires the CALLER to
// already have 'manage' access to TaskID before writing a new grant on it
// — closes a live authorization gap: previously any authenticated caller
// could call Grant on any task_id they could name, including granting
// themselves owner, with zero access check (found while grounding
// SOL-TG-03's design against the current code). Per task-service.md §9,
// Grant/RevokeGrant should emit structured audit events — not wired in this
// scaffold (no EventPublisher port defined here); see this service's
// README.
type Grant struct {
	grants            GrantRepository
	resolvePermission *ResolvePermission
}

func NewGrant(grants GrantRepository, resolvePermission *ResolvePermission) *Grant {
	return &Grant{grants: grants, resolvePermission: resolvePermission}
}

func (uc *Grant) Execute(ctx context.Context, in GrantInput) error {
	tenantID, err := tenant.RequireTenantID(ctx)
	if err != nil {
		return apperrors.New(apperrors.KindUnauthenticated, "TASK_NO_TENANT", "no tenant in request context", err)
	}
	if in.TaskID == "" {
		return apperrors.New(apperrors.KindInvalidArgument, "TASK_GRANT_INVALID", "task_id is required", nil)
	}
	if in.SubjectID == "" {
		return apperrors.New(apperrors.KindInvalidArgument, "TASK_GRANT_INVALID", "subject_id is required", nil)
	}
	if !in.Level.Valid() {
		return apperrors.New(apperrors.KindInvalidArgument, "TASK_GRANT_INVALID", "level is not a recognized grant level", nil)
	}

	callerID, _ := tenant.UserID(ctx)
	// The fix: require 'manage' on TaskID before writing ANY grant on it —
	// same "every mutating RPC calls ResolvePermission internally first"
	// rule task-service.md §3 already states for every other mutation.
	if _, err := uc.resolvePermission.Execute(ctx, ResolvePermissionInput{TaskID: in.TaskID, UserID: callerID, Action: "manage"}); err != nil {
		return err // PermissionDenied/TASK_NO_GRANT, unchanged shape
	}

	grant := domain.Grant{TaskID: in.TaskID, SubjectID: in.SubjectID, Level: in.Level, ApplyTree: in.ApplyTree}
	if err := uc.grants.Grant(ctx, tenantID, grant); err != nil {
		return apperrors.New(apperrors.KindInternal, "TASK_GRANT_FAILED", "failed to persist grant", err)
	}
	return nil
}
