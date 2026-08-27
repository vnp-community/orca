// binary_stream_registry.go is the transport-level plumbing
// terminal.multiplex (channels_terminal_multiplex.go) needs to speak the
// binary, opcode-framed sub-protocol (terminal_stream_frame.go) on the SAME
// WebSocket connection its "invoke" arrived on, alongside the existing
// JSON invoke/send/push traffic every other channel uses. Kept generic
// (nothing terminal-specific lives here) so a future binary-framed channel
// can reuse it without touching terminal.* code — mirrors
// channels_terminal.go's terminalStreamRegistry/ctx-threading pattern, one
// layer down at the transport instead of one service's business logic.
package wscompat

import (
	"context"
	"encoding/json"
	"sync"
)

// BinaryFrameHandler processes one decoded TerminalStreamFrame addressed to
// a specific streamId (including the control streamId 0).
type BinaryFrameHandler func(frame TerminalStreamFrame)

// BinaryStreamIO is what a BinaryStreamChannelHandler gets to speak the
// binary sub-protocol — the Go counterpart of the old TS backend's
// sendBinary/registerBinaryStreamHandler pair
// (backend/src/main/runtime/rpc/methods/terminal.ts's terminal.multiplex
// handler signature).
type BinaryStreamIO struct {
	// SendBinary writes one already-encoded frame (EncodeTerminalStreamFrame)
	// to the client as a WS binary message. Returns false if the write
	// failed (e.g. the connection is gone) — mirrors sendBinary's boolean
	// return in the TS source, which callers use to short-circuit a dead
	// stream instead of treating it as a hard error.
	SendBinary func(frame []byte) bool
	// RegisterFrameHandler routes every inbound binary frame whose StreamID
	// equals streamID to h, until the returned unregister func is called.
	// StreamID 0 is the control channel (Subscribe frames arrive there —
	// mirrors terminal.ts's `registerBinaryStreamHandler(0, ...)`).
	RegisterFrameHandler func(streamID uint32, h BinaryFrameHandler) (unregister func())
}

// BinaryStreamChannelHandler is a channel whose invoke sets up the binary
// opcode-framed sub-protocol for the rest of THIS connection's life (e.g.
// terminal.multiplex) instead of (or alongside) ordinary JSON push traffic.
// Registered via Registry.RegisterBinaryStreamHandler, parallel to
// Register/RegisterStream/RegisterStreamChannel — a channel is exactly one
// of these four, never more than one. The handler is expected to return
// quickly with an ack after registering whatever frame handlers/background
// goroutines it needs (bound to the connection-lifetime ctx it's called
// with — see Handler.handleInvoke, which passes ctx, not the 25s-bounded
// dispatchCtx, into BinaryStreamIO precisely so the sub-protocol outlives
// one invoke's own deadline).
type BinaryStreamChannelHandler func(ctx context.Context, id Identity, args []json.RawMessage, io BinaryStreamIO) (ack any, err error)

// binaryStreamRouter demuxes inbound binary WS frames by StreamID to
// per-streamId handlers — scoped to ONE WebSocket connection (constructed
// once in Handler.ServeHTTP), same per-connection-not-global principle
// channels_terminal.go's terminalStreamRegistry documents, for the same
// reason: StreamID is client-chosen and not guaranteed unique across
// unrelated connections.
type binaryStreamRouter struct {
	mu       sync.Mutex
	handlers map[uint32]BinaryFrameHandler
}

func newBinaryStreamRouter() *binaryStreamRouter {
	return &binaryStreamRouter{handlers: make(map[uint32]BinaryFrameHandler)}
}

func (r *binaryStreamRouter) register(streamID uint32, h BinaryFrameHandler) func() {
	r.mu.Lock()
	r.handlers[streamID] = h
	r.mu.Unlock()
	return func() {
		r.mu.Lock()
		delete(r.handlers, streamID)
		r.mu.Unlock()
	}
}

// dispatch routes frame to its registered handler, if any. A frame for a
// streamId with no registered handler is silently dropped — e.g. a stray
// frame that arrived after Unsubscribe already tore the slot down.
func (r *binaryStreamRouter) dispatch(frame TerminalStreamFrame) {
	r.mu.Lock()
	h, ok := r.handlers[frame.StreamID]
	r.mu.Unlock()
	if ok {
		h(frame)
	}
}

type binaryStreamRouterCtxKey struct{}

// binaryStreamRouterContext attaches router as ctx's per-connection binary
// frame router. Called once per WebSocket connection, from
// Handler.ServeHTTP, before that connection's read/dispatch loop starts —
// mirrors terminalStreamsContext.
func binaryStreamRouterContext(ctx context.Context, router *binaryStreamRouter) context.Context {
	return context.WithValue(ctx, binaryStreamRouterCtxKey{}, router)
}

// binaryStreamRouterFromContext resolves the calling connection's
// binaryStreamRouter, or nil if ctx was never wrapped (a wiring bug — every
// real request path wraps it in ServeHTTP; only a test that skips that setup
// would see nil).
func binaryStreamRouterFromContext(ctx context.Context) *binaryStreamRouter {
	router, _ := ctx.Value(binaryStreamRouterCtxKey{}).(*binaryStreamRouter)
	return router
}
