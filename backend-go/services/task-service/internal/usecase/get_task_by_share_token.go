package usecase

import (
	"context"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/services/task-service/internal/domain"
)

// GetTaskByShareToken looks up a task by its public share-link token with
// NO tenant/permission check (TASK-TG-003-05 — SECURITY REVIEW REQUIRED
// before merge). Deliberately does NOT call tenant.RequireTenantID: this is
// task-service's first unauthenticated read path, by design — the token
// itself IS the authorization, per BE-SOL-003's own framing ("bypasses
// ResolveGrant/OPA entirely"). Returns the dedicated, narrow
// domain.TaskShareView projection — see that type's own doc comment for why
// this is NOT domain.Task with fields blanked out.
//
// An unknown/malformed token returns the identical NotFound/TASK_NOT_FOUND
// error either way — never a distinguishable "invalid format" vs. "not
// found" error, so this path has no obvious token-enumeration side channel
// via error message alone (timing-based enumeration is a real,
// separate concern this usecase does not attempt to close — flagged for
// the security review, not solved here).
type GetTaskByShareToken struct {
	tasks TaskRepository
}

func NewGetTaskByShareToken(tasks TaskRepository) *GetTaskByShareToken {
	return &GetTaskByShareToken{tasks: tasks}
}

func (uc *GetTaskByShareToken) Execute(ctx context.Context, token string) (domain.TaskShareView, error) {
	if token == "" {
		return domain.TaskShareView{}, apperrors.New(apperrors.KindInvalidArgument, "TASK_SHARE_TOKEN_REQUIRED", "share token is required", nil)
	}
	task, err := uc.tasks.GetByShareToken(ctx, token)
	if err != nil {
		return domain.TaskShareView{}, apperrors.New(apperrors.KindNotFound, "TASK_NOT_FOUND", "no task found for this share token", err)
	}
	// Field allowlist is the entire point of this usecase — ONLY these 4
	// fields, never ai_context/ai_plan_json/comments/grants/owner_id/
	// assignee_id/reporter_id/or any other domain.Task field. See
	// TestGetTaskByShareToken_OnlyExposesAllowlistedFields, the regression
	// test that fails loudly if a future Task field addition isn't
	// explicitly excluded here.
	return domain.TaskShareView{
		ID:          task.ID,
		Title:       task.Title,
		Status:      string(task.Status),
		Description: task.Description,
	}, nil
}
