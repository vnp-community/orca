package wscompat

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"
)

// SessionValidator resolves the caller's identity from the orca_session
// cookie on the WS upgrade request — implemented for real by
// internal/adapter/authclient against auth-service.ValidateSession (not the
// unverified-JWT usecase.AuthValidator placeholder, since the cookie holds
// a raw session token, never a JWT). Kept as an interface here so this
// handler is unit-testable without a real auth-service connection.
type SessionValidator interface {
	ValidateCookie(ctx context.Context, r *http.Request) (Identity, error)
}

// wsCloseAuthRequired mirrors the old TS backend's WsSessionRouter exactly
// (backend/src/main/session/ws-session-router.ts:
// `ws.close(4401, 'Authentication required. Please log in first.')`) — a
// custom WebSocket close code in the 4000-4999 application range, sent
// AFTER the WS handshake completes, not an HTTP-level rejection.
//
// This distinction is load-bearing, not cosmetic: frontend/'s
// WebSocketRpcClient.connectInternal() resolves connect()'s promise on
// ws.onopen (i.e. on a successful 101 handshake) — it does not wait to see
// whether the server closes moments later. bootstrapWebApp() awaits
// client.connect() BEFORE ever rendering WebRootBoundary (where the real
// /auth/me check + LoginPage live) — see main-web-bootstrap.tsx. Rejecting
// the upgrade itself with an HTTP 401 (this handler's first version, found
// wrong live on 2026-08-17) makes connect() reject, bootstrapWebApp()
// retries a few times and then renders a hard "Cannot connect to Orca
// backend" error screen — the user can NEVER reach the login page, because
// bootstrap never gets past the WS-connect step. Always completing the
// handshake and closing with 4401 afterward is what lets bootstrap
// proceed to WebRootBoundary, which then correctly shows LoginPage for an
// unauthenticated session. installAuthFailedRedirect() (main-web-bootstrap.tsx)
// is the client-side handler for this same code once already logged in
// (session expiry mid-use), not just first boot.
const wsCloseAuthRequired websocket.StatusCode = 4401

// Handler serves /ws: ALWAYS completes the WS handshake (matching
// WsSessionRouter — see wsCloseAuthRequired's doc comment for why this is
// load-bearing, not a relaxed security posture), resolves the session
// cookie, and either closes with code 4401 if invalid/absent or proceeds
// to the normal read/dispatch loop, writing back
// ResultMessage/ErrorMessage per the wire format in envelope.go.
type Handler struct {
	Logger   *slog.Logger
	Auth     SessionValidator
	Registry *Registry
}

func New(logger *slog.Logger, auth SessionValidator, registry *Registry) *Handler {
	return &Handler{Logger: logger, Auth: auth, Registry: registry}
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// InsecureSkipVerify: see wsbridge.Handler's identical note — no
	// CORS/origin allow-list wired yet in this scaffold pass.
	conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{InsecureSkipVerify: true})
	if err != nil {
		h.Logger.ErrorContext(r.Context(), "wscompat: ws upgrade failed", slog.Any("error", err))
		return
	}
	defer conn.CloseNow()

	identity, err := h.Auth.ValidateCookie(r.Context(), r)
	if err != nil {
		_ = conn.Close(wsCloseAuthRequired, "Authentication required. Please log in first.")
		return
	}

	ctx := r.Context()

	// writeMu serializes writes to conn — coder/websocket, like most WS
	// libraries, does not allow concurrent writers on one connection. Reads
	// stay on this single goroutine (wsjson.Read below); every dispatched
	// invoke/send runs in its OWN goroutine (see handleInvoke/handleSend)
	// so one slow or hanging channel handler can no longer starve every
	// other in-flight request on the same connection — see the
	// "Concurrency" doc comment above ServeHTTP's declaration... actually
	// documented here since this is where the bug lived: found live
	// 2026-08-17 (docs/execution-plan.md §7) as
	// "Request timed out: preflight.check" — this loop used to read one
	// message, fully process it INLINE (including a downstream gRPC call
	// with no context deadline), and only then read the next message. Any
	// slow/unreachable downstream blocked every subsequent message on that
	// connection, including totally unrelated channels, until the client's
	// own 30s invoke timeout gave up. Reading and dispatching are now
	// decoupled: this loop's only job is to keep reading.
	var writeMu sync.Mutex

	for {
		var msg InboundMessage
		if err := wsjson.Read(ctx, conn, &msg); err != nil {
			// Normal on client disconnect/navigation — not logged as an error.
			return
		}

		switch msg.Type {
		case "invoke":
			go h.handleInvoke(ctx, conn, &writeMu, identity, msg)
		case "send":
			go h.handleSend(ctx, identity, msg)
		default:
			h.Logger.WarnContext(ctx, "wscompat: unknown message type", slog.String("type", msg.Type))
		}
	}
}

// invokeTimeout bounds every channel dispatch — matches rpc-client.ts's own
// INVOKE_TIMEOUT_MS (30s) so a hung downstream call fails with OUR error
// message before the client's own timeout fires with a generic, harder to
// diagnose "Request timed out: <channel>". Applied here (the transport
// layer) rather than per-handler in channels.go so every current AND
// future channel gets this for free, not just the ones wired so far.
const invokeTimeout = 25 * time.Second

func (h *Handler) handleInvoke(ctx context.Context, conn *websocket.Conn, writeMu *sync.Mutex, identity Identity, msg InboundMessage) {
	ctx, cancel := context.WithTimeout(ctx, invokeTimeout)
	defer cancel()

	result, err := h.Registry.Dispatch(ctx, identity, msg.Channel, msg.Args)

	writeMu.Lock()
	defer writeMu.Unlock()
	if err != nil {
		_ = wsjson.Write(ctx, conn, ErrorMessage{Type: "error", ID: msg.ID, Message: err.Error()})
		return
	}
	_ = wsjson.Write(ctx, conn, ResultMessage{Type: "result", ID: msg.ID, Result: result})
}

// handleSend dispatches a fire-and-forget "send" message the same way as
// "invoke", but never writes a response — matching rpc-client.ts's send(),
// which doesn't wait for one. Errors are logged, not surfaced to the
// client, since there's no request ID to correlate a response to.
func (h *Handler) handleSend(ctx context.Context, identity Identity, msg InboundMessage) {
	ctx, cancel := context.WithTimeout(ctx, invokeTimeout)
	defer cancel()

	var args []json.RawMessage
	if len(msg.Data) > 0 {
		args = []json.RawMessage{msg.Data}
	}
	if _, err := h.Registry.Dispatch(ctx, identity, msg.Channel, args); err != nil {
		h.Logger.WarnContext(ctx, "wscompat: send channel failed", slog.String("channel", msg.Channel), slog.Any("error", err))
	}
}
