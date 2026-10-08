/**
 * agent-turn-completion-detector.ts — FE-CV-TASK-089-04
 *
 * Pure diff of two `agentStatusByPaneKey` snapshots: which panes just finished a
 * turn. Stands in for the SOL-061 `AgentTurnCompletion` feed, which does not exist yet.
 *
 * @module components/review-map/turns/agent-turn-completion-detector
 */

import type { AgentStatusEntry } from '../../../../../shared/agent-status-types'

export type AgentTurnCompletion = {
  paneKey: string
  entry: AgentStatusEntry
}

export function detectAgentTurnCompletions(
  prev: Record<string, AgentStatusEntry>,
  next: Record<string, AgentStatusEntry>
): AgentTurnCompletion[] {
  const completions: AgentTurnCompletion[] = []
  for (const [paneKey, entry] of Object.entries(next)) {
    if (entry.state !== 'done' || !entry.worktreeId) {
      continue
    }
    const before = prev[paneKey]
    // Why: a `done` entry whose stateStartedAt is unchanged is the same turn (tool/prompt pings
    // refresh updatedAt only); a first-seen `done` is a restore, not a fresh completion.
    if (before && (before.state !== 'done' || before.stateStartedAt !== entry.stateStartedAt)) {
      completions.push({ paneKey, entry })
    }
  }
  return completions
}
