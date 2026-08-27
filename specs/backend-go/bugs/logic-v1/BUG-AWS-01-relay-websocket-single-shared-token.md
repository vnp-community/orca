# BUG-AWS-01: relay-websocket mode works end-to-end but uses one deployment-wide shared token instead of per-DevServer tokens

**Business Logic:** [BL-AWS-01](../../../../docs/logic/agent-ws/BL-AWS-01-relay-websocket.md) — relay-websocket Mode (Orca → Agent)
**Priority (per spec):** P1
**Status:** PARTIAL
**Severity:** Medium
**Symptom:** An operator running multiple Dev Servers, each meant to trust a distinct agent token (per the spec's `DevServer.agentToken` field), finds every relay-websocket Dev Server in the deployment accepts (and requires) the exact same bearer token — set once via the `ORCA_AGENT_TOKEN` environment variable on api-gateway/infra-fleet-service. There is no way to scope, rotate, or revoke a token for a single Dev Server without changing the value for all of them and restarting the service.

---

## Spec summary

BL-AWS-01 describes Orca dialing out to an agent-run WebSocket server (`ws://agent:6799/orca-relay`), authenticating with `Authorization: Bearer <agentToken>` where `agentToken` is a per-`DevServer` value (SHA-256-hashed, stored on that DevServer's own config record), over a 13-byte-header binary frame protocol carrying JSON-RPC.

## What backend-go has

The core relay-websocket transport is real and matches the wire protocol closely:
- `backend-go/services/infra-fleet-service/internal/adapter/devserveragent/client.go:132-162` (`getOrCreateSession`/`getOrDialSession`) — dials out to the DevServer's `Host` over WebSocket, lazily, with reconnect-on-drop (session.go's `backgroundReconnect`, referenced in client.go:83-88).
- `backend-go/services/infra-fleet-service/internal/adapter/devserveragent/frame.go:17-51` — `HeaderLength = 13`, `[TYPE u8][ID u32BE][ACK u32BE][LENGTH u32BE]` — byte-identical to the spec's `[TYPE:1B][SEQ:4B][ACK:4B][LEN:4B]` framing.
- `backend-go/services/infra-fleet-service/internal/adapter/devserveragent/session.go:112,120` — sends `Authorization: Bearer <cfg.Token>` on dial, matching the spec's header.
- `backend-go/services/infra-fleet-service/internal/domain/dev_server.go:16-18` — `ConnectionModeRelayWebSocket` is a real, validated enum value on `DevServer`.

## What's missing

- **No per-DevServer token.** `domain.DevServer` (`dev_server.go:53-59`) has exactly 5 fields — `ID, TenantID, Host, Mode, SSHTargetID` — no `agentToken`/`hash` field at all. The token instead lives in `devserveragent.Config.Token`, populated once at process start from the `ORCA_AGENT_TOKEN` env var (`config.go:86`, `LoadConfigFromEnv`) and reused for every relay-websocket DevServer the process ever dials (`config.go:16-24`'s own doc comment: "a static, operator-set, long-lived shared secret ... deployment-wide, not per-dev-server").
- No hashing/storage of the token anywhere (spec: `SHA-256(token)` stored per DevServer) — there is nothing to hash since there is only one process-wide plaintext value.
- No revoke path: since the token isn't tied to a DevServer record, there is no way to invalidate one Dev Server's access without changing the env var for the whole deployment (which invalidates every other Dev Server too) and restarting the process.
- No `relayWebsocketUrl` field distinct from `Host` — `Host` doubles as the WS URL (e.g. `"ws://devserver.local:6799"` per `channels_test.go:79`), which is functionally equivalent but not a documented, separately-named field.

## See also

- The transport this bug's token gap sits on top of is itself only reachable for the two `git.*` methods currently wired (`git.status`/`git.diff`), and even those don't reach a real registered handler on the agent for relay-websocket connections — see [BUG-036](../missing-v1/BUG-036-git-relay-methods-unreachable-on-agent.md). This bug (AWS-01) is about the transport/auth layer being architecturally simplified relative to the spec; BUG-036 is about the one real caller (git-gateway-service) hitting a dead end past that transport.

## References

- `backend-go/services/infra-fleet-service/internal/adapter/devserveragent/client.go:1-47,132-162` — package doc comment, `getOrCreateSession`/`getOrDialSession`
- `backend-go/services/infra-fleet-service/internal/adapter/devserveragent/config.go:9-33,75-92` — `Config.Token` doc comment admitting the deployment-wide-vs-per-DevServer gap; `LoadConfigFromEnv`
- `backend-go/services/infra-fleet-service/internal/adapter/devserveragent/session.go:112-120` — `Authorization: Bearer` header construction
- `backend-go/services/infra-fleet-service/internal/adapter/devserveragent/frame.go:17-51` — 13-byte frame codec
- `backend-go/services/infra-fleet-service/internal/domain/dev_server.go:53-59` — `DevServer` struct, no token field
- `backend-go/services/api-gateway/internal/adapter/wscompat/channels_test.go:79` — `Host: "ws://devserver.local:6799"` fixture showing `Host` doubling as the relay URL
- `docs/logic/agent-ws/BL-AWS-01-relay-websocket.md` — spec
