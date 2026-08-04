// Why: startAgent()/resumeAgent() (AgentPanel.tsx) open a
// ui:agentOrch.spawn|resume span, but the real "running" outcome only arrives
// later via the agentOrchestration:statusChanged push event (Electron IPC),
// not the response of the start/resume call itself. This registry lets the
// event hook (TASK-FE-002.3) attach step()/ok()/fail() to the right still-open
// span per worktree instead of the span being orphaned at call-site scope.
import type { TraceSpan } from '../../../shared/trace'

const openSpansByWorktreeId = new Map<string, TraceSpan>()

export function registerOpenAgentOrchSpan(worktreeId: string, span: TraceSpan): void {
  openSpansByWorktreeId.set(worktreeId, span)
}

/** Get and remove the open span for this worktree, if any — use when closing the span. */
export function takeOpenAgentOrchSpan(worktreeId: string): TraceSpan | undefined {
  const span = openSpansByWorktreeId.get(worktreeId)
  openSpansByWorktreeId.delete(worktreeId)
  return span
}

/** Peek the open span without removing it — use for a mid-flight step() while status is still 'starting'. */
export function peekOpenAgentOrchSpan(worktreeId: string): TraceSpan | undefined {
  return openSpansByWorktreeId.get(worktreeId)
}
