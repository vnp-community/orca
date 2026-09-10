// Package grpc implements the generated notificationv1.NotificationServiceServer
// interface by translating wire messages to/from usecase calls — no
// business logic here, per
// specs/backend-go/architecture/03-clean-architecture-guidelines.md's
// inbound-adapter contract.
package grpc

import (
	"context"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/protobuf/types/known/emptypb"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/notification-service/internal/domain"
	"github.com/stablyai/orca-go/services/notification-service/internal/usecase"

	notificationv1 "github.com/stablyai/orca-go/proto/gen/go/orca/notification/v1"
)

// Server implements notificationv1.UnimplementedNotificationServiceServer.
type Server struct {
	notificationv1.UnimplementedNotificationServiceServer

	subscribe                  *usecase.Subscribe
	unregisterPushSubscription *usecase.UnregisterPushSubscription
	getVapidPublicKey          *usecase.GetVapidPublicKey
	broadcaster                usecase.NotificationBroadcaster
	// signer backs GetVapidPublicKey's sibling web-push signing path,
	// actually invoked now by usecase.DeliverPush (BL-MB-02,
	// TASK-MB-02-07) — no longer "wired but never called".
	signer usecase.VaultSigner
	// buffer drains BR-MB-07's offline-buffered notifications on
	// StreamNotifications reconnect, before the live broadcast loop starts
	// (TASK-MB-02-08).
	buffer usecase.BufferedNotificationRepository

	// listNotifications/markAsRead/markAllAsRead/getUnreadCount back
	// CR-NOTIF-001's unread-state RPCs (TASK-BE-NOTIF-005/006).
	listNotifications *usecase.ListNotifications
	markAsRead        *usecase.MarkAsRead
	markAllAsRead     *usecase.MarkAllAsRead
	getUnreadCount    *usecase.GetUnreadCount
}

func New(
	subscribe *usecase.Subscribe,
	unregisterPushSubscription *usecase.UnregisterPushSubscription,
	getVapidPublicKey *usecase.GetVapidPublicKey,
	broadcaster usecase.NotificationBroadcaster,
	signer usecase.VaultSigner,
	buffer usecase.BufferedNotificationRepository,
	listNotifications *usecase.ListNotifications,
	markAsRead *usecase.MarkAsRead,
	markAllAsRead *usecase.MarkAllAsRead,
	getUnreadCount *usecase.GetUnreadCount,
) *Server {
	return &Server{
		subscribe:                  subscribe,
		unregisterPushSubscription: unregisterPushSubscription,
		getVapidPublicKey:          getVapidPublicKey,
		broadcaster:                broadcaster,
		signer:                     signer,
		buffer:                     buffer,
		listNotifications:          listNotifications,
		markAsRead:                 markAsRead,
		markAllAsRead:              markAllAsRead,
		getUnreadCount:             getUnreadCount,
	}
}

func (s *Server) Subscribe(ctx context.Context, req *notificationv1.SubscribeRequest) (*notificationv1.SubscribeResponse, error) {
	sub, err := s.subscribe.Execute(ctx, usecase.SubscribeInput{
		UserID:    req.GetUserId(),
		Endpoint:  req.GetEndpoint(),
		P256dhKey: req.GetP256DhKey(),
		AuthKey:   req.GetAuthKey(),
	})
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	return &notificationv1.SubscribeResponse{SubscriptionId: sub.ID}, nil
}

func (s *Server) UnregisterPushSubscription(ctx context.Context, req *notificationv1.UnregisterPushSubscriptionRequest) (*emptypb.Empty, error) {
	if err := s.unregisterPushSubscription.Execute(ctx, req.GetEndpoint()); err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	return &emptypb.Empty{}, nil
}

func (s *Server) GetVapidPublicKey(ctx context.Context, req *notificationv1.GetVapidPublicKeyRequest) (*notificationv1.GetVapidPublicKeyResponse, error) {
	key, err := s.getVapidPublicKey.Execute(ctx)
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	return &notificationv1.GetVapidPublicKeyResponse{PublicKey: key}, nil
}

