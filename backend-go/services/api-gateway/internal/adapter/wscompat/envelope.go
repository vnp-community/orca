// Package wscompat implements the legacy channel-based WebSocket RPC
// protocol frontend/'s WebSocketRpcClient speaks (frontend/src/platform/adapters/web/rpc-client.ts)
// — the same invoke/send/push shape Electron IPC uses, over a WS transport
// at /ws instead of ipcRenderer. This is a compatibility SHIM: it lets the
// existing, unmodified frontend talk to backend-go's real gRPC services by
// translating each named "channel" (the old RPC method name, e.g.
// "task.create") into a call against the appropriate service's generated
// client — see docs/execution-plan.md's "frontend compatibility layer"
// section for the design rationale and per-namespace coverage status.
//
// Wire format (unchanged from the old TS backend, verified against
// rpc-client.ts):
//
//	Client -> Server: {"id":"...","type":"invoke","channel":"task.create","args":[...]}
//	Client -> Server: {"type":"send","channel":"...","data":...}                  (fire-and-forget)
//	Server -> Client: {"type":"result","id":"...","result":...}
//	Server -> Client: {"type":"error","id":"...","message":"..."}
//	Server -> Client: {"type":"push","channel":"...","args":[...]}               (server-initiated)
package wscompat

import "encoding/json"

// InboundMessage is the union of what a client can send — Type
// discriminates which of the other fields are populated, matching
// rpc-client.ts's handleMessage/invoke/send exactly.
//
// Method/Params support a SECOND, distinct wire dialect: frontend/'s
// WebSessionClient (frontend/src/renderer/src/web/web-session-client.ts)
// sends no "type" key at all — WebSessionClient.call()/.subscribe() both do
// `this.send({ id, authToken: 'cookie-auth', method, params })`. Before this
// field pair existed, such a message unmarshalled with Type == "" and hit
// ServeHTTP's `default:` case — the server never responded, so every
// WebSessionClient caller (git.*, repos.*, accounts.*, ...) silently timed
// out after REQUEST_TIMEOUT_MS (see specs/backend-go/bugs/api-v1/BUG-005).
// normalizeInboundMessage (session_dialect.go) recognizes a message with an
// empty Type but a non-empty Method as this dialect and rewrites it onto
// Channel/Args so the rest of the dispatch path (ServeHTTP/handleInvoke/
// Registry.Dispatch) stays dialect-agnostic — only the write-back side
// (handleInvoke/handleSubscribe's dialect parameter) needs to know which
// wire shape to encode the response in.
type InboundMessage struct {
	ID      string            `json:"id,omitempty"`
	Type    string            `json:"type"` // "invoke" | "send"
	Channel string            `json:"channel"`
	Args    []json.RawMessage `json:"args,omitempty"` // present for "invoke"
	Data    json.RawMessage   `json:"data,omitempty"` // present for "send"

	// Method/Params: WebSessionClient's dialect only — see doc comment above.
	Method string          `json:"method,omitempty"`
	Params json.RawMessage `json:"params,omitempty"`
}

// ResultMessage is a successful "invoke" response.
type ResultMessage struct {
	Type   string `json:"type"` // always "result"
	ID     string `json:"id"`
	Result any    `json:"result"`
}

// ErrorMessage is a failed "invoke" response — Message is what
// rpc-client.ts's handleMessage surfaces as the rejected Promise's Error,
// so it should be a plain, user-legible string, not a wrapped Go error.
type ErrorMessage struct {
	Type    string `json:"type"` // always "error"
	ID      string `json:"id"`
	Message string `json:"message"`
}

// PushMessage is a server-initiated event on a channel the client has an
// `on(channel, handler)` listener registered for — written by
// push_bridge.go's pipePush for every StreamHandler-registered channel
// (Registry.RegisterStream), e.g. notifications.subscribe.
type PushMessage struct {
	Type    string `json:"type"` // always "push"
	Channel string `json:"channel"`
	Args    []any  `json:"args"`
}

// sessionClientMeta is the "_meta" envelope field WebSessionClient's wire
// shape requires (frontend/src/shared/runtime-rpc-envelope.ts's
// RuntimeRpcSuccess/RuntimeRpcFailure). WebSessionClient.handleSocketMessage
// never reads _meta.runtimeId — isSubscriptionResponse/isRuntimeFailureResponse
// don't check it, and this call path never goes through
// RuntimeRpcEnvelopeSchema.safeParse (that zod schema only gates
// remote-runtime-client.ts/remote-runtime-request-frames.ts, confirmed by
// grepping every .safeParse call site) — so RuntimeID is a stable
// placeholder, present only to match the documented shape byte-for-byte.
type sessionClientMeta struct {
	RuntimeID *string `json:"runtimeId"`
}

// sessionClientRuntimeID is the placeholder _meta.runtimeId value for a
// successful session-client-dialect response — see sessionClientMeta.
const sessionClientRuntimeID = "backend-go"

// SessionClientResultMessage is a successful response in WebSessionClient's
// dialect (id/ok/result/_meta) — the counterpart of ResultMessage for
// requests that arrived in the session-client dialect (see InboundMessage's
// doc comment and session_dialect.go).
//
// Streaming: set true on every FOLLOW-UP push frame of a subscription
// (Phase 2, push_bridge.go's pipePushForDialect) — omitted (false) on a
// plain call()/subscribe() ack. WebSessionClient.handleSocketMessage's
// isSubscriptionResponse (web-session-client.ts) treats a response as
// subscription data (routed to the subscriber's onResponse callback,
// looked up by request id — never by channel name, unlike the native
// dialect's PushMessage) only if Streaming is true OR Result looks like
// {"type":"end"|"scrollback"} — an ack with neither would instead be
// treated as a one-shot `call()` resolution and silently dropped, since
// subscribe() never registers its request id in WebSessionClient's
// `pending` map. This field's presence is therefore load-bearing, not
// decorative: omitting it on a push frame reproduces the exact silent-drop
// failure BUG-005 was filed for, just for streaming data instead of the
// initial ack.
type SessionClientResultMessage struct {
	ID        string            `json:"id"`
	OK        bool              `json:"ok"` // always true
	Result    any               `json:"result"`
	Meta      sessionClientMeta `json:"_meta"`
	Streaming bool              `json:"streaming,omitempty"`
}

// sessionClientStreamEnd is the Result payload pipePushForDialect sends in
// the final frame of a subscription once its event channel closes —
// WebSessionClient's isEndResult (web-session-client.ts) matches on this
// exact {"type":"end"} shape to fire the subscriber's onClose callback and
// stop tracking the request id, mirroring how a natively-dialected
// subscription simply stops receiving PushMessage frames when the
// connection or channel closes (which WebSessionClient has no equivalent
// signal for — an explicit end frame is required in this dialect).
var sessionClientStreamEnd = map[string]string{"type": "end"}

// sessionClientErrorBody is RuntimeRpcFailure's "error" field shape.
type sessionClientErrorBody struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// SessionClientErrorMessage is a failed response in WebSessionClient's
// dialect — the counterpart of ErrorMessage. _meta.runtimeId is null on
// failure per RuntimeRpcFailure's type (runtimeId: string | null).
type SessionClientErrorMessage struct {
	ID    string                 `json:"id"`
	OK    bool                   `json:"ok"` // always false
	Error sessionClientErrorBody `json:"error"`
	Meta  sessionClientMeta      `json:"_meta"`
}
