/**
 * agent-turn-completion.ts — FE-CV-TASK-061-01
 *
 * Pure selector for "agent finished a turn" events, derived only from agent-status
 * (live + retained). Id `${paneKey}:${doneAt}` is shared with the turn recorder.
 */

import type { AgentStatusEntry } from '../../../../../shared/agent-status-types'
import type { RetainedAgentEntry } from '@/store/slices/agent-status'

export type AgentTurnCompletion = {
  worktreeId: string
  paneKey: string
  agentType: string
  doneAt: number
  interrupted: boolean
  source: 'live' | 'retained'
  id: string
}

export type AgentTurnCompletionInputs = {
  agentStatusByPaneKey: Record<string, AgentStatusEntry>
  retainedAgentsByPaneKey: Record<string, RetainedAgentEntry>
  tabsByWorktree: Record<string, { id: string }[]>
}

function toCompletion(
  worktreeId: string,
  paneKey: string,
  entry: AgentStatusEntry,
  source: 'live' | 'retained'
): AgentTurnCompletion {
  const doneAt = entry.stateStartedAt
  return {
    worktreeId,
    paneKey,
    agentType: entry.agentType ?? 'unknown',
    doneAt,
    interrupted: entry.interrupted === true,
    source,
    id: `${paneKey}:${doneAt}`
  }
}

/** Newest first; live wins over retained for the same id. Subagents have no pane entry, so never appear. */
export function selectAgentTurnCompletions(
  state: AgentTurnCompletionInputs,
  worktreeId: string
): AgentTurnCompletion[] {
  const byId = new Map<string, AgentTurnCompletion>()
  const tabIds = new Set((state.tabsByWorktree?.[worktreeId] ?? []).map((tab) => tab.id))

  for (const [paneKey, entry] of Object.entries(state.agentStatusByPaneKey ?? {})) {
    if (!entry || entry.state !== 'done' || typeof entry.stateStartedAt !== 'number') {
      continue
    }
    const tabId = entry.tabId ?? paneKey.split(':')[0]
    const belongs = entry.worktreeId ? entry.worktreeId === worktreeId : tabIds.has(tabId)
    if (belongs) {
      const completion = toCompletion(worktreeId, paneKey, entry, 'live')
      byId.set(completion.id, completion)
    }
  }

  for (const [paneKey, retained] of Object.entries(state.retainedAgentsByPaneKey ?? {})) {
    const entry = retained?.entry
    if (
      !entry ||
      retained.worktreeId !== worktreeId ||
      entry.state !== 'done' ||
      typeof entry.stateStartedAt !== 'number'
    ) {
      continue
    }
    const completion = toCompletion(worktreeId, paneKey, entry, 'retained')
    if (!byId.has(completion.id)) {
      byId.set(completion.id, completion)
    }
  }

  return [...byId.values()].sort((a, b) => b.doneAt - a.doneAt)
}

export function selectLatestCompletion(
  completions: readonly AgentTurnCompletion[]
): AgentTurnCompletion | null {
  return completions[0] ?? null
}

export function hasUnreviewedCompletion(
  completions: readonly AgentTurnCompletion[],
  reviewedTurnId: string | null | undefined
): boolean {
  const latest = selectLatestCompletion(completions)
  return latest !== null && latest.id !== reviewedTurnId
}
