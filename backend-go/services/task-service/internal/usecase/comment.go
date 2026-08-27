package usecase

import (
	"context"

	"github.com/google/uuid"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/task-service/internal/domain"
)

type AddComment struct{ comments CommentRepository }

func NewAddComment(comments CommentRepository) *AddComment { return &AddComment{comments: comments} }

func (uc *AddComment) Execute(ctx context.Context, taskID, content string) (domain.TaskComment, error) {
	tenantID, err := tenant.RequireTenantID(ctx)
	if err != nil {
		return domain.TaskComment{}, apperrors.New(apperrors.KindUnauthenticated, "TASK_NO_TENANT", "no tenant in request context", err)
	}
	userID, _ := tenant.UserID(ctx)
	c, err := domain.NewTaskComment(uuid.NewString(), taskID, userID, content)
	if err != nil {
		return domain.TaskComment{}, apperrors.New(apperrors.KindInvalidArgument, "TASK_COMMENT_INVALID", err.Error(), err)
	}
	out, err := uc.comments.AddComment(ctx, tenantID, c)
	if err != nil {
		return domain.TaskComment{}, apperrors.New(apperrors.KindInternal, "TASK_COMMENT_ADD_FAILED", "failed to persist comment", err)
	}
	return out, nil
}

type ListComments struct{ comments CommentRepository }

func NewListComments(comments CommentRepository) *ListComments {
	return &ListComments{comments: comments}
}

func (uc *ListComments) Execute(ctx context.Context, taskID, pageToken string, pageSize int32) ([]domain.TaskComment, string, error) {
	tenantID, err := tenant.RequireTenantID(ctx)
	if err != nil {
		return nil, "", apperrors.New(apperrors.KindUnauthenticated, "TASK_NO_TENANT", "no tenant in request context", err)
	}
	return uc.comments.ListComments(ctx, tenantID, taskID, pageToken, pageSize)
}
