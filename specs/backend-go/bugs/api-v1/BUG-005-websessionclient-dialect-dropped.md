# BUG-005: `WebSessionClient`'s method/params dialect silently dropped — every real web-mode data call times out

**Service:** `api-gateway`
**File:** `internal/adapter/wscompat/handler.go`, `internal/adapter/wscompat/envelope.go`
**Severity:** Critical — every `git.*`/`repos.*`/`worktrees.*`/`accounts.*`/... call made by the deployed web frontend silently times out after 30s
**Symptom:** Every RPC call from the shipped web frontend (any feature routed through `WebSessionClient`) hangs for exactly `REQUEST_TIMEOUT_MS` (30s) and then rejects with `Error: Request timed out: <method>` — no server-side error is ever logged, because the server never even recognizes the message.
**Status:** ✅ Fixed (2026-08-26) — [SOL-005](./solutions/SOL-005-websessionclient-dialect-bridge.md)

---

## Description

`deploy/dev/docker-compose.yml` deploys ONLY backend-go services — there is no
old-TS-monolith `backend` service in this stack (confirmed by grepping the
compose file: no such service exists). `deploy/dev/docker/nginx/orca.conf`
proxies `/ws` straight to `api-gateway`. So in this real, deployed stack,
`backend-go/services/api-gateway/internal/adapter/wscompat/handler.go`'s
`Handler.ServeHTTP` is genuinely what answers every `/ws` connection.

The shipped web frontend's real data-call client,
`frontend/src/renderer/src/web/web-session-client.ts`'s `WebSessionClient`,
sends every request as:

```json
{"id": "web-session-rpc-1-...", "authToken": "cookie-auth", "method": "git.status", "params": {...}}
```

(`WebSessionClient.call()` and `.subscribe()` both call
`this.send({ id, authToken: 'cookie-auth', method, params })`) — note: **no
`"type"` field at all.**

Before this fix, `envelope.go`'s `InboundMessage` was:

```go
type InboundMessage struct {
	ID      string            `json:"id,omitempty"`
	Type    string            `json:"type"` // "invoke" | "send"
	Channel string            `json:"channel"`
	Args    []json.RawMessage `json:"args,omitempty"`
	Data    json.RawMessage   `json:"data,omitempty"`
}
```

and `handler.go`'s `ServeHTTP` read loop did:

```go
var msg InboundMessage
json.Unmarshal(data, &msg)
switch msg.Type {
case "invoke": ...
case "send": ...
default:
	h.Logger.WarnContext(ctx, "wscompat: unknown message type", slog.String("type", msg.Type))
}
```

A `WebSessionClient` message has no `"type"` key, so `msg.Type` unmarshals to
`""`, hits `default:`, gets logged as `unknown message type`, and **the
server never responds**. The client's `pending` map entry for that request
id is only cleaned up by its own 30s client-side timer
(`REQUEST_TIMEOUT_MS`, `web-session-client.ts`), which then rejects with
`Error: Request timed out: <method>`.

This affects **every** feature that goes through `WebSessionClient.call()`
or `.subscribe()` — i.e. every real feature in the deployed web app:
`git.*`, `repos.*`, `worktrees.*`, `accounts.*`, and every other namespace
registered in `Registry`.

## Root Cause

Two independent, both-necessary conditions:

1. **Envelope mismatch.** `wscompat` only ever spoke one dialect —
   `rpc-client.ts`'s Electron-IPC-shaped `{"type":"invoke"|"send",...}`
   wire format (see `envelope.go`'s package doc comment). `WebSessionClient`
   speaks a second, structurally different dialect
   (`{"id","authToken","method","params"}`) that was never accounted for.
2. **No legacy fallback in this deployed stack.** The old TS backend's
   `WsSessionRouter` (`backend/src/main/session/ws-session-router.ts`) DOES
   understand this exact `authToken: 'cookie-auth'` substitution
   (`ws.on('message', ...)` rewrites `parsed.authToken` before forwarding to
   the per-user process) — but `WsSessionRouter` is part of the old
   Electron-multi-user backend, which `deploy/dev/docker-compose.yml` does
   not deploy at all. There is no service in this stack that already speaks
   `WebSessionClient`'s dialect.

## Confirmation

Read directly (not guessed) as of this fix:

- `deploy/dev/docker-compose.yml` — grepped for any `backend`/TS-monolith
  service; none exists, only backend-go services.
- `deploy/dev/docker/nginx/orca.conf` — `/ws` proxies to `api-gateway`.
- `frontend/src/renderer/src/web/web-session-client.ts` —
  `WebSessionClient.call()`/`.subscribe()`, both `this.send({ id, authToken:
  'cookie-auth', method, params })`.
- `backend-go/services/api-gateway/internal/adapter/wscompat/envelope.go` —
  `InboundMessage` (pre-fix, no `Method`/`Params` fields).
- `backend-go/services/api-gateway/internal/adapter/wscompat/handler.go` —
  `ServeHTTP`'s `switch msg.Type { ... default: ... }`.
- `frontend/src/shared/runtime-rpc-envelope.ts` — the `RuntimeRpcResponse`
  shape `WebSessionClient` expects back.
- `backend/src/main/session/ws-session-router.ts` — confirmed this is the
  ONLY place in the codebase that already knew about
  `authToken: 'cookie-auth'`, and confirmed it's not part of this deployed
  stack.

---

## Fix

See [SOL-005](./solutions/SOL-005-websessionclient-dialect-bridge.md) for
the full implementation. Summary: `envelope.go` gained `Method`/`Params`
fields on `InboundMessage` plus new `SessionClientResultMessage`/
`SessionClientErrorMessage` response types; a new `session_dialect.go`
detects the dialect once per message and normalizes it onto the existing
`Channel`/`Args` shape so `Registry.Dispatch`/`DispatchStreamChannel`/
`BinaryStreamHandlerFor` need no changes; the write-back side encodes the
response in whichever dialect the request arrived in.

**Explicitly out of scope for this fix (Phase 2):** `pipePush`'s follow-up
push events for `StreamHandler`-registered channels (e.g.
`accounts.subscribe`) still use the native `{"type":"push",...}` shape,
which `WebSessionClient` cannot correlate to a request id. A session-client
caller of a subscribe-shaped channel gets a correct initial ack and then no
further updates — a real, deliberate, documented gap. See SOL-005's
"Phase 2" section.

---

## Files Changed

| File | Change |
|------|--------|
| `internal/adapter/wscompat/envelope.go` | Add `Method`/`Params` to `InboundMessage`; add `SessionClientResultMessage`/`SessionClientErrorMessage`/`sessionClientMeta` |
| `internal/adapter/wscompat/session_dialect.go` (new) | `dialect` enum, `normalizeInboundMessage`, `writeDialectResult`/`writeDialectError` |
| `internal/adapter/wscompat/handler.go` | `ServeHTTP` calls `normalizeInboundMessage` once; `handleInvoke`/`handleSubscribe` take a `dialect` param and use `writeDialectResult`/`writeDialectError` instead of a hardcoded `ResultMessage`/`ErrorMessage` |
| `internal/adapter/wscompat/handler_test.go` | 5 new tests (native-dialect regression guard, session-client round-trip, session-client error path, garbage-message graceful handling, subscribe-channel ack-without-crash) |