func (s *Server) ListNotifications(ctx context.Context, req *notificationv1.ListNotificationsRequest) (*notificationv1.ListNotificationsResponse, error) {
	events, next, err := s.listNotifications.Execute(ctx, usecase.ListNotificationsInput{
		UserID:     req.GetUserId(),
		Cursor:     req.GetCursor(),
		Limit:      req.GetLimit(),
		UnreadOnly: req.GetUnreadOnly(),
	})
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	out := make([]*notificationv1.Notification, 0, len(events))
	for _, e := range events {
		out = append(out, &notificationv1.Notification{
			Id:        e.ID,
			Type:      e.Type,
			Title:     e.Title,
			Body:      e.Body,
			DeepLink:  e.DeepLink,
			Severity:  string(e.Severity),
			IsRead:    e.IsRead,
			CreatedAt: e.CreatedAt.Format(time.RFC3339),
		})
	}
	return &notificationv1.ListNotificationsResponse{Notifications: out, NextCursor: next}, nil
}

func (s *Server) MarkAsRead(ctx context.Context, req *notificationv1.MarkAsReadRequest) (*emptypb.Empty, error) {
	if err := s.markAsRead.Execute(ctx, req.GetUserId(), req.GetNotificationId()); err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	return &emptypb.Empty{}, nil
}

func (s *Server) MarkAllAsRead(ctx context.Context, req *notificationv1.MarkAllAsReadRequest) (*emptypb.Empty, error) {
	if _, err := s.markAllAsRead.Execute(ctx, req.GetUserId()); err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	return &emptypb.Empty{}, nil
}

func (s *Server) GetUnreadCount(ctx context.Context, req *notificationv1.GetUnreadCountRequest) (*notificationv1.GetUnreadCountResponse, error) {
	count, err := s.getUnreadCount.Execute(ctx, req.GetUserId())
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	return &notificationv1.GetUnreadCountResponse{Count: count}, nil
}

// StreamNotifications is a real, working server-streaming handler:
// api-gateway is the gRPC CLIENT of this RPC, one call per connected
// browser/mobile session (notification-service.md §7) — this service
// never dials api-gateway, it only serves the stream api-gateway opened.
// It registers the request as a broadcaster subscriber and loops sending
// frames until the client disconnects or the server shuts down.
func (s *Server) StreamNotifications(req *notificationv1.StreamNotificationsRequest, stream grpc.ServerStreamingServer[notificationv1.NotificationServiceStreamNotificationsResponse]) error {
	ctx := stream.Context()

	tenantID, err := tenant.RequireTenantID(ctx)
	if err != nil {
		return apperrors.ToGRPCStatus(apperrors.New(apperrors.KindUnauthenticated, "NOTIFICATION_NO_TENANT", "no tenant in request context", err))
	}
	userID := req.GetUserId()
	if userID == "" {
		return apperrors.ToGRPCStatus(apperrors.New(apperrors.KindInvalidArgument, "NOTIFICATION_NO_USER", "user_id is required", nil))
	}

	// Drain BR-MB-07's offline-buffered backlog before the live loop
	// starts (TASK-MB-02-08) — a mobile client reconnecting after being
	// offline sees everything it missed, oldest first, then live events.
	// A drain failure degrades to "nothing drained this reconnect", never
	// a stream error — a missed backlog isn't worth failing the whole
	// connection over.
	if s.buffer != nil {
		if pending, err := s.buffer.ListPending(ctx, tenantID, userID); err == nil {
			delivered := make([]string, 0, len(pending))
			for _, row := range pending {
				if err := stream.Send(toProtoFrame(row.Event)); err == nil {
					delivered = append(delivered, row.ID)
				}
			}
			if len(delivered) > 0 {
				_ = s.buffer.MarkDelivered(ctx, delivered)
			}
		}
	}

	ch, unsubscribe := s.broadcaster.Subscribe(ctx, tenantID, userID)
	defer unsubscribe()

	for {
		select {
		case <-ctx.Done():
			return nil
		case event, ok := <-ch:
			if !ok {
				return nil
			}
			if err := stream.Send(toProtoFrame(event)); err != nil {
				return err
			}
		}
	}
}

func toProtoFrame(e domain.NotificationEvent) *notificationv1.NotificationServiceStreamNotificationsResponse {
	return &notificationv1.NotificationServiceStreamNotificationsResponse{
		Id:          e.ID,
		Type:        e.Type,
		PayloadJson: framePayloadJSON(e),
	}
}
