# SOL-005: Fix BUG-005 — bridge `WebSessionClient`'s method/params dialect onto the invoke/result path

**Resolves:** BUG-005
**Service:** `api-gateway`
**Affected files:** `internal/adapter/wscompat/envelope.go`, `internal/adapter/wscompat/handler.go`, `internal/adapter/wscompat/session_dialect.go` (new), `internal/adapter/wscompat/push_bridge.go` (Phase 2)
**Priority:** Critical — root cause of every real web-mode data call timing out
**Status:** ✅ IMPLEMENTED — Phase 1 (2026-08-26), Phase 2 (2026-08-27)

---

## Scope

Phase 1 bridges `WebSessionClient`'s plain `call()`/`subscribe()`-initiated
request/response (ack) path onto `wscompat`'s existing invoke/result
dispatch. Phase 2 additionally bridges the FOLLOW-UP push events a
subscription produces after its initial ack — see "Phase 2" section below
for the full wire contract.

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
| `TestSessionClientDialect_SubscribeChannelStreamsAndEnds` (Phase 2; replaces the original `...AcksWithoutCrashing`) | `StreamHandler` path: ack, a real `Streaming:true` update keyed by the request id, and a final `{"type":"end"}` frame on channel close — the original test only asserted a non-crashing native push frame; Phase 2 makes it a real, working bridge |
| `TestSessionClientDialect_StreamChannelAckAndPushBothBridge` (Phase 2, new) | `StreamChannelHandler` path (ack+events from one call, e.g. `terminal.create`'s shape) — both the ack and its follow-up push go through the dialect bridge |

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

## Phase 2 — implemented (2026-08-27)

Originally scoped out of the initial fix; implemented as a follow-up once
Phase 1 landed and was verified. `pipePush` (`push_bridge.go`) itself is
UNCHANGED (still the native dialect's exact behavior) — a new sibling
function, `pipePushForDialect`, wraps it:

```go
func pipePushForDialect(ctx context.Context, conn *websocket.Conn, writeMu *sync.Mutex, events <-chan PushEvent, d dialect, requestID string) {
	if d != dialectSessionClient {
		pipePush(ctx, conn, writeMu, events) // native dialect: delegate, unchanged
		return
	}
	for {
		select {
		case <-ctx.Done():
			return // connection going away — no point writing an end frame
		case ev, ok := <-events:
			if !ok {
				// event channel closed: one final {"type":"end"} frame so
				// WebSessionClient's isEndResult fires onClose.
				write(SessionClientResultMessage{ID: requestID, OK: true, Result: sessionClientStreamEnd, ...})
				return
			}
			// ongoing update: Streaming:true, keyed by requestID (never ev.Channel).
			write(SessionClientResultMessage{ID: requestID, OK: true, Result: pushEventResult(ev), Streaming: true, ...})
		}
	}
}
```

`handler.go`'s two `pipePush(...)` call sites (`handleInvoke`'s
`StreamChannelHandler` ack+events path, e.g. `terminal.create`, and
`handleSubscribe`'s pure `StreamHandler` path, e.g.
`notifications.subscribe`) both now call `pipePushForDialect(..., msgDialect,
msg.ID)` instead — the native dialect's behavior is byte-for-byte identical
to before (delegates straight to `pipePush`), confirmed by
`TestNativeInvokeDialect_UnchangedByDialectBridge` still passing unchanged.

**Wire contract for session-client-dialect subscribers**, now real:

1. Initial ack: `{"id","ok":true,"result":<ack value, or null for a pure
   StreamHandler>,"_meta"}` — unchanged from Phase 1.
2. Every event: `{"id"` (same id as the ack) `,"ok":true,"result":<value>,
   "_meta","streaming":true}` — `WebSessionClient.isSubscriptionResponse`
   requires `streaming===true` (or a `result.type` of `"end"`/`"scrollback"`)
   to route a same-id frame to the subscriber's `onResponse` instead of
   silently dropping it (see `handleSocketMessage`), so `Streaming` is
   load-bearing, not decorative.
3. Stream end: `{"id","ok":true,"result":{"type":"end"},"_meta"}` (no
   `streaming` field) once the handler's event channel closes —
   `WebSessionClient.isEndResult` matches this shape, deletes the
   subscription, and fires `callbacks.onClose`.

**`pushEventResult`'s single-arg unwrap:** `PushEvent.Args` is a slice
because the native dialect spreads it as positional arguments
(`handlers.forEach(h => h(...args))`, `rpc-client.ts`) — a concept
`WebSessionClient` has no equivalent for (`onResponse` takes one
`response.result` value). Every current real `StreamHandler`
(`registerNotificationStreamChannel`, `registerClientEventsChannel`,
`channels_push.go`) emits exactly one arg per event, so `pushEventResult`
unwraps a single-element `Args` to its bare value rather than forcing every
session-client consumer to index into a one-element array; a hypothetical
future multi-arg emitter degrades safely to the raw slice instead of
crashing or silently dropping data.

**Remaining caveat, not a gap in this fix:** this bridges the WS-transport
push mechanism generically — it does not, by itself, make any specific
subscribe-shaped feature (e.g. `accounts.subscribe`, `nativeChat.subscribe`)
work end-to-end, since most of those channels aren't registered in
`wscompat`'s `Registry` at all yet (see TASK-023's finding for
`accounts.subscribe` specifically). Today's only real, registered
`StreamHandler`/`StreamChannelHandler` channels are
`notifications.subscribe`, `runtime.clientEvents.subscribe`
(`channels_push.go`), and `terminal.create` (`channels_terminal.go`) —
those three now have a real, tested path to a session-client subscriber;
any NEW subscribe-shaped channel registered in the future gets this bridge
for free, with no per-channel work required.

### Tests Added — Phase 2

| Test | Verifies |
|------|----------|
| `TestSessionClientDialect_SubscribeChannelStreamsAndEnds` (replaces the old `...AcksWithoutCrashing`) | `StreamHandler` path: ack, one `Streaming:true` update keyed by the request id (not `ev.Channel`), and a final `{"type":"end"}` frame on channel close |
| `TestSessionClientDialect_StreamChannelAckAndPushBothBridge` | `StreamChannelHandler` path (ack+events from one call, e.g. `terminal.create`'s shape): both the ack and its follow-up push go through the dialect bridge |

```bash
cd backend-go/services/api-gateway
go build ./...   # clean
go vet ./...     # clean
go test ./...    # ok, all packages, including both new Phase 2 tests + all Phase 1/pre-existing wscompat tests
```
