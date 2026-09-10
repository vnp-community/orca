package usecase

import (
	"context"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/common/tenant"
)

// GetUnreadCount mirrors mark_as_read.go's structure — validate tenant
// context, validate input, call the repo, map errors through apperrors.
type GetUnreadCount struct {
	repo NotificationRepository
}

func NewGetUnreadCount(repo NotificationRepository) *GetUnreadCount {
	return &GetUnreadCount{repo: repo}
}

func (uc *GetUnreadCount) Execute(ctx context.Context, userID string) (int64, error) {
	tenantID, err := tenant.RequireTenantID(ctx)
	if err != nil {
		return 0, apperrors.New(apperrors.KindUnauthenticated, "NOTIFICATION_NO_TENANT", "no tenant in request context", err)
	}
	if userID == "" {
		return 0, apperrors.New(apperrors.KindInvalidArgument, "NOTIFICATION_NO_USER", "user_id is required", nil)
	}
	count, err := uc.repo.CountUnread(ctx, tenantID, userID)
	if err != nil {
		return 0, apperrors.New(apperrors.KindInternal, "NOTIFICATION_UNREAD_COUNT_FAILED", "failed to get unread count", err)
	}
	return count, nil
}
