# BUG-035: Server→client `push` frames are never sent — `wscompat` is request/response only

**Service:** `api-gateway`
**File:** `internal/adapter/wscompat/handler.go`
**Severity:** High — blocks every real-time/live-update feature that depends on the legacy `/ws` protocol, not just one namespace
**Symptom:** The frontend's `WebSocketRpcClient` can send `invoke`/`send` frames and get `result`/`error` replies, but the server can never spontaneously push a `{type:"push",channel,args}` frame back — anything that used to arrive as a live update instead never arrives at all (no error, no timeout, just silence)
**Status:** ❌ Open

---

## Description

`docs/execution-plan.md` §0 documents the wire protocol `wscompat` implements:

> `{id,type:"invoke",channel,args}` → `{type:"result"|"error",id,...}`,
> plus `{type:"send",channel,data}` (fire-and-forget) and
> `{type:"push",channel,args}` (server→client, **not wired to any event
> source yet**).

That gap is still exactly true today. `handler.go`'s `ServeHTTP` runs a
single loop that only ever reads client frames and dispatches them:

```go
for {
    var msg InboundMessage
    if err := wsjson.Read(ctx, conn, &msg); err != nil {
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
```
(`handler.go:101-116`)

There is no third path for `push`. More importantly, there is **no
mechanism anywhere that could produce one**: the per-connection `conn` and
`identity` are local variables inside `ServeHTTP`'s single invocation
(`handler.go:65-117`) — never registered into any connection registry,
map, or pub/sub subscriber list that something else in the process could
look up later and write to. Confirmed via search:

```
$ grep -rn "\"push\"" backend-go/services/api-gateway --include="*.go"
(no matches outside envelope type definitions/tests)
```

A real internal event bus does exist —
`backend-go/common/eventbus/eventbus.go` (NATS JetStream, outbox-pattern
publish per `specs/backend-go/tdd/architecture/08-inter-service-communication.md`)
— but nothing in `api-gateway` subscribes to it and forwards events onto
any live `/ws` connection. The only place `eventbus` is even mentioned in
`services/api-gateway` is a doc comment in `trace_routes.go` calling out
"wiring real event forwarding... tracked as a follow-up, not attempted
here" for the unrelated `/api/trace-stream` SSE endpoint.

---

## Why this matters beyond "one more missing channel"

Every other report in this directory documents a **client-initiated**
capability (a `callRuntimeRpc(...)` call with no reply). This bug is
different in kind: it's the **transport primitive** several real features
depend on to work at all, regardless of whether their own RPC methods get
wired:

- `notifications.subscribe`/`unsubscribe` and `runtime.clientEvents.subscribe`/`unsubscribe`
  (per `specs/frontend/api/backend-agent-execution-boundary.md`'s
  "Browser/computer/UI-adjacent" table) are pure pub/sub in the old
  backend — their entire purpose is receiving push frames after
  subscribing. There is no `.*` namespace bug report for these two in this
  directory because they don't appear as `callRuntimeRpc` call sites in
  `rpc-catalog.md`'s methodology (client-invoke only) — but they are real
  functionality this transport gap blocks, invisible to that doc's
  scanning approach. Worth a dedicated follow-up audit of push-only
  features, which this pass's methodology structurally can't see.
- Any UI that expects live updates without polling (e.g. a task's status
  changing while the task page is open, a git status refresh after a
  background operation, an automation run completing) degrades silently —
  no error is raised, the UI just never updates until the user manually
  refreshes/reopens, which is a materially worse failure mode than a clean
  `notImplementedHandler` error.

Note there IS a real, working server→client stream elsewhere —
`internal/adapter/wsbridge` serves `GET /v1/notifications/stream`, a
**different**, purpose-built WS endpoint that proxies exactly one gRPC
server-streaming call (`notification-service.StreamNotifications`). It's
not reusable as-is for general push: it's a dedicated connection per
stream, not a multiplexed push channel over the same `/ws` connection
`wscompat` already holds open per authenticated user, and the legacy
frontend protocol this bug is about doesn't know to connect to it for
anything other than notifications.

---

## What implementing this needs, roughly

1. A connection registry keyed by `identity.UserID`/`TenantID` (or a
   pub/sub subscription model) that `ServeHTTP` registers into on connect
   and deregisters on disconnect — so something outside the per-connection
   goroutine can find and write to a live connection.
2. Something to actually publish onto it — either bridging
   `common/eventbus` subjects into `push` frames for the relevant channels,
   or a simpler in-process fan-out for state that doesn't need
   cross-replica delivery (mirroring the old backend's "in-memory WS event
   fan-out" description of `notifications.subscribe`).
3. Careful reuse of `writeMu` (`handler.go:99`) — any push writer must
   serialize through the same per-connection write lock `handleInvoke`
   already uses, or interleaved writes will corrupt the WS frame stream.

---

## References

- `backend-go/docs/execution-plan.md` §0 — "`{type:"push",...}` (server→client, not wired to any event source yet)"
- `backend-go/services/api-gateway/internal/adapter/wscompat/handler.go:101-185` — `ServeHTTP`/`handleInvoke`/`handleSend` (no push path, no connection registry)
- `backend-go/common/eventbus/eventbus.go` — the internal event bus that exists but isn't bridged to `/ws`
- `backend-go/services/api-gateway/internal/adapter/httpgateway/trace_routes.go:24-30` — doc comment flagging the same "no real event forwarding yet" gap for a different endpoint
- `backend-go/services/api-gateway/internal/adapter/wsbridge/handler.go` — the one real (but narrowly-scoped) server-streaming precedent
- `specs/frontend/api/backend-agent-execution-boundary.md` — `notifications.subscribe`/`unsubscribe`, `runtime.clientEvents.subscribe`/`unsubscribe` rows (pub/sub features this blocks)
