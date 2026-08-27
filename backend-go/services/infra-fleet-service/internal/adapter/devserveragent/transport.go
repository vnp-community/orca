package devserveragent

import (
	"context"
	"log/slog"

	"github.com/coder/websocket"
)

// Transport is the byte-level connection a session runs its JSON-RPC frame
// protocol over. wsTransport (this file) backs relay-websocket's outbound
// dial and direct-websocket's inbound accept — both WebSocket-based.
// adapter/sshrelay implements this externally for relay-ssh's SSH exec
// channel stdio, where frames don't arrive as discrete messages the way a
// WebSocket already guarantees (see frame.go's doc comment on why
// DecodeFrame is single-shot for WS but relay-ssh needs an incremental
// decoder of its own).
type Transport interface {
	// ReadFrame blocks until one complete, well-formed frame is available,
	// ctx is cancelled, or the underlying connection fails. A malformed
	// frame is not a fatal error — a Transport implementation must skip it
	// and keep reading, only ReadFrame's OWN error return means the
	// connection itself is dead (matches the pre-abstraction behavior of
	// session.go's readLoop, which used to do this skip-and-continue
	// itself; now every Transport implementation owns that decision for its
	// own wire format).
	ReadFrame(ctx context.Context) (DecodedFrame, error)
	// WriteFrame writes one pre-encoded frame (from EncodeJSONRPCFrame or
	// EncodeKeepAliveFrame).
	WriteFrame(ctx context.Context, frame []byte) error
	// Close tears down the underlying connection. reason is a best-effort
	// diagnostic — not guaranteed to reach the peer, depending on the
	// concrete transport (a WS close frame carries it; an SSH exec channel
	// close does not).
	Close(reason string) error
}

// wsTransport adapts a *websocket.Conn to Transport — unchanged behavior
// from before this abstraction existed: DecodeFrame stays single-shot
// (a WS transport already delivers whole messages), and a decode failure
// is logged and skipped rather than treated as a dead connection, exactly
// as session.go's readLoop used to do inline.
type wsTransport struct {
	conn   *websocket.Conn
	logger *slog.Logger
}

func newWSTransport(conn *websocket.Conn, logger *slog.Logger) *wsTransport {
	return &wsTransport{conn: conn, logger: logger}
}

func (t *wsTransport) ReadFrame(ctx context.Context) (DecodedFrame, error) {
	for {
		_, data, err := t.conn.Read(ctx)
		if err != nil {
			return DecodedFrame{}, err
		}
		decoded, err := DecodeFrame(data)
		if err != nil {
			if t.logger != nil {
				t.logger.Warn("devserveragent: dropping malformed frame", slog.Any("error", err))
			}
			continue
		}
		return decoded, nil
	}
}

func (t *wsTransport) WriteFrame(ctx context.Context, frame []byte) error {
	return t.conn.Write(ctx, websocket.MessageBinary, frame)
}

func (t *wsTransport) Close(reason string) error {
	return t.conn.Close(websocket.StatusNormalClosure, reason)
}
