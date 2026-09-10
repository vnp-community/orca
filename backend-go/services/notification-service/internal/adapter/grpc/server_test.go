package grpc

import (
	"context"
	"errors"
	"testing"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/notification-service/internal/domain"
	"github.com/stablyai/orca-go/services/notification-service/internal/usecase"

	notificationv1 "github.com/stablyai/orca-go/proto/gen/go/orca/notification/v1"
)

// fakeNotificationRepository is a minimal in-memory usecase.NotificationRepository
// double, local to this package's gRPC-mapping tests — it only needs to
// satisfy the interface, not persist anything real (that's covered by the
// usecase package's own tests and the Postgres adapter's integration tests).
type fakeNotificationRepository struct {
	listEvents []domain.NotificationEvent
	listCursor string
	listErr    error

	markAsReadErr error

	markAllCount int64
	markAllErr   error

	countUnread    int64
	countUnreadErr error
}

func (f *fakeNotificationRepository) SaveNotificationEvent(ctx context.Context, event domain.NotificationEvent) error {
	return nil
}

func (f *fakeNotificationRepository) ListByRecipient(ctx context.Context, tenantID, userID, cursor string, limit int32, unreadOnly bool) ([]domain.NotificationEvent, string, error) {
	return f.listEvents, f.listCursor, f.listErr
}

func (f *fakeNotificationRepository) MarkAsRead(ctx context.Context, tenantID, userID, notificationID string) error {
	return f.markAsReadErr
}

func (f *fakeNotificationRepository) MarkAllAsRead(ctx context.Context, tenantID, userID string) (int64, error) {
	return f.markAllCount, f.markAllErr
}

func (f *fakeNotificationRepository) CountUnread(ctx context.Context, tenantID, userID string) (int64, error) {
	return f.countUnread, f.countUnreadErr
}

// noopBroadcaster satisfies usecase.NotificationBroadcaster for tests in
// this package that don't care about the read-receipt broadcast itself
// (that behavior is covered by the usecase package's own tests).
type noopBroadcaster struct{}

func (noopBroadcaster) Subscribe(ctx context.Context, tenantID, userID string) (<-chan domain.NotificationEvent, func()) {
	ch := make(chan domain.NotificationEvent)
	return ch, func() { close(ch) }
}

func (noopBroadcaster) Broadcast(ctx context.Context, event domain.NotificationEvent) {}

func ctxWithTenant(t *testing.T) context.Context {
	t.Helper()
	return tenant.WithTenantID(context.Background(), "tenant-1")
}

func TestServer_ListNotifications_MapsUsecaseOutputToProto(t *testing.T) {
	created := time.Date(2026, 9, 9, 12, 0, 0, 0, time.UTC)
	repo := &fakeNotificationRepository{
		listEvents: []domain.NotificationEvent{
			{
				ID: "n1", Type: "task.assigned", Title: "t", Body: "b",
				DeepLink: "orca://task/1", Severity: domain.SeverityInfo,
				IsRead: true, CreatedAt: created,
			},
		},
		listCursor: "next-cursor",
	}
	srv := New(nil, nil, nil, usecase.NewListNotifications(repo), nil, nil, nil, nil, nil)

	resp, err := srv.ListNotifications(ctxWithTenant(t), &notificationv1.ListNotificationsRequest{UserId: "user-1"})
	if err != nil {
		t.Fatalf("ListNotifications: %v", err)
	}
	if len(resp.GetNotifications()) != 1 {
		t.Fatalf("expected 1 notification, got %d", len(resp.GetNotifications()))
	}
	got := resp.GetNotifications()[0]
	if got.GetId() != "n1" || got.GetTitle() != "t" || !got.GetIsRead() {
		t.Fatalf("unexpected proto mapping: %+v", got)
	}
	if got.GetCreatedAt() != created.Format(time.RFC3339) {
		t.Fatalf("expected RFC3339 created_at, got %q", got.GetCreatedAt())
	}
	if resp.GetNextCursor() != "next-cursor" {
		t.Fatalf("expected next_cursor to pass through, got %q", resp.GetNextCursor())
	}
}

func TestServer_ListNotifications_UsecaseErrorMapsToGRPCStatus(t *testing.T) {
	repo := &fakeNotificationRepository{listErr: errors.New("boom")}
	srv := New(nil, nil, nil, usecase.NewListNotifications(repo), nil, nil, nil, nil, nil)

	_, err := srv.ListNotifications(ctxWithTenant(t), &notificationv1.ListNotificationsRequest{UserId: "user-1"})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if status.Code(err) != codes.Internal {
		t.Fatalf("expected codes.Internal, got %v", status.Code(err))
	}
}

func TestServer_MarkAsRead_ReturnsEmptyOnSuccess(t *testing.T) {
	repo := &fakeNotificationRepository{}
	srv := New(nil, nil, nil, nil, usecase.NewMarkAsRead(repo, noopBroadcaster{}), nil, nil, nil, nil)

	resp, err := srv.MarkAsRead(ctxWithTenant(t), &notificationv1.MarkAsReadRequest{UserId: "user-1", NotificationId: "n1"})
	if err != nil {
		t.Fatalf("MarkAsRead: %v", err)
	}
	if resp == nil {
		t.Fatal("expected non-nil Empty response")
	}
}

func TestServer_GetUnreadCount_ReturnsCountFromUsecase(t *testing.T) {
	repo := &fakeNotificationRepository{countUnread: 4}
	srv := New(nil, nil, nil, nil, nil, nil, usecase.NewGetUnreadCount(repo), nil, nil)

	resp, err := srv.GetUnreadCount(ctxWithTenant(t), &notificationv1.GetUnreadCountRequest{UserId: "user-1"})
	if err != nil {
		t.Fatalf("GetUnreadCount: %v", err)
	}
	if resp.GetCount() != 4 {
		t.Fatalf("expected count 4, got %d", resp.GetCount())
	}
}
