# SOL-FE-PW-004: Wire `AgentPanel`'s agent orchestration to backend-go's real `agent.*` RPCs on web

**Resolves:** [BUG-FE-PW-005](../BUG-FE-PW-005-agent-panel-window-api-undefined-on-web.md)
**Status:** ✅ Implemented + unit-tested (`AgentPanel.test.tsx`, 7 passing tests) and deployed. Written per user request ("làm spec chi tiết đi") before any code change; implemented per "spec ok chuaw, fix di".

## Post-investigation update — bigger than originally framed

Continued investigation (before implementation) found the Electron-era reference this bug's own framing assumed existed **never did**: `runtime.startAgent`/`OrcaRuntimeService.startAgent` (which `desktop/src/main/ipc/agent-orchestration.ts`'s `agentOrchestration:start` IPC handler calls) has no implementation anywhere in `desktop/`, and `registerAgentOrchestrationHandlers` itself is **never called** from anywhere — the whole IPC module is dead, unwired code. So this was never "port a working Electron feature to web" — it was "implement a feature that has only ever existed as an unwired stub, on both platforms."

The correct design was already documented, just not where `AgentPanel.tsx` looked: [TASK-AG-01-07](../../../../backend-go/bugs/logic-v1/tasks/TASK-AG-01-07-usecase-start-agent-session-wiring.md) states explicitly that `StartAgentSession` never resolves its own account — **the caller must call `aiProvider.resolve` first** (`channels_ai_provider.go`), which cascades user→project→server scope, optionally filtered by a `ModelHint`-derived `ProviderType` (`ai-provider-service/internal/usecase/model_provider_map.go`'s `detectProviderFromModel`, a simple prefix match: `"claude-"`→Anthropic, `"gpt-"/"o1-"/"o3-"`→OpenAI, `"gemini-"`→Google).

While wiring this, found and fixed **[BUG-022](../../../../backend-go/bugs/missing-v2/BUG-022-aiprovider-resolve-returns-raw-snake-case-proto.md)**: `aiProvider.resolve` (having zero prior callers) had never been exercised, and returned the raw proto (snake_case keys, numeric `type` enum) — the same "raw proto over wscompat" bug class as BUG-020, just never caught. Fixed for `resolve` specifically; sibling `aiProvider.*` handlers used by the already-shipped Settings > AI Providers page have the same bug but a materially different, pre-existing, out-of-scope shape mismatch — left alone, documented in BUG-022.

Resolved every item from "Not done in this spec" below except the pty-output-stream product decision (still deferred — output is opened and immediately discarded, no UI surface added, matching the "smallest correct fix" framing).

---

## Correction to BUG-FE-PW-005's own framing

BUG-FE-PW-005 (and this session's earlier guidance) assumed the fix would be "implement `agentOrchestration` in the web preload, backed by a wscompat channel" — implying the backend RPC might not exist yet, similar to CR-TSRC-001's capability gap. **That assumption was wrong.** `backend-go/services/api-gateway/internal/adapter/wscompat/channels_agent.go` already implements a complete, real `agent.start`/`agent.stop`/`agent.kill`/`agent.resume`/`agent.switchAccount`/`agent.subscribeStatus` surface, backed by infra-fleet-service's `StartAgentSession`/`StopAgentSession`/`ResumeAgentSession`/`SwitchAgentAccount` gRPC (TASK-AG-01..05). This is **not a missing-backend-feature gap** — it's a frontend-only wiring gap, but a **non-trivial one**: the real RPC contracts differ substantially from what `AgentPanel.tsx` (built years earlier against the Electron `agentOrchestration` IPC bridge) currently sends/expects. A thin 1:1 preload shim is not possible; either the shim or `AgentPanel.tsx` itself must bridge the contract gap.

## Real backend-go contract (verified from `channels_agent.go`, source read, not guessed)

| RPC | Request | Response | Notes |
|---|---|---|---|
| `agent.start` (`RegisterStreamChannel`) | `{connectionId, worktreeId, userId, cwd, modelId, accountId, trustPreset, cols, rows}` | ack: `agentSessionView` = `{id, ptyId, worktreeId, devServerId, userId, modelId, accountId, status, startedAtUnixMs, lastActiveAtUnixMs}`; **plus** a push-event stream (pty output/exit, via the same `AttachPty` mechanism `terminal.create` uses) | Needs a real `modelId`+`accountId` pair, not a loose `agentType` string |
| `agent.stop` | `{sessionId}` | `{ok: true}` | |
| `agent.kill` | `{sessionId, signal}` | `{ok: true}` | `signal: ""` defaults server-side to SIGKILL |
| `agent.resume` | `{connectionId, worktreeId, userId, cwd, cols, rows}` | same `agentSessionView` + pty stream | **Keyed by `worktreeId`, not `sessionId`** |
| `agent.switchAccount` | `{connectionId, worktreeId, userId, projectId, cwd}` | same `agentSessionView` + pty stream | |
| `agent.subscribeStatus` | none (pure push, `RegisterStream`) | pushes `{Channel: 'agent.statusChanged', Args: [payload]}` and `{Channel: 'agent:rateLimited', Args: [payload]}` from NATS `orca.infra.agent.statusChanged`/`orca.infra.agent.rateLimited` (tenant-filtered) | `payload`'s exact shape not yet traced in this pass — see "Not done" below |

## What `AgentPanel.tsx` currently sends/expects (the OLD Electron `agentOrchestration` IPC contract)

| Call | Sends | Expects back |
|---|---|---|
| `start` | `{worktreeId, agentType: 'claude'\|'codex'\|'custom', trustPreset, traceId}` | `{sessionId, status: 'started'\|'already-running'}` |
| `stop` | `{sessionId, traceId}` | void |
| `resume` | `{sessionId, traceId}` | `{sessionId, status}` (same shape as start, per the code's `result.sessionId`/`result.status` reads) |
| `onStatusChanged` | (subscribe, returns unsubscribe fn) | `{worktreeId, sessionId?, status: 'starting'\|'running'\|'stopped'\|'error', errorMessage?}` |

## Contract mismatches — the real scope of this fix (not a thin shim)

1. **`start` needs `connectionId`/`userId`/`modelId`/`accountId`/`cwd`/`cols`/`rows`, none of which `AgentPanel.tsx` currently has or computes.** `agentType` (a loose 3-value enum: claude/codex/custom) has no direct mapping to a real `modelId`+`accountId` pair (an AI-provider-account concept, matching e.g. `DiscoverCommitMessageModels`'s account-resolution pattern elsewhere in this codebase — not traced in this pass, needs its own lookup). **`connectionId`** has an existing, ready-to-use resolver: `getConnectionId(worktreeId)` (`frontend/src/renderer/src/lib/connection-context.ts`), already used by `Terminal.tsx` for the exact same purpose.
2. **`resume` is keyed by `worktreeId` server-side, but `AgentPanel.tsx` calls it keyed by `sessionId`.** There is no `ResumeAgentSession`-by-`sessionId` RPC at all today. Either `AgentPanel.tsx`'s resume button needs to resolve/pass `worktreeId` instead (likely already has it — `resumeAgent`'s closure captures `worktreeId` via the component's own prop, so this is a small, mechanical change, not a new gap), or a matching backend RPC would need to be added (not recommended — the existing shape is more correct: resuming is inherently "restart this worktree's agent," not "restart this specific old session id").
3. **Response field names differ**: real backend returns `id` (not `sessionId`), and `status` is presumably a richer string vocabulary from `infrafleetv1.AgentSession.Status` (not traced in this pass — needs checking against the proto enum) rather than the old `'started'|'already-running'` two-value contract `AgentPanel.tsx`'s `startAgent` branches on (`result.status === 'already-running'`).
4. **The pty-output stream is a NEW concept for `AgentPanel.tsx`.** The old Electron IPC never delivered raw pty bytes to this panel (it only exposed a coarse status enum) — the real `agent.start`/`resume`/`switchAccount` RPCs open an attached pty stream that must be drained (reusing `subscribeRuntimeStreamChannel<TAck, TEvent>`, `runtime-rpc-client.ts:105` — the SAME generic helper `runtime-ephemeral-vm-client.ts`/`runtime-client-events.ts` already use for other `RegisterStreamChannel`-backed RPCs). `AgentPanel.tsx` doesn't currently render any terminal/output surface at all — it's only a status badge + buttons. Whether this stream should be silently drained-and-discarded (if the panel never shows output) or whether the panel should grow an embedded output view is a **product decision, not something this spec can settle**.
5. **`onStatusChanged`'s real analog is `agent.subscribeStatus`**, a separate always-on subscription (not returned inline from `start`/`resume`) — the web adapter needs to open this subscription once (likely at `AgentPanel` mount, matching the existing `useEffect` shape) and filter/dispatch by `worktreeId`, mirroring what the old IPC's `onStatusChanged` did implicitly via Electron's broadcast-to-all-windows `broadcastAgentStatusEvent`.

## Recommended approach

Given the mismatches are substantial (not just missing keys, but a different resume semantic and a genuinely new streaming concept), **the fix belongs partly in `AgentPanel.tsx` itself**, not purely in a web-preload shim:

1. Add a real `modelId`/`accountId` resolver to replace the loose `agentType` selector — likely reusing whatever this codebase's existing "AI provider account" resolution already does elsewhere (not traced here; needs its own short investigation before implementation, e.g. check `DiscoverCommitMessageModels`'s usecase in git-gateway-service and its frontend consumer for the established pattern).
2. Change `resumeAgent` to send `worktreeId` (already in scope via the component prop) instead of `sessionId`.
3. Implement a **new** `frontend/src/renderer/src/runtime/agent-orchestration-client.ts` (naming placeholder) exposing `startAgentSession`/`stopAgentSession`/`resumeAgentSession`/`subscribeAgentStatus`, built on `callRuntimeRpc`/`subscribeRuntimeStreamChannel` — mirroring `runtime-ephemeral-vm-client.ts`'s own shape — rather than trying to preserve `window.api.agentOrchestration.*`'s exact old method names. `AgentPanel.tsx` would then call this new client directly instead of `window.api.agentOrchestration`, matching how other Project Workspace panels already call runtime clients directly rather than through a preload-bridge fiction (`ProjectSettings.tsx`, `WorkspaceTerminalPanel.tsx` per BUG-FE-PW-005's own prior investigation).
4. Update `RemoteAgentSession`'s store shape if needed to carry whatever additional fields the real `agentSessionView` provides (`ptyId`, `devServerId`) that the old IPC shape never had.

**This reframes the fix from "small preload shim" to "small-to-medium feature-completion task"** — a few files, no backend-go changes needed (the RPCs already exist), but real design work (modelId/accountId resolution, pty-stream handling decision) before implementation, not a pure mechanical wire-up.

## What was implemented

- **`frontend/src/renderer/src/runtime/runtime-agent-orchestration-client.ts`** (new): `resolveRuntimeAgentProvider` (wraps `aiProvider.resolve`, mapping `agentType`→`modelHint` via a prefix probe — `claude`→`"claude-"`, `codex`→`"gpt-"`, `custom`→ no hint, unfiltered cascade), `startRuntimeAgentSession`/`resumeRuntimeAgentSession` (both `subscribeRuntimeStreamChannel`-based, matching `agent.start`/`agent.resume`'s `RegisterStreamChannel` ack+push shape), `stopRuntimeAgentSession`, `killRuntimeAgentSession`, `subscribeRuntimeAgentStatus` (wraps the pure-push `agent.subscribeStatus`).
- **`AgentPanel.tsx`**: replaced every `window.api.agentOrchestration.*` call with the new client. `startAgent` now resolves `accountId`/`modelId` via `resolveRuntimeAgentProvider` before calling `agent.start`, using `getConnectionId(worktreeId)` for `connectionId`, `findWorktreeById(...).path` for `cwd`, and `useAuthUser()`/`useWorkspace()` for `userId`/`projectId`/`devServerId`. `resumeAgent` now resumes by `worktreeId` (per contract mismatch #2), no longer by the old `sessionId`-keyed call, and no longer branches on a `resumed: boolean` flag (the real RPC has none — success is the ack itself). `toRemoteAgentStatus` collapses the real `AgentSession.status` vocabulary (`spawning|idle|running|waiting|completed|error|stopped`, confirmed from `infrafleet.proto`) down to the panel's existing 4-value badge (`starting|running|stopped|error`) — `idle`/`waiting` both show as "Running" since the UI doesn't act on the distinction. The mount-time status subscription now opens `agent.subscribeStatus` once (tenant-wide push, no server-side worktree filter) and matches incoming `{session_id, status}` frames against the currently-tracked session id read fresh from the store on each event, since the old per-worktree IPC broadcast semantics don't apply to this NATS-backed push.
- **`backend-go/services/api-gateway/internal/adapter/wscompat/channels_ai_provider.go`**: [BUG-022](../../../../backend-go/bugs/missing-v2/BUG-022-aiprovider-resolve-returns-raw-snake-case-proto.md)'s fix — `aiProvider.resolve` now returns a camelCase `providerAccountView` instead of the raw proto.

## Resolved from "Not done" (original list)

- `AgentSession.status` vocabulary: confirmed (`infrafleetv1.AgentSession`, `spawning|idle|running|waiting|completed|error|stopped`) — mapped via `toRemoteAgentStatus`.
- `agent.subscribeStatus`'s push payload: confirmed `{session_id, status}` (snake_case, no `worktreeId`) — handled via store-based session-id matching.
- `modelId`/`accountId` resolution: `aiProvider.resolve`, per TASK-AG-01-07 — implemented.
- `agent.kill`: left unexposed in the UI (only `startRuntimeAgentSession`/`stopRuntimeAgentSession`/`resumeRuntimeAgentSession` are wired to buttons), `killRuntimeAgentSession` exists in the client for a future "force kill" affordance if ever needed.

## Still open (deliberately out of scope)

- **Pty-output stream**: `agent.start`/`agent.resume`'s attached pty stream is opened (required — `agent.start`/`agent.resume` are `RegisterStreamChannel` methods, the ack cannot be read without subscribing) but its events are discarded; no terminal-output UI was added to `AgentPanel`. Revisit if/when there's a product ask for visible agent output in this panel.
- **`files.*` wscompat handlers' worktreeId/relativePath mismatch** (BUG-021's "related, not fixed" note) — unrelated to this fix, still open.
- **The other 4 `aiProvider.*` handlers' raw-proto bug** (BUG-022's "related, not fixed" note) — the Settings > AI Providers page's own, separate, deeper contract mismatch — still open.

## Testing (done)

- `AgentPanel.test.tsx`: 7 tests covering start (provider-resolve-then-start, span stays open on ack), start failure at both the resolve and the start step, stop success/failure, resume success (asserting `worktreeId`-keyed args, not `sessionId`) and resume failure. All passing.
- Backend: `TestAiProviderResolveChannel_Success` (BUG-022) updated to assert the camelCase view and absence of snake_case keys on the wire.
- `go build`/`go vet`/`go test ./services/api-gateway/...` clean; `npx tsc --noEmit` clean for all touched files; `gitnexus impact` on `handleAiProviderResolve` and `AgentPanel` both LOW risk.
- Live verification on `b15.openledger.vn` pending user retry from Project Workspace (Beta)'s Agent tab.

## Related

- [BUG-FE-PW-005](../BUG-FE-PW-005-agent-panel-window-api-undefined-on-web.md) — the bug this solves
- [CR-PW-009](../../../../../docs/crs/v3/project-workspace/CR-PW-009-close-backend-only-ui-gaps-in-project-workspace.md) — `agent.switchAccount` UI gap, same file/area, deferred pending this fix per that CR's own note
- [CR-PW-012](../../../../../docs/crs/v3/project-workspace/CR-PW-012-worktree-click-forces-terminal-view-out-of-beta-workspace.md) — the sibling Project Workspace (Beta) bug fixed in the same investigation pass that led here
