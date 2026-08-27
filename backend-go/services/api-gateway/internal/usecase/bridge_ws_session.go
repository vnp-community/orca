package usecase

import (
	"context"
	"errors"
	"io"
	"log/slog"
)

// Frame is one message the gateway forwards from a bridged gRPC stream to a
// WS connection. Fields are deliberately generic (id/type/payload) — the
// gateway never interprets frame contents, per api-gateway.md §8: "a
// transport bridge, not a protocol translator".
type Frame struct {
	ID          string
	Type        string
	PayloadJSON string
}

// NotificationStream is the outbound port BridgeWSSession pumps from —
// implemented in internal/adapter/grpc against notificationv1's real
// StreamNotifications RPC. Kept as an interface here so the pump loop is
// unit-testable without a real gRPC connection.
type NotificationStream interface {
	// Recv blocks for the next frame. It returns io.EOF when the stream
	// ends normally (owning service closed it).
	Recv() (Frame, error)
}

// WSWriter is the inbound port BridgeWSSession pumps into — implemented in
// internal/adapter/wsbridge against the real WS connection.
type WSWriter interface {
	WriteJSON(v Frame) error
}

// BridgeWSSession pumps frames from stream to w until the stream ends
// (io.EOF), errors, the write to w fails, or ctx is cancelled (client
// closed the WS or the request context was torn down) — the pump loop
// behind the sequence diagram in api-gateway.md §8.
//
// Notifications are server-push only (notification-service's
// StreamNotifications has no client-to-server direction), so this is a
// one-directional pump. A bidirectional bridge (e.g. a future terminal
// relay over infra-fleet-service, per api-gateway.md §3's WS endpoint
// table) would additionally need a reverse WS-to-gRPC pump and the
// bounded-buffer backpressure policy §8 describes; not implemented in this
// scaffold — see README "what's really wired".
func BridgeWSSession(ctx context.Context, logger *slog.Logger, connectionID string, stream NotificationStream, w WSWriter) error {
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		frame, err := stream.Recv()
		if err != nil {
			if errors.Is(err, io.EOF) {
				logger.InfoContext(ctx, "ws bridge: upstream stream ended", slog.String("connection_id", connectionID))
				return nil
			}
			return err
		}

		if err := w.WriteJSON(frame); err != nil {
			return err
		}
	}
}
