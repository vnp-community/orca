# SOL-005: Fix BUG-005 — bridge `WebSessionClient`'s method/params dialect onto the invoke/result path

**Resolves:** BUG-005
**Service:** `api-gateway`
**Affected files:** `internal/adapter/wscompat/envelope.go`, `internal/adapter/wscompat/handler.go`, `internal/adapter/wscompat/session_dialect.go` (new)
**Priority:** Critical — root cause of every real web-mode data call timing out
**Status:** ✅ IMPLEMENTED (2026-08-26)

---

## Scope — Phase 1 only

This fix bridges `WebSessionClient`'s plain `call()`/`subscribe()`-initiated
request/response path onto `wscompat`'s existing invoke/result dispatch.
**Phase 2 (streaming/subscribe push bridge) is explicitly NOT implemented**
— see "Phase 2" section below.

---

## Design

`WebSessionClient.call(method, params)` is functionally identical to
`wscompat`'s own `{type:"invoke", channel, args}` dispatch — same one-shot
request/response shape, just a different envelope. So instead of adding a
parallel dispatch path, this fix normalizes the session-client dialect onto
the SAME `Channel`/`Args` shape `Registry.Dispatch` already expects, decided
once, at the top of the read loop.

### 1. `envelope.go` — recognize the dialect on the wire

```go
type InboundMessage struct {
	ID      string            `json:"id,omitempty"`
	Type    string            `json:"type"` // "invoke" | "send"
	Channel string            `json:"channel"`
	Args    []json.RawMessage `json:"args,omitempty"`
	Data    json.RawMessage   `json:"data,omitempty"`

	// Method/Params: WebSessionClient's dialect only.
	Method string          `json:"method,omitempty"`
	Params json.RawMessage `json:"params,omitempty"`
}
```

Plus new response types matching
`frontend/src/shared/runtime-rpc-envelope.ts`'s `RuntimeRpcSuccess`/
`RuntimeRpcFailure` exactly:

```go
type SessionClientResultMessage struct {
	ID     string            `json:"id"`
	OK     bool              `json:"ok"` // always true
	Result any               `json:"result"`
	Meta   sessionClientMeta `json:"_meta"`
}

type SessionClientErrorMessage struct {
	ID    string                 `json:"id"`
	OK    bool                   `json:"ok"` // always false
	Error sessionClientErrorBody `json:"error"`
	Meta  sessionClientMeta      `json:"_meta"`
}

type sessionClientMeta struct {
	RuntimeID *string `json:"runtimeId"`
}
```

### 2. `session_dialect.go` (new) — detect once, normalize, write back

```go
type dialect int

const (
	dialectNative dialect = iota
	dialectSessionClient
)

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
```

A message with an empty `Type` AND an empty `Method` (garbage) still returns
`dialectNative` unchanged — `ServeHTTP`'s existing `default:` case still
logs and keeps the connection open, exactly as before this fix.

`writeDialectResult`/`writeDialectError` encode the response in whichever
dialect the request arrived in:

- `dialectNative` → today's `ResultMessage`/`ErrorMessage` (byte-for-byte
  unchanged — verified by `TestNativeInvokeDialect_UnchangedByDialectBridge`).
- `dialectSessionClient` → `SessionClientResultMessage`
  (`_meta.runtimeId: "backend-go"`) on success, `SessionClientErrorMessage`
  (`_meta.runtimeId: null`, `error.code: "internal"`) on failure.

### 3. `handler.go` — thread the dialect through, once

`ServeHTTP`'s read loop calls `normalizeInboundMessage` exactly once per
message, then dispatches through the **unchanged**
`Registry.Dispatch`/`DispatchStreamChannel`/`BinaryStreamHandlerFor`
machinery — `handleInvoke` and `handleSubscribe` each gained one new
`dialect` parameter, used only at the two write-back call sites (nothing in
the dispatch logic itself needed to change):

```go
msgDialect, msg := normalizeInboundMessage(msg)

switch msg.Type {
case "invoke":
	if sh, ok := h.Registry.StreamHandlerFor(msg.Channel); ok {
		go h.handleSubscribe(ctx, conn, &writeMu, identity, msg, sh, msgDialect)
		continue
	}
	go h.handleInvoke(ctx, conn, &writeMu, identity, msg, msgDialect)
case "send":
	go h.handleSend(ctx, identity, msg)
default:
	h.Logger.WarnContext(ctx, "wscompat: unknown message type", slog.String("type", msg.Type))
}
```

`handleInvoke`'s write-back:

```go
if err != nil {
	_ = writeDialectError(writeCtx, conn, msgDialect, msg.ID, err)
	...
	return
}
_ = writeDialectResult(writeCtx, conn, msgDialect, msg.ID, result)
```

