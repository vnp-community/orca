package usecase

import (
	"context"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/notification-service/internal/domain"
)

type ListNotificationsInput struct {
	UserID     string
	Cursor     string
	Limit      int32
	UnreadOnly bool
}

// ListNotifications is the Notification Center's read path — tenantID
// comes from context (never trusted from input), matching every other
// usecase in this package.
type ListNotifications struct {
	repo NotificationRepository
}

func NewListNotifications(repo NotificationRepository) *ListNotifications {
	return &ListNotifications{repo: repo}
}

func (uc *ListNotifications) Execute(ctx context.Context, in ListNotificationsInput) ([]domain.NotificationEvent, string, error) {
	tenantID, err := tenant.RequireTenantID(ctx)
	if err != nil {
		return nil, "", apperrors.New(apperrors.KindUnauthenticated, "NOTIFICATION_NO_TENANT", "no tenant in request context", err)
	}
	if in.UserID == "" {
		return nil, "", apperrors.New(apperrors.KindInvalidArgument, "NOTIFICATION_NO_USER", "user_id is required", nil)
	}
	limit := in.Limit
	if limit <= 0 || limit > 100 {
		limit = 50 // default/cap — mirror the repo's index-friendly page size
	}
	events, next, err := uc.repo.ListByRecipient(ctx, tenantID, in.UserID, in.Cursor, limit, in.UnreadOnly)
	if err != nil {
		return nil, "", apperrors.New(apperrors.KindInternal, "NOTIFICATION_LIST_FAILED", "failed to list notifications", err)
	}
	return events, next, nil
}
