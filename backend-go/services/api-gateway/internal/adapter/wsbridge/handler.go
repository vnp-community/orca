// Package wsbridge implements api-gateway's WS<->gRPC-stream bridge — the
// most architecturally distinctive piece of this service (§8's sequence
// diagram) — for notification-service's real StreamNotifications RPC.
// Every other WS endpoint sketched in api-gateway.md §3's table (terminal
// relay, agent-status) is not implemented here; only notification-service's
// is real end-to-end in this scaffold, since it's the one owning service
// that already implements the RPC for real (see README "what's really
// wired").
package wsbridge

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"

	"net/http"

	gatewaygrpc "github.com/stablyai/orca-go/services/api-gateway/internal/adapter/grpc"
	"github.com/stablyai/orca-go/services/api-gateway/internal/usecase"

	notificationv1 "github.com/stablyai/orca-go/proto/gen/go/orca/notification/v1"
)

// StreamOpener opens a live StreamNotifications gRPC call for userID,
// scoped to ctx (cancelled when the WS connection closes). Implemented in
// internal/adapter/grpc against a shared *grpc.ClientConn to
// notification-service; kept as a function type here so this handler is
// unit-testable without a real gRPC connection.
type StreamOpener func(ctx context.Context, userID string) (notificationv1.NotificationService_StreamNotificationsClient, error)

// Handler serves /v1/notifications/stream: resolves the caller's identity
// (placeholder-verified, see usecase.AuthValidator), upgrades to WS, opens
// notification-service's real StreamNotifications gRPC call for that user,
// and pumps received messages to the WS connection as JSON frames until
// either side closes — the concrete implementation of api-gateway.md §8's
// sequence diagram for this one endpoint.
type Handler struct {
	Logger *slog.Logger
	Auth   *usecase.AuthValidator
	Open   StreamOpener
}

// New returns a Handler ready to mount at /v1/notifications/stream.
func New(logger *slog.Logger, auth *usecase.AuthValidator, open StreamOpener) *Handler {
	return &Handler{Logger: logger, Auth: auth, Open: open}
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	identity, err := h.Auth.Validate(r)
	if err != nil {
		http.Error(w, "unauthenticated: "+err.Error(), http.StatusUnauthorized)
		return
	}

	// InsecureSkipVerify: this scaffold has no CORS/origin allow-list
	// wired yet (out of scope for this pass); production must set
	// OriginPatterns to the real frontend/mobile origins before this
	// serves real traffic — see README "Known gaps".
	conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{InsecureSkipVerify: true})
	if err != nil {
		h.Logger.ErrorContext(r.Context(), "ws upgrade failed", slog.Any("error", err))
		return
	}
	defer conn.CloseNow()

	// CloseRead spins up a background reader that discards inbound frames
	// (StreamNotifications is server-push only, §3 — there is no
	// WS-to-gRPC direction for this endpoint) and, critically, returns a
	// context that's cancelled the moment the client closes the
	// connection or a read error occurs. That cancellation is what tears
	// down the gRPC stream below (§8: "client closes WS -> gateway cancels
	// gRPC stream").
	ctx := conn.CloseRead(r.Context())

	connectionID := identity.UserID + "-" + time.Now().UTC().Format(time.RFC3339Nano)

	grpcStream, err := h.Open(ctx, identity.UserID)
	if err != nil {
		h.Logger.ErrorContext(ctx, "opening notification stream failed",
			slog.String("connection_id", connectionID), slog.Any("error", err))
		_ = conn.Close(websocket.StatusInternalError, "upstream stream unavailable")
		return
	}

	stream := gatewaygrpc.NewNotificationStream(grpcStream)
	writer := &wsFrameWriter{conn: conn, ctx: ctx}

	bridgeErr := usecase.BridgeWSSession(ctx, h.Logger, connectionID, stream, writer)
	switch {
	case bridgeErr == nil:
		_ = conn.Close(websocket.StatusNormalClosure, "stream ended")
	case errors.Is(bridgeErr, context.Canceled):
		// Client closed the WS — CloseRead already tore this down; no
		// further Close call needed (and would just double-close).
	default:
		h.Logger.WarnContext(ctx, "ws bridge ended with error",
			slog.String("connection_id", connectionID), slog.Any("error", bridgeErr))
		_ = conn.Close(websocket.StatusInternalError, "bridge error")
	}
}

// wsFrameWriter adapts a *websocket.Conn to usecase.WSWriter.
type wsFrameWriter struct {
	conn *websocket.Conn
	ctx  context.Context
}

func (w *wsFrameWriter) WriteJSON(frame usecase.Frame) error {
	return wsjson.Write(w.ctx, w.conn, frame)
}
