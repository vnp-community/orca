package wscompat

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"time"

	"google.golang.org/protobuf/proto"

	gatewaygrpc "github.com/stablyai/orca-go/services/api-gateway/internal/adapter/grpc"
	"github.com/stablyai/orca-go/services/api-gateway/internal/usecase"
)

// Identity is the caller's resolved tenant/user, threaded into every
// channel handler the same way api-gateway/internal/adapter/grpc.AttachIdentity
// threads it onto outbound gRPC metadata for REST routes.
type Identity struct {
	TenantID string
	UserID   string
	// Role is the caller's global role ("admin"/"user") — populated only by
	// authclient.SessionValidator's cookie/session path (the browser/web
	// path every wscompat call goes through), never by the bearer-JWT
	// path. See common/tenant.Role's doc comment for the fail-closed
	// contract: empty means "unknown," never "trust it."
	Role string
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
	// Why: attach identity here too, for consistency with Dispatch (TASK-001,
	// specs/backend-go/bugs/missing-v2/) — deliberately NO context.WithTimeout,
	// unlike Dispatch: a stream channel's ack/events lifecycle is long-lived
	// by design (e.g. a terminal session), not a single bounded RPC.
	// Role included — see Dispatch's own AttachIdentity call below for why
	// this must be set here, not left to individual channel handlers.
	ctx = gatewaygrpc.AttachIdentity(ctx, usecase.Identity{TenantID: id.TenantID, UserID: id.UserID, Role: id.Role})
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

// dispatchRPCTimeout is the OUTER safety-net deadline applied to every
// dispatched call, attached here so a handler that sets no deadline of its
// own is never unbounded — matches 08-inter-service-communication.md's
// "Deadlines are mandatory on every outbound call... no unbounded gRPC
// call exists anywhere in the system."
//
// Set to 60s, not that doc's "default 5s for intra-cluster calls" — a
// child context.WithTimeout can only ever SHRINK a parent's deadline, never
// extend it, so a short outer default here would silently clip every
// handler that documents and relies on a longer explicit override (found
// live, not guessed: channels_tenant_project.go's projectGroup.scanNested/
// projectHostSetup.setupExistingFolder both need 30s for a filesystem scan
// that can legitimately run long, channels_repo_ssh_status_workspace.go
// has one 60s and one 20s override — a 5s outer bound broke exactly those
// cases in this package's existing test suite). 60s matches the longest
// currently-documented per-handler override, so this bound is a true
// no-handler-left-unbounded safety net, never a silent tightening of an
// already-reasoned-about per-call budget — those handlers' own shorter
// group constants (rpcTimeout=8s, etc.) still apply correctly, since a
// handler requesting LESS than the remaining outer budget always wins.
const dispatchRPCTimeout = 60 * time.Second

// Dispatch resolves and invokes the handler for channel, falling back to
// notImplementedHandler when nothing is registered. Attaches the caller's
// identity onto ctx as outbound gRPC metadata and applies dispatchRPCTimeout
// ONCE here, and normalizes a nil-slice result to an empty one before
// returning — see specs/backend-go/bugs/missing-v2/BUG-001 and BUG-005 for
// why these must not be left to each handler to do individually.
func (r *Registry) Dispatch(ctx context.Context, id Identity, channel string, args []json.RawMessage) (any, error) {
	h, ok := r.handlers[channel]
	if !ok {
		return notImplementedHandler(ctx, id, channel)
	}
	// Role included — live-verified bug (CR-DS-006 Phase 2): this call
	// (generic, runs before every channel handler) previously omitted Role,
	// appending metadata.MetadataRole="" to the outgoing gRPC context.
	// Admin-gated channel handlers each ALSO call AttachIdentity a second
	// time via attachAdminIdentity (channels_dev_server_access_control.go),
	// this time with the real Role — but grpc/metadata.AppendToOutgoingContext
	// appends, it doesn't replace, and TenantExtractionInterceptor's
	// md.Get(MetadataRole) reads only the FIRST value in that slice. Since
	// this call runs first, its empty Role always won — the real "admin"
	// value from the second call was silently discarded, so every
	// requireAdmin(ctx) check failed with INFRA_NOT_ADMIN regardless of the
	// caller's actual role. Setting it correctly here (the call that always
	// runs first for every channel) fixes every admin-gated RPC at once and
	// makes attachAdminIdentity's own AttachIdentity call redundant-but-
	// harmless (same value appended twice, first one still wins).
	ctx = gatewaygrpc.AttachIdentity(ctx, usecase.Identity{TenantID: id.TenantID, UserID: id.UserID, Role: id.Role})
	ctx, cancel := context.WithTimeout(ctx, dispatchRPCTimeout)
	defer cancel()
	result, err := h(ctx, id, args)
	if err != nil {
		return nil, err
	}
	return normalizeNilSlices(result), nil
}

func notImplementedHandler(_ context.Context, _ Identity, channel string) (any, error) {
	return nil, fmt.Errorf("channel %q is not yet implemented in backend-go — see backend-go/docs/execution-plan.md's frontend-compatibility-layer coverage table", channel)
}

// normalizeNilSlices replaces a nil slice — at the top level, or one level
// into a plain (non-proto) value struct's exported fields — with a
// non-nil, empty slice of the same type, so encoding/json emits [] instead
// of null. See specs/backend-go/bugs/missing-v2/BUG-005: several channel
// handlers return a proto-generated getter (e.g. resp.GetGroups()) or a
// locally-declared `var xs []T` that stays nil for an empty result, and
// every real frontend caller for those channels destructures/iterates the
// result with no null-guard.
//
// Deliberately conservative: proto.Message values (the overwhelming
// majority of single-object channel results, e.g. *projectv1.Project) are
// returned completely untouched, never copied or dereferenced — an
// earlier version of this function walked into every pointer/struct
// result generically and broke type assertions + proto's own no-copy
// contract for results that were never nil-slice cases at all (found via
// the full wscompat test suite, not guessed — see TASK-010's Status
// note). Only a bare top-level slice (projectGroup.list/ssh.listTargets/
// team.list's shape) or a plain, non-proto VALUE struct
// (credentials.list's credentialsListView) are normalized; a pointer
// result is never unwrapped.
func normalizeNilSlices(v any) any {
	if v == nil {
		return v
	}
	if _, isProto := v.(proto.Message); isProto {
		return v
	}
	rv := reflect.ValueOf(v)
	switch rv.Kind() {
	case reflect.Slice:
		if rv.IsNil() {
			return reflect.MakeSlice(rv.Type(), 0, 0).Interface()
		}
		return v
	case reflect.Struct:
		return normalizeStructSliceFields(rv)
	default:
		return v
	}
}

func normalizeStructSliceFields(rv reflect.Value) any {
	t := rv.Type()
	out := reflect.New(t).Elem()
	out.Set(rv)
	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		if !field.IsExported() {
			continue
		}
		fv := out.Field(i)
		if fv.Kind() == reflect.Slice && fv.IsNil() {
			fv.Set(reflect.MakeSlice(fv.Type(), 0, 0))
		}
	}
	return out.Interface()
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
