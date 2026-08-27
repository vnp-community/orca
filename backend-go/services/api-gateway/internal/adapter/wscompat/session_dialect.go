package wscompat

import (
	"context"
	"encoding/json"

	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"
)

// dialect identifies which wire format an InboundMessage arrived in, so the
// write-back side (handleInvoke/handleSubscribe) knows which response shape
// to encode. Decided ONCE, in normalizeInboundMessage, and threaded down as
// a plain parameter — not re-detected at write time, since by then msg has
// already been rewritten onto the native Channel/Args shape and the
// distinguishing "no type but has method" signal is gone.
type dialect int

const (
	// dialectNative is today's rpc-client.ts wire format:
	// {"id","type":"invoke"|"send","channel","args"|"data"} in, {"type":
	// "result"|"error",...} out. Unchanged by this file.
	dialectNative dialect = iota
	// dialectSessionClient is WebSessionClient's wire format: no "type" key,
	// {"id","authToken","method","params"} in, RuntimeRpcResponse-shaped
	// {"id","ok","result"|"error","_meta"} out. See InboundMessage's doc
	// comment in envelope.go and specs/backend-go/bugs/api-v1/BUG-005.
	dialectSessionClient
)

// normalizeInboundMessage detects WebSessionClient's dialect (empty Type,
// non-empty Method) and rewrites msg onto the SAME internal shape the
// native "invoke" dialect already uses — Channel from Method, Args wrapping
// Params as the single positional arg (matches how every channels_*.go
// handler already calls decodeArg[T](args, 0) against one object-shaped
// param, e.g. channels_accounts.go's accountsRelayArgs). This lets
// ServeHTTP/handleInvoke/handleSubscribe/Registry.Dispatch stay fully
// dialect-agnostic; only the write-back side needs the returned dialect.
//
// Phase 1 scope only: this bridges the plain call()/invoke request/response
// shape. WebSessionClient never sends a "send"-shaped (fire-and-forget)
// message — call()/subscribe() both always wait for a response — so a
// native msg.Type == "send" is passed through unchanged here and
// handleSend needs no dialect awareness at all.
//
// A message with neither Type nor Method (garbage/malformed) is also passed
// through unchanged as dialectNative — ServeHTTP's existing `default:` case
// still logs it and keeps the connection open, matching pre-bridge
// behavior exactly.
func normalizeInboundMessage(msg InboundMessage) (dialect, InboundMessage) {
	if msg.Type != "" || msg.Method == "" {
		return dialectNative, msg
	}
	msg.Type = "invoke"
	msg.Channel = msg.Method
	if msg.Params != nil {
		msg.Args = []json.RawMessage{msg.Params}
	}
	return dialectSessionClient, msg
}

// writeDialectResult encodes and writes a successful dispatch result in d's
// wire shape — ResultMessage for dialectNative (unchanged), or
// SessionClientResultMessage for dialectSessionClient (see envelope.go).
func writeDialectResult(ctx context.Context, conn *websocket.Conn, d dialect, id string, result any) error {
	if d == dialectSessionClient {
		runtimeID := sessionClientRuntimeID
		return wsjson.Write(ctx, conn, SessionClientResultMessage{
			ID:     id,
			OK:     true,
			Result: result,
			Meta:   sessionClientMeta{RuntimeID: &runtimeID},
		})
	}
	return wsjson.Write(ctx, conn, ResultMessage{Type: "result", ID: id, Result: result})
}

// writeDialectError encodes and writes a failed dispatch result in d's wire
// shape — ErrorMessage for dialectNative (unchanged), or
// SessionClientErrorMessage for dialectSessionClient. "internal" is a
// generic, stable error code: channel handlers in this package return plain
// Go errors (fmt.Errorf, wrapped grpc-status errors), not a typed code the
// session-client dialect could forward instead — see this function's BUG-005
// solution doc for why that's an acceptable Phase 1 simplification.
func writeDialectError(ctx context.Context, conn *websocket.Conn, d dialect, id string, err error) error {
	if d == dialectSessionClient {
		return wsjson.Write(ctx, conn, SessionClientErrorMessage{
			ID:    id,
			OK:    false,
			Error: sessionClientErrorBody{Code: "internal", Message: err.Error()},
			Meta:  sessionClientMeta{RuntimeID: nil},
		})
	}
	return wsjson.Write(ctx, conn, ErrorMessage{Type: "error", ID: id, Message: err.Error()})
}
