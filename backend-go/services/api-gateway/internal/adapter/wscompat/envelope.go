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
type InboundMessage struct {
	ID      string            `json:"id,omitempty"`
	Type    string            `json:"type"` // "invoke" | "send"
	Channel string            `json:"channel"`
	Args    []json.RawMessage `json:"args,omitempty"` // present for "invoke"
	Data    json.RawMessage   `json:"data,omitempty"` // present for "send"
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
// `on(channel, handler)` listener registered for. Not yet wired to any
// backend-go event source in this pass — see registry.go's doc comment.
type PushMessage struct {
	Type    string `json:"type"` // always "push"
	Channel string `json:"channel"`
	Args    []any  `json:"args"`
}
