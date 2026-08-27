# TASK-AG-03-07: [CROSS-REPO FOLLOW-UP] Thread `ptyId` into `AgentHookRelayEnvelope`

**From Solution:** SOL-AG-03
**Priority:** P2 — hardening; TASK-AG-03-05's worktree-correlation fallback covers the common case today
**Service:** `agent/` (cross-repo, outside `backend-go`)
**File:** `agent/src/relay/agent-hook-server.ts`
**Depends on:** none (independent of the backend-go tasks; backend-go's `AgentHookEvent`/`routeAgentHookNotification` already tolerate an eventually-added `ptyId` field being ignored today)
**Status:** `[ ]` TODO — cross-repo, optional hardening; not required to close BUG-AG-03

---

## Context

`AgentHookRelayEnvelope` identifies the originating pane by `paneKey`/`tabId`/`worktreeId` — a Desktop-UI addressing scheme — and carries no `ptyId` or Orca session id. `infra-fleet-service`'s `AgentSession` is keyed by `pty_id`/`id`, not `paneKey`, so there is no exact join key between an `agent.hook` notification and the `AgentSession` row it belongs to. TASK-AG-03-05 implements a best-effort `(tenant_id, worktree_id)` → most-recent-active-session fallback that is correct in the common case (BR-AG-01 guarantees at most one non-terminal session per worktree+user) but wrong in a narrow race — a hook event arriving just after its session transitions out of the active set. This task closes that race exactly, once picked up.

## Changes to make

In `agent/src/relay/agent-spawner.ts`, thread the spawning `ptyId` (already
known at spawn time, unique per session) into whatever context the hook
script's launching pane carries, so `agent-hook-server.ts`'s `forwardEvent`
can populate a new `ptyId` field on `AgentHookRelayEnvelope` alongside the
existing `paneKey`/`tabId`/`worktreeId` fields.

Once this field exists on the wire, update `TASK-AG-03-03`'s
`agentHookNotificationParams` (backend-go, `session.go`) to also decode
`ptyId`, and update `TASK-AG-03-05`'s `RecordAgentHookProviderSession.Handle`
to prefer an exact `AgentSessionRepository.Get`-by-`ptyId`-lookup (add a
`GetByPtyID` method to `AgentSessionRepository` if one doesn't already
exist) over the `MostRecentActiveForWorktree` fallback, falling back to the
worktree-correlation path only when `ptyId` is empty (older agent builds
mid-rollout).

## Verify

Cross-repo integration test (once this lands): spawn → CLI-side hook fires
with a `session_id` → `agent.hook` arrives with `ptyId` populated →
`AgentSession.ResumeProviderSessionID` is set via the exact `ptyId` lookup,
not the worktree fallback → resume succeeds with the exact id even when a
second agent session for the same worktree+user has since started (the
race the fallback cannot handle).

```bash
# agent/ repo test, once the field is threaded through:
cd agent
pnpm test src/relay/agent-hook-server.test.ts
```
