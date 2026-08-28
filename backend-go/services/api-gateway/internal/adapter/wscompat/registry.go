package wscompat

import (
	"context"
	"encoding/json"
	"fmt"
)

// Identity is the caller's resolved tenant/user, threaded into every
// channel handler the same way api-gateway/internal/adapter/grpc.AttachIdentity
// threads it onto outbound gRPC metadata for REST routes.
type Identity struct {
	TenantID string
	UserID   string
	// DeviceID is non-empty only for a mobile-paired-device JWT (SOL-MB-01,
	// minted by auth-service's CompleteDevicePairing/IssueToken.ExecuteForDevice)
	// — required by every mobile.* channel (channels_mobile_dispatch.go,
	// channels_mobile_status.go). A plain browser session's Identity never
	// carries one, so mobile.* channels stay unreachable from a browser
	// session.
	DeviceID string
}

// ChannelHandler implements one RPC channel (e.g. "task.create") — args are
// the raw JSON args array from the invoke envelope; the handler decodes
// whatever shape it expects. Returns the value to serialize into
// ResultMessage.Result, or an error whose Error() string becomes
// ErrorMessage.Message (kept short and user-legible, not a wrapped Go
// error chain — see apperrors.ToGRPCStatus's sibling rule at the gRPC
// boundary; this is the same discipline at the WS boundary).
type ChannelHandler func(ctx context.Context, id Identity, args []json.RawMessage) (any, error)

// Registry maps channel name -> handler. Every channel the frontend can
// call (see specs/frontend/api/rpc-catalog.md, 262 methods across 36
// namespaces) resolves through this map; anything not explicitly
// registered falls through to notImplementedHandler — which is the fix for
// the originally-reported bug (a missing channel used to silently return
// the SPA's index.html with HTTP 200 to a WebSocket upgrade; now every
// channel gets a real, protocol-correct response, even if that response is
// "not implemented yet").
type Registry struct {
	handlers              map[string]ChannelHandler
	streamHandlers        map[string]StreamHandler
	streamChannelHandlers map[string]StreamChannelHandler
	binaryStreamHandlers  map[string]BinaryStreamChannelHandler
}

func NewRegistry() *Registry {
	return &Registry{
		handlers:              make(map[string]ChannelHandler),
		streamHandlers:        make(map[string]StreamHandler),
		streamChannelHandlers: make(map[string]StreamChannelHandler),
		binaryStreamHandlers:  make(map[string]BinaryStreamChannelHandler),
	}
}

// StreamChannelHandler is a channel whose invoke ALSO opens a push
// subscription — e.g. terminal.create both acks with the new session (so the
// caller learns its ptyId) AND starts terminal.output/terminal.exited push
// frames. Distinct from StreamHandler (push_bridge.go): that one has no ack
// value and is used for pure-subscribe channels like notifications.subscribe,
// whose invoke resolves with nothing meaningful. A channel registers as
// EITHER a ChannelHandler, a StreamHandler, or a StreamChannelHandler, never
// more than one of the three.
type StreamChannelHandler func(ctx context.Context, id Identity, args []json.RawMessage) (ack any, events <-chan PushEvent, err error)

// RegisterStreamChannel adds or replaces the StreamChannelHandler for channel.
func (r *Registry) RegisterStreamChannel(channel string, h StreamChannelHandler) {
	r.streamChannelHandlers[channel] = h
}

// DispatchStreamChannel resolves and invokes channel's StreamChannelHandler,
// if one is registered. ok=false means channel has no StreamChannelHandler —
// the caller should fall back to the ordinary Dispatch path.
func (r *Registry) DispatchStreamChannel(ctx context.Context, id Identity, channel string, args []json.RawMessage) (ack any, events <-chan PushEvent, ok bool, err error) {
	h, found := r.streamChannelHandlers[channel]
	if !found {
		return nil, nil, false, nil
	}
	ack, events, err = h(ctx, id, args)
	return ack, events, true, err
}

// Register adds or replaces the handler for channel. Called from main.go's
// composition root, once per real channel this pass wires — see
// channels_*.go for the ones with actual backend-go logic behind them.
func (r *Registry) Register(channel string, h ChannelHandler) {
	r.handlers[channel] = h
}

// RegisterStream adds a StreamHandler for a push-capable channel (e.g.
// notifications.subscribe, see push_bridge.go's StreamHandler doc comment).
// Distinct from Register — a channel is either request/response or
// stream-registering, never both.
func (r *Registry) RegisterStream(channel string, h StreamHandler) {
	r.streamHandlers[channel] = h
}

// StreamHandlerFor resolves channel's registered StreamHandler, if any.
func (r *Registry) StreamHandlerFor(channel string) (StreamHandler, bool) {
	h, ok := r.streamHandlers[channel]
	return h, ok
}

// RegisterBinaryStreamHandler adds or replaces channel's
// BinaryStreamChannelHandler (binary_stream_registry.go) — e.g.
// terminal.multiplex. A channel is registered as exactly one of
// ChannelHandler, StreamHandler, StreamChannelHandler, or
// BinaryStreamChannelHandler, never more than one of the four.
func (r *Registry) RegisterBinaryStreamHandler(channel string, h BinaryStreamChannelHandler) {
	r.binaryStreamHandlers[channel] = h
}

// BinaryStreamHandlerFor resolves channel's registered
// BinaryStreamChannelHandler, if any — checked by Handler.handleInvoke
// before falling through to DispatchStreamChannel/Dispatch, the same
// "check the more specific registration first" shape handler.go's ServeHTTP
// already uses for StreamHandlerFor.
func (r *Registry) BinaryStreamHandlerFor(channel string) (BinaryStreamChannelHandler, bool) {
	h, ok := r.binaryStreamHandlers[channel]
	return h, ok
}

// Dispatch resolves and invokes the handler for channel, falling back to
// notImplementedHandler when nothing is registered.
func (r *Registry) Dispatch(ctx context.Context, id Identity, channel string, args []json.RawMessage) (any, error) {
	h, ok := r.handlers[channel]
	if !ok {
		return notImplementedHandler(ctx, id, channel)
	}
	return h(ctx, id, args)
}

func notImplementedHandler(_ context.Context, _ Identity, channel string) (any, error) {
	return nil, fmt.Errorf("channel %q is not yet implemented in backend-go — see backend-go/docs/execution-plan.md's frontend-compatibility-layer coverage table", channel)
}

// decodeArg is a small helper every channels_*.go handler uses to pull a
// typed value out of the raw args array at a given position.
func decodeArg[T any](args []json.RawMessage, index int) (T, error) {
	var v T
	if index >= len(args) {
		return v, fmt.Errorf("missing arg[%d]", index)
	}
	if err := json.Unmarshal(args[index], &v); err != nil {
		return v, fmt.Errorf("decoding arg[%d]: %w", index, err)
	}
	return v, nil
}

// decodeOptionalArg is decodeArg's tolerant counterpart, for handlers that
// must fall back to a zero-value struct instead of erroring when the arg is
// missing or malformed — e.g. registerEmulatorChannels/registerHostChannels
// (TASK-048/TASK-070), where a caller on an older frontend build that never
// sends a connectionId at all is a valid, expected case (the honest local
// stub answer), not a decode failure.
func decodeOptionalArg[T any](args []json.RawMessage, index int) T {
	var v T
	if index < len(args) {
		_ = json.Unmarshal(args[index], &v)
	}
	return v
}
