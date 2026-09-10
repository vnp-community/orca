package grpc

import (
	"context"
	"testing"
	"time"

	"google.golang.org/grpc/metadata"

	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/notification-service/internal/domain"

	notificationv1 "github.com/stablyai/orca-go/proto/gen/go/orca/notification/v1"
)

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
	}
}
