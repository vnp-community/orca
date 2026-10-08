/**
 * review-turn-marker-builder.ts — FE-CV-TASK-060-07
 *
 * Builds the lightweight marker stored when an agent finishes a turn. It deliberately has no
 * prompt field: markers are for UI comparison; provenance (agent_turns) is a separate store.
 *
 * @module components/review-map/turns/review-turn-marker-builder
 */

import type { ReviewTurnMarker } from '../../../../../shared/code-intel-types'
import type { AgentStatusEntry } from '../../../../../shared/agent-status-types'
import { fileIdentities } from './turn-file-identity'
import type { TurnFileEntry } from './turn-file-identity'

export const TURN_MARKER_MAX_SYMBOL_KEYS = 1500

export function reviewTurnId(paneKey: string, doneAt: number): string {
  return `${paneKey}:${doneAt}`
}

export function buildReviewTurnMarker(input: {
  worktreeId: string
  paneKey: string
  entry: Pick<AgentStatusEntry, 'agentType' | 'stateStartedAt' | 'stateHistory' | 'interrupted'>
  headOid: string | null
  baseOid: string | null
  mergeBase: string | null
  statusEntries: readonly TurnFileEntry[]
  /** null when the change overlay is not loaded for this worktree. */
  symbolKeys: readonly string[] | null
}): ReviewTurnMarker {
  const lastWorking = (input.entry.stateHistory ?? []).findLast((h) => h.state === 'working')
  const { files } = fileIdentities(input.statusEntries, {
    headOid: input.headOid,
    mergeBase: input.mergeBase
  })
  return {
    turnId: reviewTurnId(input.paneKey, input.entry.stateStartedAt),
    worktreeId: input.worktreeId,
    paneKey: input.paneKey,
    agentType: input.entry.agentType ?? null,
    startedAt: lastWorking?.startedAt ?? null,
    endedAt: input.entry.stateStartedAt,
    ...(input.entry.interrupted ? { interrupted: true } : {}),
    baseOid: input.mergeBase ?? input.baseOid,
    headOid: input.headOid,
    files,
    ...(input.symbolKeys ? { symbolKeys: input.symbolKeys.slice(0, TURN_MARKER_MAX_SYMBOL_KEYS) } : {}),
    overlayAvailable: input.symbolKeys !== null
  }
}
