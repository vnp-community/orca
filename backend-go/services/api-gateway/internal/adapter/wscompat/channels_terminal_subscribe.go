// channels_terminal_subscribe.go implements terminal.subscribe/
// terminal.unsubscribe/terminal.updateViewport — the plain-JSON terminal I/O
// fallback the web session client (WebSessionClient) actually uses. Found
// live 2026-08-30, via direct WebSocket-frame capture against a real deployed
// session: WebSessionClient cannot carry binary WS frames at all —
// handleSocketMessage's `if (typeof rawData !== 'string') return` drops any
// binary frame outright, and its subscribe()'s returned sendBinary
// unconditionally throws ("Binary frames not supported in session mode over
// this channel"). The frontend's TS interface declares sendBinary/onBinary
// on this client (satisfying RemoteRuntimeMultiplexedTerminalCallbacks
// structurally) but the implementation stubs them — an earlier pass in this
// same investigation mistook the type for real support and routed
// 'session-auth' terminals to terminal.multiplex (channels_terminal_multiplex.go),
// which backend-go implements correctly but which a WebSessionClient-backed
// session can never actually use. remote-runtime-terminal-json-subscribe.ts
// (unmodified frontend reference) is the real fallback contract this file
// implements: subscribe once (ack {"type":"subscribed"}, no scrollback —
// same accepted degradation as multiplex's own SnapshotRequest no-op), then
// stream {"type":"data","chunk":...} pushes and a final {"type":"end"} on
// exit; actual input/resize still ride the separate plain terminal.send/
// terminal.updateViewport RPCs (see those channels' own doc comments for the
// matching terminal/text field-name fix).
package wscompat

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"

	infrafleetv1 "github.com/stablyai/orca-go/proto/gen/go/orca/infrafleet/v1"

	gatewaygrpc "github.com/stablyai/orca-go/services/api-gateway/internal/adapter/grpc"
	"github.com/stablyai/orca-go/services/api-gateway/internal/usecase"
)

// terminalJsonSubscribeClient mirrors the real frontend's {id, type} shape —
// decoded only so decodeArg doesn't choke on it; no per-client behavior
// differs server-side (unlike terminal.multiplex's streamId-keyed
// multi-viewer slots).
type terminalJsonSubscribeClient struct {
	ID   string `json:"id"`
	Type string `json:"type"`
}

type terminalJsonSubscribeArgs struct {
	Terminal string                      `json:"terminal"`
	Client   terminalJsonSubscribeClient `json:"client"`
}

// terminalJsonSubscribeEvent covers both terminal.subscribe's own ack
// ({"type":"subscribed"}) and its streamed data pushes
// ({"type":"data","chunk":...}) — "end" is sent automatically by
// pipePushForDialect when the events channel closes (see
// sessionClientStreamEnd), never constructed here.
type terminalJsonSubscribeEvent struct {
	Type  string `json:"type"`
	Chunk string `json:"chunk,omitempty"`
}

// terminalJsonSubscribeRegistry tracks the live AttachPty stream each
// terminal.subscribe call opened, keyed by subscriptionId ("<ptyId>:<clientId>",
// matching remote-runtime-terminal-json-subscribe.ts's own convention) so
// terminal.unsubscribe can tear down the right one. Scoped to ONE WebSocket
// connection — same per-connection-not-shared rationale as
// terminalStreamRegistry (see channels_terminal.go's package doc comment):
// ptyId is agent-assigned and not guaranteed unique across connections.
type terminalJsonSubscribeRegistry struct {
	mu      sync.Mutex
	cancels map[string]context.CancelFunc
}

func newTerminalJSONSubscribeRegistry() *terminalJsonSubscribeRegistry {
	return &terminalJsonSubscribeRegistry{cancels: make(map[string]context.CancelFunc)}
}

type terminalJsonSubscribeCtxKey struct{}

// terminalJSONSubscribeContext attaches reg as ctx's per-connection registry —
// called once per WebSocket connection from Handler.ServeHTTP, alongside
// terminalStreamsContext/binaryStreamRouterContext.
func terminalJSONSubscribeContext(ctx context.Context, reg *terminalJsonSubscribeRegistry) context.Context {
	return context.WithValue(ctx, terminalJsonSubscribeCtxKey{}, reg)
}

func terminalJSONSubscribeFromContext(ctx context.Context) *terminalJsonSubscribeRegistry {
	reg, _ := ctx.Value(terminalJsonSubscribeCtxKey{}).(*terminalJsonSubscribeRegistry)
	return reg
}

func terminalSubscriptionKey(ptyID, clientID string) string {
	return ptyID + ":" + clientID
}

