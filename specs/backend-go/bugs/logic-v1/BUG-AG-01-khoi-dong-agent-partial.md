# BUG-AG-01: Starting an AI agent only spawns a generic shell PTY — no agent binary, credentials, or session record

**Business Logic:** [BL-AG-01](../../../../docs/logic/agent-orchestration/BL-AG-01-khoi-dong-agent.md) — Khởi động AI Agent
**Priority (per spec):** P0
**Status:** PARTIAL
**Severity:** Critical
**Symptom:** A user clicking "Start Agent" in a worktree has no backend-go path that spawns `claude`/`codex`/etc with the right env, records a resumable session, or reports startup timeout/binary-not-found errors the way the spec describes. The only thing backend-go can spawn on a Dev Server via WS is a bare login shell.

---

## Spec summary

BL-AG-01 spawns an AI agent binary (Claude Code, Codex, …) in a PTY on the Dev Server, reached over a WebSocket connection the Dev Server itself opened to Orca Server. Orca must: resolve `AgentConfig` (binary/args/trustPresetEnvVars), build env via `ProfileResolver` (3-layer) and `AIProviderResolver` (provider + `apiKeyEnvVar`), call `agent.spawn` over the existing WS connection with an HMAC-signed, 30s-TTL `RpcExecutionContext`, then persist `orca_sessions{sessionId, worktreeId, devServerId, startedAt}` and watch for OSC "idle" to confirm startup (30s timeout → kill + error).

## What backend-go has

- **WS relay topology is real** (BR-AG-18/19/20): `infra-fleet-service` treats the Dev Server as a JSON-RPC-over-WebSocket client that dials in (`backend-go/services/infra-fleet-service/internal/adapter/agentwsserver/server.go`), and Orca Server issues calls back over that same session (`backend-go/services/infra-fleet-service/internal/adapter/devserveragent/client.go`, `getOrCreateSession`).
- **A PTY-spawn primitive exists, but it is a plain shell, not an agent launcher**: `SpawnTerminalSession` (`backend-go/services/infra-fleet-service/internal/usecase/spawn_terminal_session.go:37-94`) calls `DevServerAgentClient.SpawnPty`, which sends the JSON-RPC method `pty.create` with params `{cwd, shellOverride?, cols?, rows?}` (`backend-go/services/infra-fleet-service/internal/adapter/devserveragent/methods.go:23-70`). There is **no `agentBinary`, `args`, `env`, or `userId` param** — `usecase.SpawnPtyInput` has no `Env` field at all (methods.go:27-28 doc comment: "env is not threaded through SpawnPtyInput — no caller populates it yet").
- **Reachable from the frontend today only as a generic terminal**: `wscompat`'s `terminal.create` channel (`backend-go/services/api-gateway/internal/adapter/wscompat/channels_terminal.go:188-282`) calls this same `SpawnTerminalSession` RPC — it is the shell-terminal feature, not an "AI agent" feature.
- **`AIProviderResolver.resolve()` with the priority cascade is real** (used elsewhere, not wired to spawn): `ai-provider-service`'s `ResolveProvider` usecase (`backend-go/services/ai-provider-service/internal/usecase/resolve_provider.go:44-89`) correctly implements user → project → server scope cascade, exposed over gRPC (`backend-go/services/ai-provider-service/internal/adapter/grpc/server.go:71-79`) and REST (`backend-go/services/api-gateway/internal/adapter/httpgateway/ai_provider_routes.go:29,66-90`, `GET /v1/aiprovider/resolve`). It returns only metadata (id, provider_type, credential_ref) — by design it never returns a key, and nothing calls it from a PTY-spawn path.

## What's missing

- No `AgentConfig` concept (binary/args/`sessionResumeFlag`/`trustPresetEnvVars`) anywhere in backend-go — grep for `trustPresetEnvVars`/`startupCommand`/`AgentBinary` across `backend-go/` (excluding tests) returns nothing.
- No `ProfileResolver.resolve(userId)` env-building step feeding a spawn call (tenant-service's `ProfileResolver`/`profile_resolution.go` exists for settings resolution, not for building spawn-time env vars).
- No wiring of `AIProviderResolver.resolve()` output (provider + `apiKeyEnvVar`) into any PTY spawn — `SpawnPtyInput` has no env field to carry it (see above).
- No AI credential file read (`~/.orca/ai-providers/<accountId>.enc`) at spawn time — no such path/reference exists in backend-go.
- No trust-preset application or conflict warning (BR-AG-03, [A4]).
- No `agent.spawn` JSON-RPC method — the only spawn primitive on the wire is `pty.create` (generic shell), confirmed by grepping `devserveragent/methods.go` for all `sess.call(...)` sites: `pty.create`, `pty.write`, `pty.resize`, `pty.destroy`, `pty.sendSignal`, `pty.listProcesses` only.
- No `RpcExecutionContext` HMAC-SHA256/30s-TTL envelope (BR-AG-21) — grep for `hmac`/`HMAC` under `backend-go/services/infra-fleet-service` returns zero hits.
- No `orca_sessions` table or equivalent persistence of `{sessionId, worktreeId, devServerId, startedAt}` — grep across all `backend-go/**/*.sql` migrations for `orca_session`/`agent_session`/`pty_session` returns nothing.
- No OSC-sequence parsing to detect agent "idle" status, and no 30s startup-timeout-then-kill logic (BR-AG-04, [A3]).
- No "binary not found" (`-32001`) error mapping or install-guide link ([A1]).

## See also

- specs/backend-go/bugs/missing-v1/BUG-029-terminal-channels-not-implemented.md — stale for current HEAD (terminal.* channels are now wired, see `channels.go:124`), but documents the same underlying generic-PTY-only transport this bug relies on.
- specs/backend-go/bugs/missing-v1/BUG-005-aiprovider-channels-not-implemented.md — related aiProvider.* WS-channel wiring gaps.

## References

- `docs/logic/agent-orchestration/BL-AG-01-khoi-dong-agent.md`
- `backend-go/services/infra-fleet-service/internal/usecase/spawn_terminal_session.go:37-94`
- `backend-go/services/infra-fleet-service/internal/adapter/devserveragent/methods.go:23-70,183-211`
- `backend-go/services/api-gateway/internal/adapter/wscompat/channels_terminal.go:188-282`
- `backend-go/services/ai-provider-service/internal/usecase/resolve_provider.go:44-89`
- `backend-go/services/ai-provider-service/internal/adapter/grpc/server.go:71-79`
- `backend-go/services/api-gateway/internal/adapter/httpgateway/ai_provider_routes.go:29,66-90`
- `backend-go/proto/orca/infrafleet/v1/infrafleet.proto:48,297-306` — `SpawnTerminalSession` RPC (no agent-binary concept)
