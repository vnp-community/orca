package usecase

import (
	"context"
	"time"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/notification-service/internal/domain"
)

// MarkAllAsRead mirrors mark_as_read.go's structure — validate tenant
// context, validate input, call the repo, map errors through apperrors,
// then broadcast a read receipt so other tabs/devices update in real time.
type MarkAllAsRead struct {
	repo        NotificationRepository
	broadcaster NotificationBroadcaster
}

func NewMarkAllAsRead(repo NotificationRepository, broadcaster NotificationBroadcaster) *MarkAllAsRead {
	return &MarkAllAsRead{repo: repo, broadcaster: broadcaster}
}

func (uc *MarkAllAsRead) Execute(ctx context.Context, userID string) (int64, error) {
	tenantID, err := tenant.RequireTenantID(ctx)
	if err != nil {
		return 0, apperrors.New(apperrors.KindUnauthenticated, "NOTIFICATION_NO_TENANT", "no tenant in request context", err)
	}
	if userID == "" {
		return 0, apperrors.New(apperrors.KindInvalidArgument, "NOTIFICATION_NO_USER", "user_id is required", nil)
	}
	count, err := uc.repo.MarkAllAsRead(ctx, tenantID, userID)
	if err != nil {
		return 0, apperrors.New(apperrors.KindInternal, "NOTIFICATION_MARK_ALL_READ_FAILED", "failed to mark all notifications as read", err)
	}
	// No single notificationID here — Type "notification_all_read" (no ID)
	// tells the client to reset its unread count to 0 rather than
	// decrementing for one specific id. Only fires after persist succeeds.
	uc.broadcaster.Broadcast(ctx, domain.NotificationEvent{
		TenantID: tenantID, RecipientUserIDs: []string{userID},
		Type: "notification_all_read", IsRead: true, CreatedAt: time.Now().UTC(),
	})
	return count, nil
}
