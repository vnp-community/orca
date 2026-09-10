# BACKLOG-022: Automation/VM-lifecycle "agent finished" trigger — needs a product decision, not more code

**Origin:** `specs/frontend/crs/v4/automation/tasks/FE-TASK-AUTO-007-agent-session-signal-investigation.md` (FE-AUTO-SOL-005, CR-AUTO-005)
**Priority:** Medium — no current feature is broken; this blocks a *future* automation/ephemeral-VM trigger type from being built correctly
**Blocked on:** **A product decision** between two mitigation designs (see below) — this is explicitly an investigation-only task; the AI executing it was instructed not to write code
**Owner:** whoever owns CR-AUTO-005 and `docs/crs/v3/ephemeral-vm/`'s CR-EVM-010 (same underlying question, shouldn't be investigated twice)

---

## What this is

Automation and ephemeral-VM auto-suspend both want a trigger for "the agent
just finished its task." The obvious candidate — `AgentDetector`'s
working→idle transition (`desktop/src/main/stats/agent-detector.ts`) — was
investigated (2026-09-09) instead of wired up directly, per this task's
explicit "investigation only, do not write code" scope.

## What the investigation found

`AgentDetector`'s signal is **not reliable enough to use directly**:

- It's a per-PTY, OSC-terminal-title heuristic (`detectAgentStatusFromTitle`)
  — one PTY can span multiple agent sessions, and `onAgentStop` fires both
  when the agent is genuinely done **and** when it's mid-task waiting for a
  user reply ("Yes/No?"). Both look identical through this signal — a
  concrete, confirmed false-positive.
- Today's only consumer is `StatsCollector` (usage/billing stats). There's
  also no `onAgentStopped(listener)` hook to subscribe to even if the signal
  were trustworthy — only `onAgentStarted` exists.

## The decision needed

Pick one before any automation/VM-lifecycle trigger code gets built on this:

1. **Extend `AgentDetector`** with a false-positive filter (e.g., only treat
   idle as "done" after N seconds, or key off PTY exit instead of a
   mid-session idle transition) — plus add the missing
   `onAgentStopped(listener)` hook.
2. **Skip `AgentDetector` entirely** and trigger off something already
   precise: the response of the `agent.exec`/PTY-exit event where the agent
   RPC call was made. Cheaper and more accurate, but can't detect "agent
   finished one prompt but is still sitting in the session" (a tradeoff,
   not a bug).

## What unblocks this

A product/architecture decision on which of the two designs above to use —
shared with `docs/crs/v3/ephemeral-vm/`'s CR-EVM-010, which asks the same
question for auto-suspend. Once decided, FE-TASK-AUTO-006's "external
trigger" extension and `FE-TASK-EVM-008` (auto-suspend on task completion)
can both proceed against a concrete design.
