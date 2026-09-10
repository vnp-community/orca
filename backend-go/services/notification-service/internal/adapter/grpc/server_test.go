package grpc

import (
	"context"
<<<<<<< HEAD
	"testing"
	"time"

	"google.golang.org/grpc/metadata"

	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/notification-service/internal/domain"
=======
	"errors"
	"testing"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/notification-service/internal/domain"
	"github.com/stablyai/orca-go/services/notification-service/internal/usecase"
>>>>>>> feat/team-rbac-implementation

	notificationv1 "github.com/stablyai/orca-go/proto/gen/go/orca/notification/v1"
)

<<<<<<< HEAD
// --- fakes -------------------------------------------------------------

type fakeBroadcaster struct {
	ch <-chan domain.NotificationEvent
}

func (f *fakeBroadcaster) Subscribe(ctx context.Context, tenantID, userID string) (<-chan domain.NotificationEvent, func()) {
	if f.ch != nil {
		return f.ch, func() {}
	}
	ch := make(chan domain.NotificationEvent)
	return ch, func() {}
}
func (f *fakeBroadcaster) Broadcast(ctx context.Context, event domain.NotificationEvent) {}

type fakeBufferRepo struct {
	pending   []domain.BufferedNotification
	delivered []string
	listErr   error
}

func (f *fakeBufferRepo) Enqueue(ctx context.Context, tenantID, userID, subscriptionID string, eventJSON []byte) error {
	return nil
}
func (f *fakeBufferRepo) ListPending(ctx context.Context, tenantID, userID string) ([]domain.BufferedNotification, error) {
	return f.pending, f.listErr
}
func (f *fakeBufferRepo) MarkDelivered(ctx context.Context, ids []string) error {
	f.delivered = append(f.delivered, ids...)
	return nil
}

// fakeStream implements grpc.ServerStreamingServer[NotificationServiceStreamNotificationsResponse]
// well enough for StreamNotifications' handler logic — it records every
// sent frame and lets the test end the stream deterministically by
// cancelling ctx after the expected number of Send calls.
type fakeStream struct {
	ctx    context.Context
	cancel context.CancelFunc
	sent   []*notificationv1.NotificationServiceStreamNotificationsResponse
	stopAt int
}

func (s *fakeStream) Send(m *notificationv1.NotificationServiceStreamNotificationsResponse) error {
	s.sent = append(s.sent, m)
	if s.stopAt > 0 && len(s.sent) >= s.stopAt {
		s.cancel()
	}
	return nil
}
func (s *fakeStream) Context() context.Context     { return s.ctx }
func (s *fakeStream) SetHeader(metadata.MD) error  { return nil }
func (s *fakeStream) SendHeader(metadata.MD) error { return nil }
func (s *fakeStream) SetTrailer(metadata.MD)       {}
func (s *fakeStream) SendMsg(m any) error          { return nil }
func (s *fakeStream) RecvMsg(m any) error          { return nil }

func TestStreamNotifications_DrainsBufferedBacklogBeforeLiveLoop(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	ctx = tenant.WithTenantID(ctx, "tenant-1")

	buffer := &fakeBufferRepo{
		pending: []domain.BufferedNotification{
			{ID: "buf-1", Event: domain.NotificationEvent{ID: "ne-1", Type: "agent_completed", Title: "done 1"}},
			{ID: "buf-2", Event: domain.NotificationEvent{ID: "ne-2", Type: "agent_completed", Title: "done 2"}},
		},
	}
	srv := New(nil, nil, nil, &fakeBroadcaster{}, nil, buffer)

	stream := &fakeStream{ctx: ctx, cancel: cancel, stopAt: 2}
	err := srv.StreamNotifications(&notificationv1.StreamNotificationsRequest{UserId: "user-1"}, stream)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(stream.sent) != 2 {
		t.Fatalf("expected 2 buffered frames sent before the live loop, got %d", len(stream.sent))
	}
	if stream.sent[0].Id != "ne-1" || stream.sent[1].Id != "ne-2" {
		t.Fatalf("expected buffered frames sent oldest-first, got %v", stream.sent)
	}
	if len(buffer.delivered) != 2 {
		t.Fatalf("expected both buffered rows marked delivered, got %v", buffer.delivered)
	}
}

func TestStreamNotifications_NilBuffer_SkipsDrainWithoutPanic(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	ctx = tenant.WithTenantID(ctx, "tenant-1")

	srv := New(nil, nil, nil, &fakeBroadcaster{}, nil, nil)
	stream := &fakeStream{ctx: ctx, cancel: cancel}
	if err := srv.StreamNotifications(&notificationv1.StreamNotificationsRequest{UserId: "user-1"}, stream); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(stream.sent) != 0 {
		t.Fatalf("expected no frames sent, got %d", len(stream.sent))
=======
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
>>>>>>> feat/team-rbac-implementation
	}
}
