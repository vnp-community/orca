package usecase

import (
	"context"

	"github.com/stablyai/orca-go/services/notification-service/internal/domain"
)

// fakeNotificationRepository is an in-memory usecase.NotificationRepository
// shared by list_notifications_test.go/mark_as_read_test.go/
// mark_all_as_read_test.go/get_unread_count_test.go (CR-NOTIF-001's
// unread-state usecases).
type fakeNotificationRepository struct {
	saveErr error
	saved   []domain.NotificationEvent
	order   *[]string // shared with a broadcaster fake to assert call order

	listEvents                                       []domain.NotificationEvent
	listCursor                                       string
	listErr                                          error
	lastListTenantID, lastListUserID, lastListCursor string
	lastListLimit                                    int32
	lastListUnreadOnly                               bool

	markAsReadErr                                                  error
	lastMarkAsReadTenantID, lastMarkAsReadUserID, lastMarkAsReadID string

	markAllAsReadCount                     int64
	markAllAsReadErr                       error
	lastMarkAllTenantID, lastMarkAllUserID string

	countUnreadResult                  int64
	countUnreadErr                     error
	lastCountTenantID, lastCountUserID string
}

func (f *fakeNotificationRepository) SaveNotificationEvent(ctx context.Context, event domain.NotificationEvent) error {
	if f.order != nil {
		*f.order = append(*f.order, "save")
	}
	if f.saveErr != nil {
		return f.saveErr
	}
	f.saved = append(f.saved, event)
	return nil
}

func (f *fakeNotificationRepository) ListByRecipient(ctx context.Context, tenantID, userID, cursor string, limit int32, unreadOnly bool) ([]domain.NotificationEvent, string, error) {
	f.lastListTenantID, f.lastListUserID, f.lastListCursor = tenantID, userID, cursor
	f.lastListLimit, f.lastListUnreadOnly = limit, unreadOnly
	if f.listErr != nil {
		return nil, "", f.listErr
	}
	return f.listEvents, f.listCursor, nil
}

func (f *fakeNotificationRepository) MarkAsRead(ctx context.Context, tenantID, userID, notificationID string) error {
	f.lastMarkAsReadTenantID, f.lastMarkAsReadUserID, f.lastMarkAsReadID = tenantID, userID, notificationID
	return f.markAsReadErr
}

func (f *fakeNotificationRepository) MarkAllAsRead(ctx context.Context, tenantID, userID string) (int64, error) {
	f.lastMarkAllTenantID, f.lastMarkAllUserID = tenantID, userID
	if f.markAllAsReadErr != nil {
		return 0, f.markAllAsReadErr
	}
	return f.markAllAsReadCount, nil
}

func (f *fakeNotificationRepository) CountUnread(ctx context.Context, tenantID, userID string) (int64, error) {
	f.lastCountTenantID, f.lastCountUserID = tenantID, userID
	if f.countUnreadErr != nil {
		return 0, f.countUnreadErr
	}
	return f.countUnreadResult, nil
}
