package usecase

import (
	"context"
	"time"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/notification-service/internal/domain"
)

type MarkAsRead struct {
	repo        NotificationRepository
	broadcaster NotificationBroadcaster
}

func NewMarkAsRead(repo NotificationRepository, broadcaster NotificationBroadcaster) *MarkAsRead {
	return &MarkAsRead{repo: repo, broadcaster: broadcaster}
}

func (uc *MarkAsRead) Execute(ctx context.Context, userID, notificationID string) error {
	tenantID, err := tenant.RequireTenantID(ctx)
	if err != nil {
		return apperrors.New(apperrors.KindUnauthenticated, "NOTIFICATION_NO_TENANT", "no tenant in request context", err)
	}
	if userID == "" || notificationID == "" {
		return apperrors.New(apperrors.KindInvalidArgument, "NOTIFICATION_MISSING_FIELD", "user_id and notification_id are required", nil)
	}
	if err := uc.repo.MarkAsRead(ctx, tenantID, userID, notificationID); err != nil {
		return apperrors.New(apperrors.KindInternal, "NOTIFICATION_MARK_READ_FAILED", "failed to mark notification as read", err)
	}
	// Read receipt: reuses the existing NotificationEvent/Broadcaster
	// pipeline (no new channel/struct) so every WS-connected session of
	// this user sees is_read flip in real time — required for the
	// multi-tab/multi-device SSH remote-workflow case (AGENTS.md). Only
	// fires after persist succeeds — never broadcast "read" if the DB
	// write failed, or a client's view would drift from the DB.
	uc.broadcaster.Broadcast(ctx, domain.NotificationEvent{
		ID: notificationID, TenantID: tenantID, RecipientUserIDs: []string{userID},
		Type: "notification_read", IsRead: true, CreatedAt: time.Now().UTC(),
	})
	return nil
}