// registerTerminalSubscribeChannel opens its OWN AttachPty stream per
// subscribe call — the same one-stream-per-subscriber pattern
// channels_terminal_multiplex.go's subscribe() already established (the
// agent supports multiple concurrent attach streams to one pty; confirmed by
// that file's own working design), rather than trying to fan out
// terminal.create's own stream to a second consumer.
func registerTerminalSubscribeChannel(r *Registry, client infrafleetv1.InfraFleetServiceClient) {
	r.RegisterStreamChannel("terminal.subscribe", func(ctx context.Context, id Identity, args []json.RawMessage) (any, <-chan PushEvent, error) {
		in, err := decodeArg[terminalJsonSubscribeArgs](args, 0)
		if err != nil {
			return nil, nil, err
		}
		if in.Terminal == "" {
			return nil, nil, fmt.Errorf("wscompat: terminal.subscribe requires a terminal (ptyId)")
		}

		// See channels_terminal.go's attachContext doc comment: the stream
		// must outlive this one invoke's 25s deadline.
		streamCtx, cancel := attachContext(id)
		stream, err := client.AttachPty(streamCtx)
		if err != nil {
			cancel()
			return nil, nil, fmt.Errorf("wscompat: opening AttachPty stream for pty %q: %w", in.Terminal, err)
		}
		if err := stream.Send(&infrafleetv1.PtyClientFrame{
			Frame: &infrafleetv1.PtyClientFrame_Attach{Attach: &infrafleetv1.AttachToSession{PtyId: in.Terminal}},
		}); err != nil {
			cancel()
			return nil, nil, fmt.Errorf("wscompat: sending AttachPty's initial attach frame for pty %q: %w", in.Terminal, err)
		}

		if reg := terminalJSONSubscribeFromContext(ctx); reg != nil {
			key := terminalSubscriptionKey(in.Terminal, in.Client.ID)
			reg.mu.Lock()
			if prior, ok := reg.cancels[key]; ok {
				prior() // a stale subscription under the same key never got explicitly unsubscribed — replace it
			}
			reg.cancels[key] = cancel
			reg.mu.Unlock()
		}

		events := make(chan PushEvent)
		go func() {
			defer close(events)
			for {
				frame, err := stream.Recv()
				if err != nil {
					return // stream ended — io.EOF on a clean close, or a transport error either way
				}
				switch f := frame.GetFrame().(type) {
				case *infrafleetv1.PtyServerFrame_Out:
					// Plain string, not base64: outputProcessor.processData
					// (remote-runtime-pty-transport.ts) feeds this chunk
					// straight to xterm as text — unlike terminal.create's own
					// terminal.output push, which Go's json package
					// auto-base64-encodes as a []byte field ("data") and
					// which a DIFFERENT frontend consumer decodes separately.
					ev := PushEvent{
						Channel: "terminal.subscribe.event",
						Args:    []any{terminalJsonSubscribeEvent{Type: "data", Chunk: string(f.Out.GetData())}},
					}
					select {
					case events <- ev:
					case <-streamCtx.Done():
						return
					}
				case *infrafleetv1.PtyServerFrame_Exited:
					return // channel close -> pipePushForDialect auto-sends {"type":"end"}
				}
			}
		}()

		return terminalJsonSubscribeEvent{Type: "subscribed"}, events, nil
	})
}

// ── terminal.unsubscribe ────────────────────────────────────────────────

type terminalUnsubscribeArgs struct {
	SubscriptionID string `json:"subscriptionId"`
}

func registerTerminalUnsubscribeChannel(r *Registry) {
	r.Register("terminal.unsubscribe", func(ctx context.Context, _ Identity, args []json.RawMessage) (any, error) {
		in, err := decodeArg[terminalUnsubscribeArgs](args, 0)
		if err != nil {
			return nil, err
		}
		reg := terminalJSONSubscribeFromContext(ctx)
		if reg == nil {
			return nil, nil
		}
		reg.mu.Lock()
		cancel, ok := reg.cancels[in.SubscriptionID]
		delete(reg.cancels, in.SubscriptionID)
		reg.mu.Unlock()
		if ok {
			cancel() // unblocks the subscribe goroutine's Recv() and closes its events channel
		}
		return nil, nil
	})
}

// ── terminal.updateViewport ─────────────────────────────────────────────
//
// The plain-JSON fallback's actual resize mechanism — see
// remote-runtime-terminal-json-subscribe.ts's own doc comment ("No
// input/resize/viewport-claim support over this stream ... the existing
// plain-RPC fallback (terminal.send / terminal.updateViewport) fires
// instead"). Same underlying RPC as terminal.resize, different arg shape
// (nested viewport, matching the real frontend's call site) — kept as its
// own channel rather than aliased onto "terminal.resize" since the frontend
// never actually calls that name for this path.

type terminalUpdateViewportArgs struct {
	Terminal string `json:"terminal"`
	Viewport struct {
		Cols int32 `json:"cols"`
		Rows int32 `json:"rows"`
	} `json:"viewport"`
}

func registerTerminalUpdateViewportChannel(r *Registry, client infrafleetv1.InfraFleetServiceClient) {
	r.Register("terminal.updateViewport", func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
		in, err := decodeArg[terminalUpdateViewportArgs](args, 0)
		if err != nil {
			return nil, err
		}
		if in.Viewport.Cols <= 0 || in.Viewport.Rows <= 0 {
			return nil, nil
		}
		ctx = gatewaygrpc.AttachIdentity(ctx, usecase.Identity{TenantID: id.TenantID, UserID: id.UserID})
		_, err = client.ResizeTerminalSession(ctx, &infrafleetv1.ResizeTerminalSessionRequest{PtyId: in.Terminal, Cols: in.Viewport.Cols, Rows: in.Viewport.Rows})
		return nil, err
	})
}
