/**
 * review-turn-marker-row.ts — FE-CV-TASK-060-05
 *
 * Turn markers live only on the worktree-level ReviewState row (baseCommit = headCommit = '').
 * That row has no other writer in the UI, but several components may record at once, so saves
 * are serialized per worktree and a version conflict is merged once and retried.
 *
 * @module components/review-map/turns/review-turn-marker-row
 */

import type { ReviewTurnMarker } from '../../../../../shared/code-intel-types'
import type { ReviewDataApi } from '../review-shell-data'
import { mergeTurnMarkers, readTurnMarkers } from '../notes/review-state-merge'

const WORKTREE_ROW = { baseCommit: '', headCommit: '' } as const

export type TurnMarkerSaveResult =
  | { ok: true; markers: ReviewTurnMarker[] }
  | { ok: false; errorKind: string }

export async function loadTurnMarkers(
  api: ReviewDataApi,
  worktreeId: string
): Promise<{ ok: true; markers: ReviewTurnMarker[] } | { ok: false; errorKind: string }> {
  const res = await api.getReviewState(worktreeId, WORKTREE_ROW)
  return res.ok
    ? { ok: true, markers: readTurnMarkers(res.value.turnMarkers) }
    : { ok: false, errorKind: res.error.kind }
}

async function saveOnce(
  api: ReviewDataApi,
  worktreeId: string,
  incoming: readonly ReviewTurnMarker[]
): Promise<TurnMarkerSaveResult> {
  for (let attempt = 0; attempt < 2; attempt += 1) {
    const current = await api.getReviewState(worktreeId, WORKTREE_ROW)
    if (!current.ok) {
      return { ok: false, errorKind: current.error.kind }
    }
    const markers = mergeTurnMarkers(incoming, readTurnMarkers(current.value.turnMarkers))
    const res = await api.saveReviewState(
      worktreeId,
      { ...current.value, ...WORKTREE_ROW, turnMarkers: markers },
      current.value.version
    )
    if (res.ok) {
      return { ok: true, markers: readTurnMarkers(res.value.turnMarkers ?? markers) }
    }
    if (res.error.kind !== 'conflict') {
      return { ok: false, errorKind: res.error.kind }
    }
  }
  return { ok: false, errorKind: 'conflict' }
}

const queues = new Map<string, Promise<unknown>>()

/** Serialized per worktree; the entry is dropped once the tail settles so nothing accumulates. */
export function saveTurnMarkerRow(
  api: ReviewDataApi,
  worktreeId: string,
  incoming: readonly ReviewTurnMarker[]
): Promise<TurnMarkerSaveResult> {
  const tail = (queues.get(worktreeId) ?? Promise.resolve()).then(
    () => saveOnce(api, worktreeId, incoming),
    () => saveOnce(api, worktreeId, incoming)
  )
  queues.set(worktreeId, tail)
  const cleanup = (): void => {
    if (queues.get(worktreeId) === tail) {
      queues.delete(worktreeId)
    }
  }
  tail.then(cleanup, cleanup)
  return tail
}

export function pendingTurnMarkerQueues(): number {
  return queues.size
}