`handleSend` is untouched: `WebSessionClient` has no fire-and-forget
concept (`call()`/`subscribe()` both always wait for a response), so
`normalizeInboundMessage` never produces `msg.Type == "send"` for the
session-client dialect — confirmed and documented in `handleSend`'s own doc
comment rather than adding dead branching.

---

## `_meta`/zod-schema finding

`frontend/src/shared/runtime-rpc-envelope.ts` exports both the
`RuntimeRpcEnvelopeSchema` zod schema AND the plain `RuntimeRpcResponse`
TypeScript type. Grepping every `RuntimeRpcEnvelopeSchema.safeParse` call
site across the frontend:

- `frontend/src/shared/remote-runtime-client.ts` (2 call sites)
- `frontend/src/shared/remote-runtime-request-frames.ts` (1 call site)

**`web-session-client.ts` is NOT among them.**
`WebSessionClient.handleSocketMessage` does a bare `JSON.parse` and then
manual, unvalidated shape checks (`'id' in response`,
`isRuntimeFailureResponse`, `isSubscriptionResponse`) — none of which read
`_meta` at all. So for this specific call path, nothing gates on `_meta` or
validates through the zod schema; a well-formed literal object matching the
TypeScript type is sufficient, which is what this fix produces
(`_meta.runtimeId: "backend-go"` on success, `null` on failure — matching
`RuntimeRpcSuccess`/`RuntimeRpcFailure`'s declared shapes exactly).

---

## Tests Added — `handler_test.go`

All in the existing `handler_test.go` (this package's established
convention — no new test file), reusing `dialTestClient`/`newTestHandlerServer`
from `push_bridge_test.go`/this file:

| Test | Verifies |
|------|----------|
| `TestNativeInvokeDialect_UnchangedByDialectBridge` | (a) regression guard — native `{"type":"invoke",...}` still gets `{"type":"result",...}`, no `_meta` |
| `TestSessionClientDialect_RoundTripsThroughRegisteredChannel` | (b) `{"id","authToken","method","params"}` round-trips through a real registered channel, comes back `{"id","ok":true,"result","_meta":{"runtimeId":"backend-go"}}` |
| `TestSessionClientDialect_ErrorPathReturnsOkFalse` | (c) unregistered channel via session-client dialect returns `{"id","ok":false,"error":{"code","message"},"_meta":{"runtimeId":null}}` |
| `TestGarbageMessage_NeitherTypeNorMethod_HandledGracefully` | (d) a message with neither `type` nor `method` is logged, connection stays open, no panic — a subsequent ordinary invoke on the same connection still works |
| `TestSessionClientDialect_SubscribeChannelAcksWithoutCrashing` | Point 6 — a session-client request on a `StreamHandler`-registered channel gets a correct ack and a follow-up push event doesn't panic/hang the connection (Phase 2 gap, not a regression) |

`writeRaw`/`readRawFrame`/`readSessionClientWireMessage` were added as small
test helpers — `InboundMessage.Type` lacks `omitempty`, so marshaling it via
`wsjson.Write` always emits `"type":""`, which is NOT what a real
`WebSessionClient` sends; the session-client-dialect tests write literal
raw JSON bytes instead to test the real wire shape.

### Verification

```
cd backend-go/services/api-gateway
go build ./...   # clean
go vet ./...     # clean
go test ./...    # ok, all packages, including the 5 new + all pre-existing wscompat tests
```

---

## Phase 2 — explicitly NOT implemented here

`pipePush` (`push_bridge.go`) is untouched: follow-up push frames for
`StreamHandler`-registered channels (e.g. `accounts.subscribe`,
`notifications.subscribe`) always use the native
`{"type":"push","channel","args"}` shape, regardless of which dialect the
subscribing request arrived in.

`WebSessionClient` has no channel-keyed push concept — its
`handleSocketMessage` only correlates follow-up frames by the original
request `id` via `isSubscriptionResponse()`'s `streaming`/`result.type ===
'end'` checks. A session-client caller of a subscribe-shaped channel today
gets:

1. A correct, dialect-aware initial ack (this fix).
2. Silence after that — the native push frames it can't parse into its
   `pending`/`subscriptions` maps are simply frames it doesn't recognize
   (see `handleSocketMessage`'s `if (!('id' in response) ...) return`
   guard — a push frame has no top-level `id`, so it's dropped harmlessly,
   not mis-delivered).

This is strictly better than the pre-fix total 30s timeout (the initial
call now resolves), but is NOT a working subscription bridge. Implementing
that requires either (a) teaching `wscompat` to also emit an id-correlated
streaming frame shape for session-client-dialect subscribers, or (b)
teaching `WebSessionClient` to understand channel-keyed push — both out of
scope for this fix. Tracked as future work, not filed as a separate BUG
number since it's a known, deliberate scope boundary of this same fix
rather than an independently-discovered gap.
