package grpc

import (
	notificationv1 "github.com/stablyai/orca-go/proto/gen/go/orca/notification/v1"
	"github.com/stablyai/orca-go/services/api-gateway/internal/usecase"
)

// notificationStream adapts notificationv1's generated server-streaming
// client to usecase.NotificationStream, translating the wire message into
// the transport-agnostic usecase.Frame — no other translation happens here,
// per api-gateway.md §8's "never interpreting frame contents".
type notificationStream struct {
	client notificationv1.NotificationService_StreamNotificationsClient
}

// NewNotificationStream wraps a live StreamNotifications client stream.
func NewNotificationStream(client notificationv1.NotificationService_StreamNotificationsClient) usecase.NotificationStream {
	return &notificationStream{client: client}
}

func (s *notificationStream) Recv() (usecase.Frame, error) {
	msg, err := s.client.Recv()
	if err != nil {
		return usecase.Frame{}, err
	}
	return usecase.Frame{
		ID:          msg.GetId(),
		Type:        msg.GetType(),
		PayloadJSON: msg.GetPayloadJson(),
	}, nil
}
