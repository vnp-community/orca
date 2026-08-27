# BUG-AWS-02: direct-websocket handshake works but its wire protocol and close-code taxonomy diverge from the documented spec

**Business Logic:** [BL-AWS-02](../../../../docs/logic/agent-ws/BL-AWS-02-direct-websocket.md) — direct-websocket Mode (Agent → Orca)
**Priority (per spec):** P1
**Status:** PARTIAL
**Severity:** Medium
**Symptom:** A third-party agent author who implements the handshake exactly as documented (send a plain-JSON `{"type":"agent.handshake",...}` frame after receiving a `{"type":"handshake-request"}` push from Orca, and expect a WS close with code 4001/4002/4003/4004 on failure) cannot connect: backend-go never sends a `handshake-request` push, requires the agent's very first WS frame to already be a binary 13-byte-framed JSON-RPC `agent.handshake` request, and closes every failure case with generic WebSocket code 1008 rather than the documented 4001-4004 codes.

---

## Spec summary

BL-AWS-02 documents the agent dialing `ws://orca:6768/agent` as a WS client, receiving a `{"type":"handshake-request","version":"1"}` push, replying with `{"type":"agent.handshake", agentToken, name, version}`, and getting back `{"type":"handshake-ok", sessionId}` or a close with one of 4 documented codes (4001 invalid token, 4002 handshake timeout, 4003 protocol version mismatch, 4004 server at capacity).

## What backend-go has

The endpoint, the underlying auth check, and session hand-off are real:
- `backend-go/services/infra-fleet-service/internal/adapter/agentwsserver/server.go:95-106` (`ServeHTTP`) — accepts the WS upgrade at `/agent` (mounted via `backend-go/services/api-gateway/internal/adapter/httpgateway/router.go:112`, proxied through to infra-fleet-service per `agent_proxy_routes.go:9-39`).
- `server.go:120-172` (`handleConnection`) — waits for the agent's first frame, requires it to decode as an `agent.handshake` JSON-RPC request, validates the token against `Registry.Consume` (single-use, SHA-256-hashed — `slots.go:99-111`), and on success calls `Client.AttachInboundSession` (`client.go:224-235`).
- `server.go:191-204` (`acknowledgeHandshake`) — returns `{ok:true, orcaVersion, sessionId}`, functionally equivalent to the spec's `handshake-ok`.

## What's missing

- **No `handshake-request` server push.** The spec's step 2 (`S→C: {"type":"handshake-request","version":"1"}`) does not exist anywhere in `server.go` — `handleConnection` (`server.go:120-128`) goes straight to `conn.Read`, i.e. it expects the agent to speak first, unprompted.
- **Wrong wire format for the handshake itself.** The spec's examples (Python/Go) send/receive plain JSON text messages. Backend-go instead requires the first message to already be a binary, 13-byte-header-framed (`devserveragent.DecodeFrame`, `server.go:130-134`) JSON-RPC request — a plain-JSON WS text message as shown in the spec's own Python example would fail decoding and get rejected as a "Protocol violation" (`server.go:132`).
- **Close codes don't match the documented taxonomy at all.** Every failure path in `server.go` — bad first frame (`:126`), wrong frame type (`:132`), wrong method name (`:138`), and invalid token (`:188`, via `rejectHandshake`) — closes with `websocket.StatusPolicyViolation` (WS code **1008**), never 4001. Grep for `4001`/`4002`/`4003`/`4004` anywhere in `agentwsserver/` or `devserveragent/` returns no matches.
- **No handshake-timeout close code.** `handshakeTimeout = 20 * time.Second` (`server.go:24-27`) does bound the wait via a `context.WithTimeout`, but a timeout just makes `conn.Read` fail like any other bad-frame case, closing with 1008 — never the documented 4002.
- **No protocol-version-mismatch handling (4003).** Nothing in `server.go` inspects an agent-reported protocol version and rejects on mismatch.
- **No capacity-limit handling (4004).** There is no connection-count cap or "server at capacity" rejection path anywhere in the package.

## See also

- The token this handshake validates is a single-use, ephemeral, admin-API-secret-gated token (`agentwsserver.TokenIssuer`/`Registry`) rather than the persistent, named, revocable per-DevServer tokens BL-AWS-03 describes — see [BUG-AWS-03](./BUG-AWS-03-token-management-not-persistent.md) for that gap in detail.

## References

- `backend-go/services/infra-fleet-service/internal/adapter/agentwsserver/server.go:17-35,95-221` — full handshake handler, constants, close-code usage
- `backend-go/services/infra-fleet-service/internal/adapter/agentwsserver/slots.go:99-111` — `Registry.Consume` (single-use token check)
- `backend-go/services/api-gateway/internal/adapter/httpgateway/router.go:112-113` — `/agent`, `/api/agent-token` mounted
- `backend-go/services/api-gateway/internal/adapter/httpgateway/agent_proxy_routes.go:9-39` — raw byte-proxy to infra-fleet-service, no frame interpretation at the gateway
- `docs/logic/agent-ws/BL-AWS-02-direct-websocket.md` — spec (handshake sequence, close-code table)
